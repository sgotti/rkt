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

import (
	"net/url"
	"testing"

	"github.com/appc/spec/discovery"
)

func TestNewAppcFromAppString(t *testing.T) {
	tests := []struct {
		in     string
		uriStr string
	}{
		{
			"example.com/app01",
			"cimd:appc:v=0:example.com/app01",
		},
		{
			"example.com/app01:v1.0.0",
			"cimd:appc:v=0:example.com/app01?version=v1.0.0",
		},
		{
			"example.com/app01,version=v1.0.0",
			"cimd:appc:v=0:example.com/app01?version=v1.0.0",
		},
		{
			"example.com/app01,version=v1.0.0,label01=?&*/",
			"cimd:appc:v=0:example.com/app01?label01=%3F%26%2A%2F&version=v1.0.0",
		},
	}

	for _, tt := range tests {
		app, err := discovery.NewAppFromString(tt.in)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		appc := NewAppcFromApp(app)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		u, err := url.Parse(tt.uriStr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		td, err := NewAppc(u)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !appc.Compare(td) {
			t.Fatalf("expected identical distribution but got %q != %q", td.URI().String(), appc.URI().String())
		}
	}
}
