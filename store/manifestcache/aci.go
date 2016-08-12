package manifestcache

import (
	"archive/tar"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"github.com/appc/spec/schema"
	"github.com/coreos/rkt/store/casref/rwcasref"
	"github.com/hashicorp/errwrap"
	"github.com/peterbourgon/diskv"
)

const (
	defaultPathPerm = os.FileMode(0770 | os.ModeSetgid)
	defaultFilePerm = os.FileMode(0660)
)

type ACIManifestCache struct {
	dir string
	s   *rwcasref.Store

	cache *diskv.Diskv
}

func NewACIManifestCache(dir string, s *rwcasref.Store) (*ACIManifestCache, error) {
	// We need to allow the store's setgid bits (if any) to propagate, so
	// disable umask
	um := syscall.Umask(0)
	defer syscall.Umask(um)

	c := &ACIManifestCache{
		dir: dir,
		s:   s,
	}

	c.cache = diskv.New(diskv.Options{
		PathPerm:  defaultPathPerm,
		FilePerm:  defaultFilePerm,
		BasePath:  filepath.Join(dir, "cache"),
		Transform: blockTransform,
	})

	return c, nil
}

func (c *ACIManifestCache) GetManifestJSON(digest string) ([]byte, error) {
	if c.cache.Has(digest) {
		imj, err := c.cache.Read(digest)
		if err != nil {
			return nil, err
		}
		// Check if the manifest can be unmarshalled before returning
		// it, or if not (corrupted?), remove it from the cache
		var im *schema.ImageManifest
		if err = json.Unmarshal(imj, &im); err != nil {
			// ignore error since we already return with an error
			c.cache.Erase(digest)
			return nil, errwrap.Wrap(errors.New("error unmarshalling image manifest"), err)
		}
		return imj, nil
	}

	fh, err := c.s.ReadBlob(digest)
	imj, err := manifestFromImage(fh)
	if err != nil {
		return nil, errwrap.Wrap(errors.New("error extracting image manifest"), err)
	}
	var im *schema.ImageManifest
	if err = json.Unmarshal(imj, &im); err != nil {
		return nil, errwrap.Wrap(errors.New("error unmarshalling image manifest"), err)
	}
	// save the manifest in the cache
	c.cache.Write(digest, imj)

	return imj, nil
}

func (c *ACIManifestCache) GetManifest(digest string) (*schema.ImageManifest, error) {
	imj, err := c.GetManifestJSON(digest)
	if err != nil {
		return nil, err
	}

	var im *schema.ImageManifest
	if err = json.Unmarshal(imj, &im); err != nil {
		return nil, errwrap.Wrap(errors.New("error unmarshalling image manifest"), err)
	}
	return im, nil
}

// manifestFromImage extracts the manifestn from the given ACI image.
func manifestFromImage(rs io.Reader) ([]byte, error) {
	tr := tar.NewReader(rs)

	for {
		hdr, err := tr.Next()
		switch err {
		case io.EOF:
			return nil, errors.New("missing manifest")
		case nil:
			if filepath.Clean(hdr.Name) == "manifest" {
				return ioutil.ReadAll(tr)
			}
		default:
			return nil, fmt.Errorf("error extracting tarball: %v", err)
		}
	}
}
