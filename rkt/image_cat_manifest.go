// Copyright 2015 The rkt Authors
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

package main

import (
	"encoding/json"

	"github.com/coreos/rkt/store"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/spf13/cobra"
)

var (
	cmdImageCatManifest = &cobra.Command{
		Use:   "cat-manifest IMAGE",
		Short: "Inspect and print the image manifest",
		Long:  `IMAGE should be a string referencing an image; either a hash or an image name.`,
		Run:   runWrapper(runImageCatManifest),
	}
	flagPrettyPrint bool
)

func init() {
	cmdImage.AddCommand(cmdImageCatManifest)
	cmdImageCatManifest.Flags().BoolVar(&flagPrettyPrint, "pretty-print", false, "apply indent to format the output")
}

func runImageCatManifest(cmd *cobra.Command, args []string) (exit int) {
	if len(args) != 1 {
		cmd.Usage()
		return 1
	}

	s, err := store.NewStore(globalFlags.Dir)
	if err != nil {
		stderr("image cat-manifest: cannot open store: %v", err)
		return 1
	}

	key, err := getStoreKeyFromAppOrHash(s, args[0])
	if err != nil {
		stderr("image cat-manifest: %v", err)
		return 1
	}

	manifest, err := s.GetImageManifest(key)
	if err != nil {
		stderr("image cat-manifest: cannot get image manifest: %v", err)
		return 1
	}

	var b []byte
	if flagPrettyPrint {
		b, err = json.MarshalIndent(manifest, "", "\t")
	} else {
		b, err = json.Marshal(manifest)
	}
	if err != nil {
		stderr("image cat-manifest: cannot read the image manifest: %v", err)
		return 1
	}

	stdout(string(b))
	return 0
}
