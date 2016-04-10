// Copyright 2016 VMware, Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/stringid"
	"github.com/vmware/vic/metadata"
	"github.com/vmware/vic/pkg/dio"
)

// pathPrefix is used for testing - it allows for creating and manupulating files outside of
// a full containerVM environment
var pathPrefix string

// the reload channel is used to block reloading of the config
// there will only be something on this channel on two occasions:
// 1. initial start
// 2. post-vmfork
var reload chan bool

// config holds the main configuration for the executor
var config *metadata.ExecutorConfig

// pty holds the map of sessions to PTY multiwriters for now - should be updated when we can
// extend the core config structs
var ptys map[string]*ptySession

// ptySession groups up those elements we need for pty enabled sessions
type ptySession struct {
	pty    *os.File
	writer dio.DynamicMultiWriter
	wait   sync.WaitGroup
}

// Set of child PIDs created by us.
var childPidTable = make(map[int]*metadata.Cmd)

// Exclusive access to childPidTable
var childPidTableMutex = &sync.Mutex{}

// RemoveChildPid is a synchronized accessor for the pid map the deletes the entry and returns the value
func RemoveChildPid(pid int) (*metadata.Cmd, bool) {
	childPidTableMutex.Lock()
	defer childPidTableMutex.Unlock()

	cmd, ok := childPidTable[pid]
	delete(childPidTable, pid)
	return cmd, ok
}

// LenChildPid returns the number of entries
func LenChildPid() int {
	childPidTableMutex.Lock()
	defer childPidTableMutex.Unlock()

	return len(childPidTable)
}

func run(loader metadata.ConfigLoader) error {
	reload = make(chan bool, 1)
	ptys = make(map[string]*ptySession)

	// HACK: workaround file descriptor conflict in pipe2 return from the exec.Command.Start
	// it's not clear whether this is a cross platform issue, or still an issue as of this commit
	// keeping it until there's time to verify and fix properly with a Go PR.
	_, _, _ = os.Pipe()

	// perform basic one off OS specific setup
	err := setup()
	if err != nil {
		detail := fmt.Sprintf("failed during setup: %s", err)
		log.Error(detail)
		return errors.New(detail)
	}

	// initial setup, so seed this
	reload <- true
	for _ = range reload {
		// load the config - this modifies the structure values in place
		config, err := loader.LoadConfig()
		if err != nil {
			detail := fmt.Sprintf("failed to load config: %s", err)
			log.Error(detail)
			// we don't attempt to recover from this - our async backchannel isn't working
			// as expected so just exit
			return errors.New(detail)
		}

		logConfig(config)

		if err := SetHostname(stringid.TruncateID(config.ID)); err != nil {
			detail := fmt.Sprintf("failed to set hostname: %s", err)
			log.Error(detail)
			// we don't attempt to recover from this - it's a fundemental misconfiguration
			// so just exit
			return errors.New(detail)
		}

		// process the sessions and launch if needed
		attach := false
		for id, session := range config.Sessions {
			var proc *os.Process
			if session.Cmd.Cmd != nil {
				proc = session.Cmd.Cmd.Process
			}

			if session.Attach {
				// this will return nil if already running
				err := attachServer.start()
				if err != nil {
					detail := fmt.Sprintf("unable to start attach server: %s", err)
					log.Error(detail)
					continue
				}
			}

			// check if session is alive and well
			if proc != nil && proc.Signal(syscall.Signal(0)) != nil {
				continue
			}

			// check if session has never been started
			if proc == nil {
				err := launch(&session)
				if err != nil {
					detail := fmt.Sprintf("failed to launch %s for %s: %s", session.Cmd.Path, id, err)
					log.Error(detail)

					// TODO: check if failure to launch this is fatal to everything in this containerVM
					return errors.New(detail)
				}

				// TODO: decide how to handle restart - probably needs to glue into the child reaping
			}

			// handle exited session
			// TODO
		}

		if !attach {
			attachServer.stop()
		}
	}

	return nil
}

// handleSessionExit processes the result from the session command, records it in persistent
// maner and determines if the Executor should exit
func handleSessionExit(cmd *metadata.Cmd) error {
	// flush session log output

	// record exit status

	// check for executor behaviour
	if LenChildPid() == 0 {
		// let the main loop exit if there's no more sessions to wait on
		close(reload)
	}

	return nil
}

// launch will launch the command defined in the session.
// This will return an error if the session fails to launch
func launch(session *metadata.SessionConfig) error {
	c := &session.Cmd
	cmd := &exec.Cmd{
		Path: c.Path,
		Args: c.Args,
		Env:  processEnvOS(c.Env),
		Dir:  c.Dir,
	}
	c.Cmd = cmd

	writer, err := sessionLogWriter()
	if err != nil {
		detail := fmt.Sprintf("failed to get log writer for session: %s", err)
		log.Error(detail)
		return errors.New(detail)
	}

	// Use the mutex to make creating a child and adding the child pid into the
	// childPidTable appear atomic to the reaper function. Use a anonymous function
	// so we can defer unlocking locally
	err = func() error {
		childPidTableMutex.Lock()
		defer childPidTableMutex.Unlock()

		log.Infof("Launching command %+q\n", cmd.Args)
		cmd.Stdin = nil
		cmd.Stdout = writer
		cmd.Stderr = writer
		if !session.Tty {
			err = cmd.Start()
		} else {
			var p *ptySession
			p, err = establishPty(cmd)
			ptys[session.ID] = p
		}

		if err != nil {
			detail := fmt.Sprintf("failed to start container process: %s", err)
			log.Error(detail)
			return errors.New(detail)
		}

		// ChildReaper will use this channel to inform us the wait status of the child.
		childPidTable[cmd.Process.Pid] = c

		return nil
	}()

	return err
}

func logConfig(config *metadata.ExecutorConfig) {
	// just pretty print the json for now
	log.Info("Loaded executor config")
	json, err := json.MarshalIndent(config, "", "   ")
	if err != nil {
		log.Debugf("Failed to marshal config into json for logging: %s", err)
		return
	}

	log.Debug(string(json))
}

func forkHandler() {
	log.Println("Registering fork trigger signal handler")
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered in StartConnectionManager", r)
		}
	}()

	incoming := make(chan os.Signal, 1)
	signal.Notify(incoming, syscall.SIGABRT)

	log.Println("SIGABRT handling initialized for fork support")
	for _ = range incoming {
		// validate that this is a fork trigger and not just a random signal from
		// container processes
		log.Println("Received SIGABRT - preparing to transition to fork parent")

		// TODO: record fork trigger in Config and persist

		// TODO: do we need to rebind session executions stdio to /dev/null or to files?
		err := Fork(config)
		if err != nil {
			log.Errorf("vmfork failed:%s\n", err)
			// TODO: how do we handle fork failure behaviour at a container level?
			// Does it differ if triggered manually vs via pointcut conditions in a build file
			continue
		}

		// trigger a reload of the configuration
		reload <- true
	}
}
