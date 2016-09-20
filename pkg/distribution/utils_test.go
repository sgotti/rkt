// Copyright 2016 The rkt Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package distribution

import "testing"

func TestSimpleDockerString(t *testing.T) {
	tests := []struct {
		inDs     string
		simpleDs string
		err      error
	}{
		{
			"busybox",
			"busybox",
			nil,
		},
		{
			"busybox:latest",
			"busybox",
			nil,
		},
		{
			"registry-1.docker.io/library/busybox:latest",
			"busybox",
			nil,
		},
		{
			"busybox:1.0",
			"busybox:1.0",
			nil,
		},
		{
			"repo/image",
			"repo/image",
			nil,
		},
		{
			"repo/image:latest",
			"repo/image",
			nil,
		},
		{
			"repo/image:1.0",
			"repo/image:1.0",
			nil,
		},
		{
			"busybox@sha256:a59906e33509d14c036c8678d687bd4eec81ed7c4b8ce907b888c607f6a1e0e6",
			"busybox@sha256:a59906e33509d14c036c8678d687bd4eec81ed7c4b8ce907b888c607f6a1e0e6",
			nil,
		},
		{
			"myregistry.example.com:4000/busybox",
			"myregistry.example.com:4000/busybox",
			nil,
		},
		{
			"myregistry.example.com:4000/busybox:latest",
			"myregistry.example.com:4000/busybox",
			nil,
		},
		{
			"myregistry.example.com:4000/busybox:1.0",
			"myregistry.example.com:4000/busybox:1.0",
			nil,
		},
	}

	for _, tt := range tests {
		// Test NewDockerFromDockerString
		d, err := NewDockerFromDockerString(tt.inDs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ds := d.(*Docker).DockerString()
		simpleDs, err := SimpleDockerString(ds)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tt.simpleDs != simpleDs {
			t.Fatalf("expected simple string %q, but got %q", tt.simpleDs, simpleDs)
		}
	}

}