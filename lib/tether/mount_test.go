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

package tether

import (
	"net/url"
	"testing"

	"github.com/vmware/vic/lib/metadata"
)

func TestBasicMount(t *testing.T) {
	testSetup(t)
	defer testTeardown(t)

	cfg := metadata.ExecutorConfig{
		Common: metadata.Common{
			ID:   "sethostname",
			Name: "tether_test_executor",
		},
		Mounts: map[string]metadata.MountSpec{
			"mydata": metadata.MountSpec{
				Source: url.URL{
					Scheme: "label",
					Host:   "0xdeadbeef",
				},
				Path: "/var/lib/data",
			},
		},
	}

	tthr, _ := StartTether(t, &cfg)

	<-Mocked.Started

	// prevent indefinite wait in tether - normally session exit would trigger this
	tthr.Stop()

	// wait for tether to exit
	<-Mocked.Cleaned

	assertEqual(t, len(Mocked.Mounts), 1, "Expected one mount in map")
	assertEqual(t, Mocked.Mounts["0xdeadbeef"], "/var/lib/data", "Expected label to map to target")
}
