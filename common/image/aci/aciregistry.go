package aci

import (
	"crypto/sha512"
	"fmt"
	"hash"
	"io"

	"github.com/appc/spec/discovery"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
	"github.com/coreos/rkt/common/distribution"
	"github.com/coreos/rkt/store/casref/rwcasref"
	"github.com/coreos/rkt/store/manifestcache"
)

const (
	digestPrefix = "sha512-"
	lenHash      = sha512.Size // raw byte size
)

// ACINotFoundError is returned when an ACI cannot be found by GetACI
// Useful to distinguish a generic error from an aci not found.
type ACINotFoundError struct {
	name   types.ACIdentifier
	labels types.Labels
}

func (e ACINotFoundError) Error() string {
	return fmt.Sprintf("cannot find aci satisfying name: %q and labels: %s in the local store", e.name, labelsToString(e.labels))
}

type ACIRegistry struct {
	s *rwcasref.Store
	c *manifestcache.ACIManifestCache
}

func NewACIRegistry(s *rwcasref.Store, c *manifestcache.ACIManifestCache) *ACIRegistry {
	return &ACIRegistry{s: s, c: c}
}

// GetACI retrieves the ACI digest that matches the provided app name and labels.
func (r *ACIRegistry) GetACI(name types.ACIdentifier, labels types.Labels) (string, error) {
	app, err := discovery.NewApp(name.String(), labels.ToMap())
	if err != nil {
		return "", err
	}
	refID := distribution.NewAppcFromApp(app).ComparableURIString()

	fmt.Printf("refID: %s\n", refID)

	ref, err := r.s.GetRef(refID)
	if err != nil {
		return "", err
	}
	fmt.Printf("hash: %s\n", ref.Digest)
	digest := ref.Digest
	if digest == "" {
		return "", ACINotFoundError{name: name, labels: labels}
	}
	return "", ACINotFoundError{name: name, labels: labels}
}

func (r *ACIRegistry) GetImageManifest(key string) (*schema.ImageManifest, error) {
	return r.c.GetManifest(key)
}

func (r *ACIRegistry) ReadStream(key string) (io.ReadCloser, error) {
	return ReadACI(r.s, key)
}

func (r *ACIRegistry) ResolveKey(key string) (string, error) {
	return r.s.ResolveDigest(key)
}

// HashToKey takes a hash.Hash (which currently _MUST_ represent a full SHA512),
// calculates its sum, and returns a string which should be used as the key to
// store the data matching the hash.
func (r *ACIRegistry) HashToKey(h hash.Hash) string {
	s := h.Sum(nil)
	if len(s) != lenHash {
		panic(fmt.Sprintf("bad hash passed to hashToKey: %x", s))
	}
	return fmt.Sprintf("%s%x", digestPrefix, s)
}
