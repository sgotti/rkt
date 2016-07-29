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
	"errors"
	"fmt"
	"time"

	"github.com/coreos/rkt/common"
	"github.com/coreos/rkt/pkg/lock"
	"github.com/coreos/rkt/store/imagestore"
	"github.com/coreos/rkt/store/treestore"
	"github.com/hashicorp/errwrap"
	"github.com/spf13/cobra"
)

const (
	defaultImageGracePeriod = 24 * time.Hour
)

var (
	cmdImageGC = &cobra.Command{
		Use:   "gc",
		Short: "Garbage collect local store",
		Long: `This is intended to be run periodically from a timer or cron job.

The default grace period is 24h. Use --grace-period=0s to effectively disable
the grace-period.`,
		Run: runWrapper(runGCImage),
	}
	flagImageGracePeriod time.Duration
)

func init() {
	cmdImage.AddCommand(cmdImageGC)
	cmdImageGC.Flags().DurationVar(&flagImageGracePeriod, "grace-period", defaultImageGracePeriod, "duration to wait since an image was last used before removing it")
}

func runGCImage(cmd *cobra.Command, args []string) (exit int) {
	s, err := imagestore.NewStore(storeDir())
	if err != nil {
		stderr.PrintE("cannot open store", err)
		return 1
	}

	ts, err := newTreeStore(s)
	if err != nil {
		stderr.PrintE("cannot open store", err)
		return
	}

	if err := gcTreeStore(ts); err != nil {
		stderr.PrintE("failed to remove unreferenced treestores", err)
		return 1
	}

	if err := gcStore(s, flagImageGracePeriod); err != nil {
		stderr.Error(err)
		return 1
	}

	return 0
}

// gcTreeStore removes all treeStoreIDs not referenced by any non garbage
// collected pod from the store.
func gcTreeStore(ts *treestore.Store) error {
	// Take an exclusive lock to block other pods being created.
	// This is needed to avoid races between the below steps (getting the
	// list of referenced treeStoreIDs, getting the list of treeStoreIDs
	// from the store, removal of unreferenced treeStoreIDs) and new
	// pods/treeStores being created/referenced
	keyLock, err := lock.ExclusiveKeyLock(lockDir(), common.PrepareLock)
	if err != nil {
		return errwrap.Wrap(errors.New("cannot get exclusive prepare lock"), err)
	}
	defer keyLock.Close()
	referencedTreeStoreIDs, err := getReferencedTreeStoreIDs()
	if err != nil {
		return errwrap.Wrap(errors.New("cannot get referenced treestoreIDs"), err)
	}
	treeStoreIDs, err := ts.ListIDs()
	if err != nil {
		return errwrap.Wrap(errors.New("cannot get treestoreIDs from the store"), err)
	}
	errors := 0
	for _, treeStoreID := range treeStoreIDs {
		if _, ok := referencedTreeStoreIDs[treeStoreID]; !ok {
			if err := ts.Remove(treeStoreID); err != nil {
				stderr.PrintE(fmt.Sprintf("error removing treestore %q", treeStoreID), err)
				errors++
			} else {
				stderr.Printf("removed treestore %q", treeStoreID)
			}
		}
	}
	if errors > 0 {
		return fmt.Errorf("failed to remove %d treestores", errors)
	}
	return nil
}

func getReferencedTreeStoreIDs() (map[string]struct{}, error) {
	treeStoreIDs := map[string]struct{}{}
	// Consider pods in preparing, prepared, run, exitedgarbage state
	if err := walkPods(includeMostDirs, func(p *pod) {
		stage1TreeStoreID, err := p.getStage1TreeStoreID()
		if err != nil {
			stderr.PrintE(fmt.Sprintf("cannot get stage1 treestoreID for pod %s", p.uuid), err)
			return
		}
		appsTreeStoreIDs, err := p.getAppsTreeStoreIDs()
		if err != nil {
			stderr.PrintE(fmt.Sprintf("cannot get apps treestoreID for pod %s", p.uuid), err)
			return
		}
		allTreeStoreIDs := append(appsTreeStoreIDs, stage1TreeStoreID)

		for _, treeStoreID := range allTreeStoreIDs {
			treeStoreIDs[treeStoreID] = struct{}{}
		}
	}); err != nil {
		return nil, errwrap.Wrap(errors.New("failed to get pod handles"), err)
	}
	return treeStoreIDs, nil
}

func gcStore(s *imagestore.Store, gracePeriod time.Duration) error {
	var imagesToRemove []string
	aciinfos, err := s.GetAllACIInfos([]string{"lastused"}, true)
	if err != nil {
		return errwrap.Wrap(errors.New("failed to get aciinfos"), err)
	}
	for _, ai := range aciinfos {
		if time.Now().Sub(ai.LastUsed) <= gracePeriod {
			break
		}
		imagesToRemove = append(imagesToRemove, ai.BlobKey)
	}

	if err := rmImages(s, imagesToRemove); err != nil {
		return err
	}

	return nil
}
