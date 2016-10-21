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

package exec

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/vmware/govmomi/guest"
	"github.com/vmware/govmomi/task"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/extraconfig"
	"github.com/vmware/vic/pkg/vsphere/extraconfig/vmomi"
	"github.com/vmware/vic/pkg/vsphere/tasks"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

// NotYetExistError is returned when a call that requires a VM exist is made
type NotYetExistError struct {
	ID string
}

func (e NotYetExistError) Error() string {
	return fmt.Sprintf("%s is not completely created", e.ID)
}

// containerBase holds fields common between Handle and Container. The fields and
// methods in containerBase should not require locking as they're primary use is:
// a. for read-only reference when used in Container
// b. single use/no-concurrent modification when used in Handle
type containerBase struct {
	ExecConfig *executor.ExecutorConfig

	// original - can be pointers so long as refreshes
	// use different instances of the structures
	Config  *types.VirtualMachineConfigInfo
	Runtime *types.VirtualMachineRuntimeInfo

	// doesn't change so can be copied here
	vm *vm.VirtualMachine
}

func newBase(vm *vm.VirtualMachine, c *types.VirtualMachineConfigInfo, r *types.VirtualMachineRuntimeInfo) *containerBase {
	base := &containerBase{
		ExecConfig: &executor.ExecutorConfig{},
		Config:     c,
		Runtime:    r,
		vm:         vm,
	}

	// construct a working copy of the exec config
	if c != nil && c.ExtraConfig != nil {
		src := vmomi.OptionValueSource(c.ExtraConfig)
		extraconfig.Decode(src, base.ExecConfig)
	}

	return base
}

// unlocked refresh of container state
func (c *containerBase) refresh(op *trace.Operation) error {
	defer trace.End(trace.BeginOp(op, "refresh base state from remote"))

	base, err := c.updates(op)
	if err != nil {
		op.Errorf("Unable to update base state")
		return err
	}

	// copy over the new state
	*c = *base

	op.Debugf("base state now ChangeVersion: %s", base.Config.ChangeVersion)
	return nil
}

// updates acquires updates from the infrastructure without holding a lock
// This does not modify the containerBase that calls it
func (c *containerBase) updates(op *trace.Operation) (*containerBase, error) {
	defer trace.End(trace.BeginOp(op, "return base state from remote"))

	var o mo.VirtualMachine

	// make sure we have vm
	if c.vm == nil {
		return nil, NotYetExistError{c.ExecConfig.ID}
	}

	if err := c.vm.Properties(op, c.vm.Reference(), []string{"config", "runtime"}, &o); err != nil {
		return nil, err
	}

	base := &containerBase{
		vm:         c.vm,
		Config:     o.Config,
		Runtime:    &o.Runtime,
		ExecConfig: &executor.ExecutorConfig{},
	}

	// Get the ExtraConfig
	extraconfig.Decode(vmomi.OptionValueSource(o.Config.ExtraConfig), base.ExecConfig)

	return base, nil
}

func (c *containerBase) startGuestProgram(op *trace.Operation, name string, args string) error {
	defer trace.End(trace.BeginOp(op, "start guest program"))

	// make sure we have vm
	if c.vm == nil {
		return NotYetExistError{c.ExecConfig.ID}
	}

	o := guest.NewOperationsManager(c.vm.Client.Client, c.vm.Reference())
	m, err := o.ProcessManager(op)
	if err != nil {
		op.Errorf("unable to get process manager: %s", err)
		return err
	}

	spec := types.GuestProgramSpec{
		ProgramPath: name,
		Arguments:   args,
	}

	auth := types.NamePasswordAuthentication{
		Username: c.ExecConfig.ID,
	}

	op.Debugf("starting %s %s", name, args)
	_, err = m.StartProgram(op, &auth, &spec)

	return err
}

func (c *containerBase) start(op *trace.Operation) error {
	defer trace.End(trace.BeginOp(op, "start container"))

	id := c.ExecConfig.ID
	op.Infof("container start for %s", id)

	// make sure we have vm
	if c.vm == nil {
		op.Errorf("unable to start container without VM created")
		return NotYetExistError{id}
	}

	// Power on
	_, err := tasks.WaitForResult(op, func(ctx context.Context) (tasks.Task, error) {
		return c.vm.PowerOn(ctx)
	})
	if err != nil {
		return err
	}

	// guestinfo key that we want to wait for
	key := fmt.Sprintf("guestinfo.vice..sessions|%s.started", id)
	var detail string

	// Wait some before giving up...
	newop, cancel := trace.WithTimeout(op, propertyCollectorTimeout, "wait for extraconfig key")
	defer cancel()

	detail, err = c.vm.WaitForKeyInExtraConfig(&newop, key)
	if err != nil {
		return fmt.Errorf("unable to wait for process launch status: %s", err.Error())
	}

	if detail != "true" {
		op.Errorf("container start failed for %s: %s", id, detail)
		return errors.New(detail)
	}

	op.Infof("container started: %s", id)
	return nil
}

