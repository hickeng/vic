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
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"testing"

	log "github.com/Sirupsen/logrus"
)

// createFakeDevices creates regular files or pipes in place of the char devices used
// in a full VM
func createFakeDevices() error {
	var err error
	// create control channel
	path := fmt.Sprintf("%s/ttyS0", pathPrefix)
	err = MkNamedPipe(path+"s", os.ModePerm)
	if err != nil {
		detail := fmt.Sprintf("failed to create fifo pipe %ss for com0: %s", path, err)
		return errors.New(detail)
	}
	err = MkNamedPipe(path+"c", os.ModePerm)
	if err != nil {
		detail := fmt.Sprintf("failed to create fifo pipe %sc for com0: %s", path, err)
		return errors.New(detail)
	}
	log.Debugf("created %s/ttyS0{c,s} as raw conn pipes", pathPrefix)

	// others are non-interactive
	for i := 1; i < 3; i++ {
		path = fmt.Sprintf("%s/ttyS%d", pathPrefix, i)
		_, err = os.Create(path)
		if err != nil {
			detail := fmt.Sprintf("failed to create %s for com%d: %s", path, i+1, err)
			return errors.New(detail)
		}
		log.Debugf("created %s as persistent log destinations", path)
	}

	// make an access to urandom
	path = fmt.Sprintf("%s/urandom", pathPrefix)
	err = os.Symlink("/dev/urandom", path)
	if err != nil {
		detail := fmt.Sprintf("failed to create urandom access: %s", err)
		return errors.New(detail)
	}

	return nil
}

// TestMain simply so we have control of debugging level and somewhere to call package wide test setup
func TestMain(m *testing.M) {
	log.SetLevel(log.DebugLevel)

	// save the base os specific structures
	specificOps = ops
	specificUtils = utils

	retCode := m.Run()

	// call with result of m.Run()
	os.Exit(retCode)
}

func testSetup(t *testing.T) {
	var err error

	pc, _, _, _ := runtime.Caller(1)
	name := runtime.FuncForPC(pc).Name()

	log.Infof("Started test setup for %s", name)

	// use the mock ops - fresh one each time as tests might apply different mocked calls
	mocked = mocker{
		ops:     specificOps,
		utils:   specificUtils,
		started: make(chan bool, 0),
		cleaned: make(chan bool, 0),
	}
	ops = &mocked
	utils = &mocked

	pathPrefix, err = ioutil.TempDir("", path.Base(name))
	if err != nil {
		fmt.Println(err)
		t.Error(err)
	}

	err = os.MkdirAll(pathPrefix, 0777)
	if err != nil {
		fmt.Println(err)
		t.Error(err)
	}
	log.Infof("Using %s as test prefix", pathPrefix)

	backchannelMode = os.ModeNamedPipe | os.ModePerm
	err = createFakeDevices()
	if err != nil {
		fmt.Println(err)
		t.Error(err)
	}

	// supply custom attach server so we can inspect its state
	testServer := &testAttachServer{
		updated: make(chan bool, 10),
	}
	server = testServer
}

func testTeardown(t *testing.T) {
	// let the main tether loop exit
	r := reload
	reload = nil
	if r != nil {
		close(r)
	}
	// cleanup
	os.RemoveAll(pathPrefix)
	log.SetOutput(os.Stdout)

	<-mocked.cleaned

	pc, _, _, _ := runtime.Caller(1)
	name := runtime.FuncForPC(pc).Name()

	log.Infof("Finished test teardown for %s", name)
}
