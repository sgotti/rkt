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

package image

import (
	"container/list"
	"errors"
	"fmt"
	"net/url"
	"os"
	"runtime"

	"github.com/coreos/rkt/common"
	"github.com/coreos/rkt/common/apps"
	dist "github.com/coreos/rkt/common/distribution"
	"github.com/coreos/rkt/stage0"
	"github.com/coreos/rkt/store/imagestore"
	"github.com/hashicorp/errwrap"

	"github.com/appc/spec/discovery"
	"github.com/appc/spec/schema/types"
)

// Fetcher will try to fetch images into the store.
type Fetcher action

// FetchImages uses FetchImage to attain a list of image hashes
func (f *Fetcher) FetchImages(al *apps.Apps) error {
	return al.Walk(func(app *apps.App) error {
		d, err := DistFromImageString(app.Image)
		if err != nil {
			return err
		}
		h, err := f.FetchImage(d, app.Asc)
		if err != nil {
			return err
		}
		app.ImageID = *h
		return nil
	})
}

// FetchImage will take an image as either a path, a URL or a name
// string and import it into the store if found. If ascPath is not "",
// it must exist as a local file and will be used as the signature
// file for verification, unless verification is disabled. If
// f.WithDeps is true also image dependencies are fetched.
func (f *Fetcher) FetchImage(d dist.Distribution, ascPath string) (*types.Hash, error) {
	ensureLogger(f.Debug)
	a := f.getAsc(ascPath)
	hash, err := f.fetchSingleImage(d, a)
	if err != nil {
		return nil, err
	}
	if f.WithDeps {
		err = f.fetchImageDeps(hash)
		if err != nil {
			return nil, err
		}
	}
	// we need to be able to do a chroot and access to the tree store
	// directories, check if we're root
	if common.SupportsOverlay() && os.Geteuid() == 0 {
		if _, err := f.Ts.Render(hash, false); err != nil {
			return nil, errwrap.Wrap(errors.New("error rendering tree store"), err)
		}
	}
	h, err := types.NewHash(hash)
	if err != nil {
		// should never happen
		log.PanicE("got an invalid hash, looks like it is corrupted", err)
	}
	return h, nil
}

func (f *Fetcher) getAsc(ascPath string) *asc {
	if ascPath != "" {
		return &asc{
			Location: ascPath,
			Fetcher:  &localAscFetcher{},
		}
	}
	return &asc{}
}

// fetchImageDeps will recursively fetch all the image dependencies
func (f *Fetcher) fetchImageDeps(hash string) error {
	imgsl := list.New()
	seen := map[string]struct{}{}
	f.addImageDeps(hash, imgsl, seen)
	for el := imgsl.Front(); el != nil; el = el.Next() {
		a := &asc{}
		distStr := el.Value.(string)
		d, err := dist.NewAppc(distStr)
		if err != nil {
			return err
		}
		hash, err := f.fetchSingleImage(d, a)
		if err != nil {
			return err
		}
		f.addImageDeps(hash, imgsl, seen)
	}
	return nil
}

func (f *Fetcher) addImageDeps(hash string, imgsl *list.List, seen map[string]struct{}) error {
	dependencies, err := f.getImageDeps(hash)
	if err != nil {
		return errwrap.Wrap(fmt.Errorf("failed to get dependencies for image ID %q", hash), err)
	}
	for _, d := range dependencies {
		imgName := d.ImageName.String()
		app, err := discovery.NewApp(imgName, d.Labels.ToMap())
		if err != nil {
			return errwrap.Wrap(fmt.Errorf("one of image ID's %q dependencies (image %q) is invalid", hash, imgName), err)
		}
		d := dist.NewAppcFromApp(app)
		distStr := d.ComparableURIString()
		// To really catch already seen deps the saved string must be a
		// reproducible string keeping the labels order
		if _, ok := seen[distStr]; ok {
			continue
		}
		imgsl.PushBack(distStr)
		seen[distStr] = struct{}{}
	}
	return nil
}

func (f *Fetcher) getImageDeps(hash string) (types.Dependencies, error) {
	key, err := f.S.ResolveKey(hash)
	if err != nil {
		return nil, err
	}
	im, err := f.S.GetImageManifest(key)
	if err != nil {
		return nil, err
	}
	return im.Dependencies, nil
}

func (f *Fetcher) fetchSingleImage(d dist.Distribution, a *asc) (string, error) {

	switch d.Type() {
	case dist.DistTypeACIArchive:
		return f.fetchACIArchive(d.(*dist.ACIArchive), a)
	case dist.DistTypeAppc:
		return f.fetchSingleImageByName(d.(*dist.Appc), a)
	case dist.DistTypeDocker:
		return f.fetchSingleImageByDockerURL(d.(*dist.Docker))
	default:
		return "", fmt.Errorf("unknown distribution type %d", d.Type())
	}
}

func (f *Fetcher) fetchACIArchive(d *dist.ACIArchive, a *asc) (string, error) {
	u := d.ArchiveURL()

	switch u.Scheme {
	case "http", "https":
		return f.fetchSingleImageByHTTPURL(u, a)
	case "file":
		return f.fetchSingleImageByPath(u.Path, a)
	case "":
		return "", fmt.Errorf("expected image URL %q to contain a scheme", u.String())
	default:
		return "", fmt.Errorf("an unsupported URL scheme %q - the only URL schemes supported by rkt for an archive are http, https and file", u.Scheme)
	}
}

