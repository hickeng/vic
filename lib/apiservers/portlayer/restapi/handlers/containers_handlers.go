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

package handlers

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	middleware "github.com/go-swagger/go-swagger/httpkit/middleware"
	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"

	"net/http"

	"github.com/vmware/vic/lib/apiservers/portlayer/models"
	"github.com/vmware/vic/lib/apiservers/portlayer/restapi/operations"
	"github.com/vmware/vic/lib/apiservers/portlayer/restapi/operations/containers"
	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/lib/portlayer/exec"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/uid"
	"github.com/vmware/vic/pkg/version"
)

const containerWaitTimeout = 3 * time.Minute

// ContainersHandlersImpl is the receiver for all of the exec handler methods
type ContainersHandlersImpl struct {
	handlerCtx *HandlerContext
}

// Configure assigns functions to all the exec api handlers
func (handler *ContainersHandlersImpl) Configure(api *operations.PortLayerAPI, handlerCtx *HandlerContext) {
	api.ContainersCreateHandler = containers.CreateHandlerFunc(handler.CreateHandler)
	api.ContainersStateChangeHandler = containers.StateChangeHandlerFunc(handler.StateChangeHandler)
	api.ContainersGetHandler = containers.GetHandlerFunc(handler.GetHandler)
	api.ContainersCommitHandler = containers.CommitHandlerFunc(handler.CommitHandler)
	api.ContainersGetStateHandler = containers.GetStateHandlerFunc(handler.GetStateHandler)
	api.ContainersContainerRemoveHandler = containers.ContainerRemoveHandlerFunc(handler.RemoveContainerHandler)
	api.ContainersGetContainerInfoHandler = containers.GetContainerInfoHandlerFunc(handler.GetContainerInfoHandler)
	api.ContainersGetContainerListHandler = containers.GetContainerListHandlerFunc(handler.GetContainerListHandler)
	api.ContainersContainerSignalHandler = containers.ContainerSignalHandlerFunc(handler.ContainerSignalHandler)
	api.ContainersGetContainerLogsHandler = containers.GetContainerLogsHandlerFunc(handler.GetContainerLogsHandler)
	api.ContainersContainerWaitHandler = containers.ContainerWaitHandlerFunc(handler.ContainerWaitHandler)

	handler.handlerCtx = handlerCtx
}

// CreateHandler creates a new container
func (handler *ContainersHandlersImpl) CreateHandler(params containers.CreateParams) middleware.Responder {
	var err error
	id := uid.New().String()
	name := *params.CreateConfig.Name

	op := trace.NewOperation(context.Background(), "create container")
	defer trace.End(trace.BeginOp(&op, "create %s:%s", name, id))

	session := handler.handlerCtx.Session

	op.Infof("Path: %#v", params.CreateConfig.Path)
	op.Debugf("WorkingDir: %#v", params.CreateConfig.WorkingDir)

	// Init key for tether
	privateKey, err := rsa.GenerateKey(rand.Reader, 512)
	if err != nil {
		return containers.NewCreateNotFound().WithPayload(&models.Error{Message: err.Error()})
	}
	privateKeyBlock := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   x509.MarshalPKCS1PrivateKey(privateKey),
	}

	m := &executor.ExecutorConfig{
		Common: executor.Common{
			ID:   id,
			Name: name,
		},
		CreateTime: time.Now().UTC().Unix(),
		Version:    version.GetBuild(),
		Sessions: map[string]*executor.SessionConfig{
			id: &executor.SessionConfig{
				Common: executor.Common{
					ID:   id,
					Name: name,
				},
				Tty:    *params.CreateConfig.Tty,
				Attach: *params.CreateConfig.Attach,
				Cmd: executor.Cmd{
					Env:  params.CreateConfig.Env,
					Dir:  *params.CreateConfig.WorkingDir,
					Path: *params.CreateConfig.Path,
					Args: append([]string{*params.CreateConfig.Path}, params.CreateConfig.Args...),
				},
				StopSignal: *params.CreateConfig.StopSignal,
			},
		},
		Key:      pem.EncodeToMemory(&privateKeyBlock),
		LayerID:  *params.CreateConfig.Image,
		RepoName: *params.CreateConfig.RepoName,
	}

	if params.CreateConfig.Annotations != nil && len(params.CreateConfig.Annotations) > 0 {
		m.Annotations = make(map[string]string)
		for k, v := range params.CreateConfig.Annotations {
			m.Annotations[k] = v
		}
	}

	// Create the executor.ExecutorCreateConfig
	c := &exec.ContainerCreateConfig{
		Metadata:       m,
		ParentImageID:  *params.CreateConfig.Image,
		ImageStoreName: params.CreateConfig.ImageStore.Name,
	}

	h, err := exec.Create(&op, session, c)
	if err != nil {
		op.Errorf("ContainerCreate error: %s", err.Error())
		return containers.NewCreateNotFound().WithPayload(&models.Error{Message: err.Error()})
	}

	//  send the container id back to the caller
	return containers.NewCreateOK().WithPayload(&models.ContainerCreatedInfo{ID: id, Handle: h.String()})
}

