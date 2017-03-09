// Copyright 2017 VMware, Inc. All Rights Reserved.
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

package task

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/portlayer/exec"
	"github.com/vmware/vic/pkg/trace"

	log "github.com/Sirupsen/logrus"
)

// Toggle launching of the process in the container
func toggleActive(ctx context.Context, h interface{}, id string, active bool) (interface{}, error) {
	defer trace.End(trace.Begin(""))

	handle, ok := h.(*exec.Handle)
	if !ok {
		return nil, fmt.Errorf("Type assertion failed for %#+v", handle)
	}

	// if the container isn't running then this is a persistent change
	tasks := handle.ExecConfig.Sessions
	if handle.Runtime != nil && handle.Runtime.PowerState != types.VirtualMachinePowerStatePoweredOff {
		log.Debug("Task configuration applies to ephemeral set")
		tasks = handle.ExecConfig.Execs

		// TODO: add check for container version - if the tether doesn't support reload/exec then
		// this should fail
	}

	task, ok := tasks[id]
	if !ok {
		return nil, fmt.Errorf("unknown persistent task ID: %s", id)
	}

	task.Active = active
	handle.Reload()

	return handle, nil
}
