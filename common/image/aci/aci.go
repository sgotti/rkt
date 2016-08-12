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

package aci

import (
	"errors"
	"io"

	"github.com/appc/spec/aci"
	"github.com/coreos/rkt/common/mediatype"
	"github.com/coreos/rkt/pkg/digest"
	"github.com/coreos/rkt/store/casref/rwcasref"
	"github.com/hashicorp/errwrap"
)

// WriteACI takes an ACI encapsulated in an io.Reader, decompresses it if
// necessary, and then stores it in the store under a digest based on the hash
// of the uncompressed ACI.
func WriteACI(s *rwcasref.Store, r io.ReadSeeker) (string, error) {
	dr, err := aci.NewCompressedReader(r)
	if err != nil {
		return "", errwrap.Wrap(errors.New("error decompressing image"), err)
	}
	defer dr.Close()

	return s.WriteBlob(dr, string(mediatype.ACI), nil, digest.SHA512)
}

func ReadACI(s *rwcasref.Store, digest string) (io.ReadCloser, error) {
	// TODO(sgotti) Check that the ACIInfo blob data exists. If not it
	// means something broke between writing the blob and setting the data.
	return s.ReadBlob(digest)
}