// StateChangeHandler changes the state of a container
func (handler *ContainersHandlersImpl) StateChangeHandler(params containers.StateChangeParams) middleware.Responder {
	h := exec.GetHandle(params.Handle)
	if h == nil {
		return containers.NewStateChangeNotFound()
	}

	id := h.ExecConfig.ID
	op := h.Operation
	defer trace.End(trace.BeginOp(op, "power change %s: %s", id, params.State))

	var state exec.State
	switch params.State {
	case "RUNNING":
		state = exec.StateRunning
	case "STOPPED":
		state = exec.StateStopped
	case "CREATED":
		state = exec.StateCreated
	default:
		return containers.NewStateChangeDefault(http.StatusServiceUnavailable).WithPayload(&models.Error{Message: "unknown state"})
	}

	h.SetTargetState(state)
	return containers.NewStateChangeOK().WithPayload(h.String())
}

func (handler *ContainersHandlersImpl) GetStateHandler(params containers.GetStateParams) middleware.Responder {

	// NOTE: I've no idea why GetStateHandler takes a handle instead of an ID - hopefully there was a reason for an inspection
	// operation to take this path
	h := exec.GetHandle(params.Handle)
	if h == nil || h.ExecConfig == nil {
		return containers.NewGetStateNotFound()
	}

	op := h.Operation
	defer trace.End(trace.BeginOp(op, "power state from handle: %s", h.String()))

	container := exec.Containers.Container(h.ExecConfig.ID)
	if container == nil {
		return containers.NewGetStateNotFound()
	}

	var state string
	switch container.CurrentState() {
	case exec.StateRunning:
		state = "RUNNING"

	case exec.StateStopped:
		state = "STOPPED"

	case exec.StateCreated:
		state = "CREATED"

	default:
		return containers.NewGetStateDefault(http.StatusServiceUnavailable)
	}

	op.Debugf("power state: %s", state)
	return containers.NewGetStateOK().WithPayload(&models.ContainerGetStateResponse{Handle: h.String(), State: state})
}

func (handler *ContainersHandlersImpl) GetHandler(params containers.GetParams) middleware.Responder {
	op := trace.NewOperation(context.Background(), "new handle")
	defer trace.End(trace.BeginOp(&op, "new handle: %s", params.ID))

	h := exec.GetContainer(&op, uid.Parse(params.ID))
	if h == nil {
		op.Errorf("no such container")
		return containers.NewGetNotFound().WithPayload(&models.Error{Message: fmt.Sprintf("container %s not found", params.ID)})
	}

	op.Infof("new handle: %s", h.String())

	return containers.NewGetOK().WithPayload(h.String())
}

func (handler *ContainersHandlersImpl) CommitHandler(params containers.CommitParams) middleware.Responder {
	h := exec.GetHandle(params.Handle)
	if h == nil {
		log.Errorf("handle not found: %s", params.Handle)
		return containers.NewCommitNotFound().WithPayload(&models.Error{Message: "container not found"})
	}

	op := h.Operation
	defer trace.End(trace.BeginOp(op, "commit handle: %s", params.Handle))

	if err := h.Commit(op, handler.handlerCtx.Session, params.WaitTime); err != nil {
		switch err := err.(type) {
		case exec.ConcurrentAccessError:
			op.Errorf("commit failed due to concurrent modification: %#v", err)

			return containers.NewCommitConflict().WithPayload(&models.Error{Message: err.Error()})
		default:
			op.Errorf("commit error: %#v", err)

			return containers.NewCommitDefault(http.StatusServiceUnavailable).WithPayload(&models.Error{Message: err.Error()})
		}
	}

	op.Errorf("committed handle")
	return containers.NewCommitOK()
}