func (f *Fetcher) fetchSingleImageByHTTPURL(u *url.URL, a *asc) (string, error) {
	rem, err := remoteForURL(f.S, u)
	if err != nil {
		return "", err
	}
	if h := f.maybeCheckRemoteFromStore(rem); h != "" {
		return h, nil
	}
	if h, err := f.maybeFetchHTTPURLFromRemote(rem, u, a); h != "" || err != nil {
		return h, err
	}
	return "", fmt.Errorf("unable to fetch image from URL %q: either image was not found in the store or store was disabled and fetching from remote yielded nothing or it was disabled", u.String())
}

func (f *Fetcher) fetchSingleImageByDockerURL(d *dist.Docker) (string, error) {
	urlStr := "docker://" + d.DockerString()
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	rem, err := remoteForURL(f.S, u)
	if err != nil {
		return "", err
	}
	if h := f.maybeCheckRemoteFromStore(rem); h != "" {
		return h, nil
	}
	if h, err := f.maybeFetchDockerURLFromRemote(u); h != "" || err != nil {
		return h, err
	}
	return "", fmt.Errorf("unable to fetch docker image from URL %q: either image was not found in the store or store was disabled and fetching from remote yielded nothing or it was disabled", u.String())
}

func (f *Fetcher) maybeCheckRemoteFromStore(rem *imagestore.Remote) string {
	if f.NoStore || rem == nil {
		return ""
	}
	log.Printf("using image from local store for url %s", rem.ACIURL)
	return rem.BlobKey
}

func (f *Fetcher) maybeFetchHTTPURLFromRemote(rem *imagestore.Remote, u *url.URL, a *asc) (string, error) {
	if !f.StoreOnly {
		log.Printf("remote fetching from URL %q", u.String())
		hf := &httpFetcher{
			InsecureFlags: f.InsecureFlags,
			S:             f.S,
			Ks:            f.Ks,
			Rem:           rem,
			Debug:         f.Debug,
			Headers:       f.Headers,
		}
		return hf.Hash(u, a)
	}
	return "", nil
}

func (f *Fetcher) maybeFetchDockerURLFromRemote(u *url.URL) (string, error) {
	if !f.StoreOnly {
		log.Printf("remote fetching from URL %q", u.String())
		df := &dockerFetcher{
			InsecureFlags: f.InsecureFlags,
			DockerAuth:    f.DockerAuth,
			S:             f.S,
			Debug:         f.Debug,
		}
		return df.Hash(u)
	}
	return "", nil
}

func (f *Fetcher) fetchSingleImageByPath(path string, a *asc) (string, error) {
	log.Printf("using image from file %s", path)
	ff := &fileFetcher{
		InsecureFlags: f.InsecureFlags,
		S:             f.S,
		Ks:            f.Ks,
		Debug:         f.Debug,
	}
	return ff.Hash(path, a)
}

type appBundle struct {
	App *discovery.App
	Str string
}

func newAppBundle(name string) (*appBundle, error) {
	app, err := discovery.NewAppFromString(name)
	if err != nil {
		return nil, errwrap.Wrap(fmt.Errorf("invalid image name %q", name), err)
	}
	if _, ok := app.Labels["arch"]; !ok {
		app.Labels["arch"] = runtime.GOARCH
	}
	if _, ok := app.Labels["os"]; !ok {
		app.Labels["os"] = runtime.GOOS
	}
	if err := types.IsValidOSArch(app.Labels, stage0.ValidOSArch); err != nil {
		return nil, errwrap.Wrap(fmt.Errorf("invalid image name %q", name), err)
	}
	bundle := &appBundle{
		App: app,
		Str: name,
	}
	return bundle, nil
}

func (f *Fetcher) fetchSingleImageByName(d *dist.Appc, a *asc) (string, error) {
	app, err := newAppBundle(AppcSimpleString(d))
	if err != nil {
		return "", err
	}
	if h, err := f.maybeCheckStoreForApp(app); h != "" || err != nil {
		return h, err
	}
	if h, err := f.maybeFetchImageFromRemote(app, a); h != "" || err != nil {
		return h, err
	}
	return "", fmt.Errorf("unable to fetch image from image name %q: either image was not found in the store or store was disabled and fetching from remote yielded nothing or it was disabled", app.Str)
}

func (f *Fetcher) maybeCheckStoreForApp(app *appBundle) (string, error) {
	if !f.NoStore {
		key, err := f.getStoreKeyFromApp(app)
		if err == nil {
			log.Printf("using image from local store for image name %s", app.Str)
			return key, nil
		}
		switch err.(type) {
		case imagestore.ACINotFoundError:
			// ignore the "not found" error
		default:
			return "", err
		}
	}
	return "", nil
}

func (f *Fetcher) getStoreKeyFromApp(app *appBundle) (string, error) {
	labels, err := types.LabelsFromMap(app.App.Labels)
	if err != nil {
		return "", errwrap.Wrap(fmt.Errorf("invalid labels in the name %q", app.Str), err)
	}
	key, err := f.S.GetACI(app.App.Name, labels)
	if err != nil {
		switch err.(type) {
		case imagestore.ACINotFoundError:
			return "", err
		default:
			return "", errwrap.Wrap(fmt.Errorf("cannot find image %q", app.Str), err)
		}
	}
	return key, nil
}

func (f *Fetcher) maybeFetchImageFromRemote(app *appBundle, a *asc) (string, error) {
	if !f.StoreOnly {
		nf := &nameFetcher{
			InsecureFlags:      f.InsecureFlags,
			S:                  f.S,
			Ks:                 f.Ks,
			NoCache:            f.NoCache,
			Debug:              f.Debug,
			Headers:            f.Headers,
			TrustKeysFromHTTPS: f.TrustKeysFromHTTPS,
		}
		return nf.Hash(app.App, a)
	}
	return "", nil
}