func (c *containerBase) stop(op *trace.Operation, waitTime *int32) error {
	defer trace.End(trace.BeginOp(op, "stop container"))

	id := c.ExecConfig.ID
	op.Infof("container stop for %s", id)

	// make sure we have vm
	if c.vm == nil {
		op.Errorf("unable to stop container without VM created")
		return NotYetExistError{id}
	}

	// get existing state and set to stopping
	// if there's a failure we'll revert to existing

	err := c.shutdown(op, waitTime)
	if err == nil {
		return nil
	}

	op.Warnf("stopping %s via hard power off due to: %s", id, err)

	_, err = tasks.WaitForResult(op, func(ctx context.Context) (tasks.Task, error) {
		return c.vm.PowerOff(ctx)
	})

	if err != nil {

		// It is possible the VM has finally shutdown in between, ignore the error in that case
		if terr, ok := err.(task.Error); ok {
			switch terr := terr.Fault().(type) {
			case *types.InvalidPowerState:
				if terr.ExistingState == types.VirtualMachinePowerStatePoweredOff {
					op.Warnf("power off %s task skipped (state was already %s)", id, terr.ExistingState)
					return nil
				}
				op.Warnf("invalid power state during power off: %s", terr.ExistingState)

			case *types.GenericVmConfigFault:

				// Check if the poweroff task was canceled due to a concurrent guest shutdown
				if len(terr.FaultMessage) > 0 && terr.FaultMessage[0].Key == vmNotSuspendedKey {
					op.Infof("power off %s task skipped due to guest shutdown", id)
					return nil
				}
				op.Errorf("generic vm config fault during power off: %#v", terr)

			default:
				op.Errorf("hard power off failed due to: %#v", terr)
			}
		}

		return err
	}

	op.Infof("container stopped: %s", id)
	return nil
}

func (c *containerBase) shutdown(op *trace.Operation, waitTime *int32) error {
	defer trace.End(trace.BeginOp(op, "clean shutdown"))

	id := c.ExecConfig.ID
	op.Infof("container shutdown for %s", id)

	// make sure we have vm
	if c.vm == nil {
		return NotYetExistError{id}
	}

	wait := 10 * time.Second // default
	if waitTime != nil && *waitTime > 0 {
		wait = time.Duration(*waitTime) * time.Second
	}

	cs := c.ExecConfig.Sessions[id]
	stop := []string{cs.StopSignal, string(ssh.SIGKILL)}
	if stop[0] == "" {
		stop[0] = string(ssh.SIGTERM)
	}

	for _, sig := range stop {
		msg := fmt.Sprintf("sending kill -%s %s", sig, id)
		op.Infof(msg)

		err := c.startGuestProgram(op, "kill", sig)
		if err != nil {
			return fmt.Errorf("%s: %s", msg, err)
		}

		op.Debugf("waiting %s for %s to power off", wait, id)
		timeout, err := c.waitForPowerState(op, wait, types.VirtualMachinePowerStatePoweredOff)
		if err == nil {
			op.Infof("container shutdown: %s", id)
			return nil // VM has powered off
		}

		op.Errorf("container shutdown failed for %s: %s", id, err)

		if !timeout {
			return err // error other than timeout
		}

		op.Warnf("timeout (%s) waiting for %s to power off via SIG%s", wait, id, sig)
	}

	return fmt.Errorf("failed to shutdown %s via kill signals %s", id, stop)
}

func (c *containerBase) waitForPowerState(op *trace.Operation, max time.Duration, state types.VirtualMachinePowerState) (bool, error) {
	defer trace.End(trace.BeginOp(op, "wait for power state %s: %s", c.ExecConfig.ID, state))

	timeout, cancel := trace.WithTimeout(op, max, "wait for power state")
	defer cancel()

	err := c.vm.WaitForPowerState(timeout, state)
	if err != nil {
		return timeout.Err() == err, err
	}

	return false, nil
}