func (handler *ContainersHandlersImpl) RemoveContainerHandler(params containers.ContainerRemoveParams) middleware.Responder {
	// NOTE: why does this not require a handle?

	// get the indicated container for removal
	cID := uid.Parse(params.ID)
	h := exec.GetContainer(nil, cID)
	if h == nil || h.ExecConfig == nil {
		return containers.NewContainerRemoveNotFound()
	}

	op := h.Operation
	defer trace.End(trace.BeginOp(op, "remove container: %s (handle: %s)", cID, h.String()))

	container := exec.Containers.Container(h.ExecConfig.ID)
	if container == nil {
		return containers.NewGetStateNotFound()
	}

	// NOTE: this should allowing batching of operations, as with Create, Start, Stop, et al
	err := container.Remove(op, handler.handlerCtx.Session)
	if err != nil {
		switch err := err.(type) {
		case exec.NotFoundError:
			return containers.NewContainerRemoveNotFound()
		case exec.RemovePowerError:
			return containers.NewContainerRemoveConflict().WithPayload(&models.Error{Message: err.Error()})
		default:
			return containers.NewContainerRemoveInternalServerError()
		}
	}

	return containers.NewContainerRemoveOK()
}

func (handler *ContainersHandlersImpl) GetContainerInfoHandler(params containers.GetContainerInfoParams) middleware.Responder {
	op := trace.NewOperation(context.Background(), fmt.Sprintf("container info: %s", params.ID))
	defer trace.End(trace.BeginOp(&op, "container info: %s", params.ID))

	container := exec.Containers.Container(params.ID)
	if container == nil {
		info := fmt.Sprintf("GetContainerInfoHandler ContainerCache miss for container(%s)", params.ID)
		log.Error(info)
		return containers.NewGetContainerInfoNotFound().WithPayload(&models.Error{Message: info})
	}

	containerInfo := convertContainerToContainerInfo(container)
	return containers.NewGetContainerInfoOK().WithPayload(containerInfo)
}

func (handler *ContainersHandlersImpl) GetContainerListHandler(params containers.GetContainerListParams) middleware.Responder {
	op := trace.NewOperation(context.Background(), "list containers")
	defer trace.End(trace.BeginOp(&op, "list containers"))

	var state *exec.State
	if params.All != nil && !*params.All {
		state = new(exec.State)
		*state = exec.StateRunning
	}

	containerVMs := exec.Containers.Containers(state)
	containerList := make([]*models.ContainerInfo, 0, len(containerVMs))

	for _, container := range containerVMs {
		// convert to return model
		info := convertContainerToContainerInfo(container)
		containerList = append(containerList, info)
	}
	return containers.NewGetContainerListOK().WithPayload(containerList)
}

func (handler *ContainersHandlersImpl) ContainerSignalHandler(params containers.ContainerSignalParams) middleware.Responder {
	op := trace.NewOperation(context.Background(), "signal container")
	defer trace.End(trace.BeginOp(&op, "signal container: %s", params.ID))

	// NOTE: I feel that this should be in a Commit path for consistency
	// it would allow phrasings such as:
	// 1. join Volume to container
	// 2. send HUP to primary process
	// Only really relevant when we can connect networks or join volumes live
	container := exec.Containers.Container(params.ID)
	if container == nil {
		return containers.NewContainerSignalNotFound().WithPayload(&models.Error{Message: fmt.Sprintf("container %s not found", params.ID)})
	}

	err := container.Signal(&op, params.Signal)
	if err != nil {
		return containers.NewContainerSignalInternalServerError().WithPayload(&models.Error{Message: err.Error()})
	}

	return containers.NewContainerSignalOK()
}

func (handler *ContainersHandlersImpl) GetContainerLogsHandler(params containers.GetContainerLogsParams) middleware.Responder {
	op := trace.NewOperation(context.Background(), "container logs")
	defer trace.End(trace.BeginOp(&op, "container logs: %s", params.ID))

	container := exec.Containers.Container(params.ID)
	if container == nil {
		return containers.NewGetContainerLogsNotFound().WithPayload(&models.Error{
			Message: fmt.Sprintf("container %s not found", params.ID),
		})
	}

	follow := false
	tail := -1

	if params.Follow != nil {
		follow = *params.Follow
	}

	if params.Taillines != nil {
		tail = int(*params.Taillines)
	}

	reader, err := container.LogReader(&op, tail, follow)
	if err != nil {
		return containers.NewGetContainerLogsInternalServerError().WithPayload(&models.Error{Message: err.Error()})
	}

	detachableOut := NewFlushingReader(reader)

	return NewContainerOutputHandler("logs").WithPayload(detachableOut, params.ID)
}

