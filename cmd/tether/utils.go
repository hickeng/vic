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
	"net"

	"github.com/vmware/vic/metadata"
	"golang.org/x/net/context"
)

// ops is here so we can switch the OS Ops impls for test mocking
var ops osops

type osops interface {
	SetHostname(hostname string) error
	Apply(endpoint *metadata.NetworkEndpoint) error
	MountLabel(label, target string, ctx context.Context) error
	Fork(config *metadata.ExecutorConfig) error
	Backchannel(ctx context.Context) (net.Conn, error)
}