func (handler *ContainersHandlersImpl) ContainerWaitHandler(params containers.ContainerWaitParams) middleware.Responder {
	op := trace.NewOperation(context.Background(), "container wait")
	defer trace.End(trace.BeginOp(&op, "container wait: %s:%d", params.ID, params.Timeout))

	// default context timeout in seconds
	defaultTimeout := int64(containerWaitTimeout.Seconds())

	// if we have a positive timeout specified then use it
	if params.Timeout > 0 {
		defaultTimeout = params.Timeout
	}

	timeout := time.Duration(defaultTimeout) * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	c := exec.Containers.Container(uid.Parse(params.ID).String())
	if c == nil {
		return containers.NewContainerWaitNotFound().WithPayload(&models.Error{
			Message: fmt.Sprintf("container %s not found", params.ID),
		})
	}

	select {
	case <-c.WaitForState(exec.StateStopped):
		containerInfo := convertContainerToContainerInfo(c)
		return containers.NewContainerWaitOK().WithPayload(containerInfo)
	case <-ctx.Done():
		return containers.NewContainerWaitInternalServerError().WithPayload(&models.Error{
			Message: fmt.Sprintf("ContainerWaitHandler(%s) Error: %s", params.ID, ctx.Err()),
		})
	}
}

// utility function to convert from a Container type to the API Model ContainerInfo (which should prob be called ContainerDetail)
func convertContainerToContainerInfo(container *exec.Container) *models.ContainerInfo {
	defer trace.End(trace.Begin(container.ExecConfig.ID))
	// convert the container type to the required model
	info := &models.ContainerInfo{ContainerConfig: &models.ContainerConfig{}, ProcessConfig: &models.ProcessConfig{}}

	ccid := container.ExecConfig.ID
	info.ContainerConfig.ContainerID = &ccid

	s := container.CurrentState().String()
	info.ContainerConfig.State = &s
	info.ContainerConfig.LayerID = &container.ExecConfig.LayerID
	info.ContainerConfig.RepoName = &container.ExecConfig.RepoName
	info.ContainerConfig.CreateTime = &container.ExecConfig.CreateTime
	info.ContainerConfig.Names = []string{container.ExecConfig.Name}

	restart := int32(container.ExecConfig.Diagnostics.ResurrectionCount)
	info.ContainerConfig.RestartCount = &restart

	tty := container.ExecConfig.Sessions[ccid].Tty
	info.ContainerConfig.Tty = &tty

	attach := container.ExecConfig.Sessions[ccid].Attach
	info.ContainerConfig.AttachStdin = &attach
	info.ContainerConfig.AttachStdout = &attach
	info.ContainerConfig.AttachStderr = &attach

	info.ContainerConfig.StorageSize = &container.VMUnsharedDisk

	if container.ExecConfig.Annotations != nil && len(container.ExecConfig.Annotations) > 0 {
		info.ContainerConfig.Annotations = make(map[string]string)

		for k, v := range container.ExecConfig.Annotations {
			info.ContainerConfig.Annotations[k] = v
		}
	}

	path := container.ExecConfig.Sessions[ccid].Cmd.Path
	info.ProcessConfig.ExecPath = &path

	dir := container.ExecConfig.Sessions[ccid].Cmd.Dir
	info.ProcessConfig.WorkingDir = &dir

	info.ProcessConfig.ExecArgs = container.ExecConfig.Sessions[ccid].Cmd.Args
	info.ProcessConfig.Env = container.ExecConfig.Sessions[ccid].Cmd.Env

	exitcode := int32(container.ExecConfig.Sessions[ccid].ExitStatus)
	info.ProcessConfig.ExitCode = &exitcode

	startTime := container.ExecConfig.Sessions[ccid].StartTime
	info.ProcessConfig.StartTime = &startTime

	stopTime := container.ExecConfig.Sessions[ccid].StopTime
	info.ProcessConfig.StopTime = &stopTime

	// started is a string in the vmx that is not to be confused
	// with started the datetime in the models.ContainerInfo
	status := container.ExecConfig.Sessions[ccid].Started
	info.ProcessConfig.Status = &status

	info.HostConfig = &models.HostConfig{}
	for _, endpoint := range container.ExecConfig.Networks {
		if len(endpoint.Ports) > 0 {
			info.HostConfig.Ports = append(info.HostConfig.Ports, endpoint.Ports...)
		}
	}
	return info
}
