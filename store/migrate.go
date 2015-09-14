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

package store

import (
	"database/sql"
	"fmt"
	"time"
)

type migrateFunc func(*sql.Tx) error

var (
	// migrateTable is a map of migrate functions. The key is the db
	// version to migrate to.
	migrateTable = map[int]migrateFunc{
		1: migrateToV1,
		2: migrateToV2,
		3: migrateToV3,
	}
)

func migrate(tx *sql.Tx, finalVersion int) error {
	if finalVersion > dbVersion {
		return fmt.Errorf("required migrate final version greater than the last supported db version")
	}
	version, err := getDBVersion(tx)
	if err != nil {
		return err
	}

	for v := version + 1; v <= finalVersion; v++ {
		f, ok := migrateTable[v]
		if !ok {
			return fmt.Errorf("missing migrate function for version %d", v)
		}
		err := f(tx)
		if err == nil {
			updateDBVersion(tx, v)
		}
		if err != nil {
			return fmt.Errorf("failed to migrate db to version %d: %v", v, err)
		}
	}
	return nil
}

func migrateToV1(tx *sql.Tx) error {
	return nil
}

func migrateToV2(tx *sql.Tx) error {
	_, err := tx.Exec("ALTER TABLE remote ADD cachemaxage int")
	if err != nil {
		return err
	}
	_, err = tx.Exec("ALTER TABLE remote ADD downloadedtime time")
	if err != nil {
		return err
	}
	// Set the default values for the new columns on the current rows
	_, err = tx.Exec("UPDATE remote cachemaxage = 0")
	if err != nil {
		return err
	}
	t := time.Time{}.UTC()
	_, err = tx.Exec("UPDATE remote downloadedtime = $1", t)
	if err != nil {
		return err
	}
	return nil
}

func migrateToV3(tx *sql.Tx) error {
	for _, t := range []string{
		"CREATE TABLE aciinfo_tmp (blobkey string, name string, importtime time, latest bool);",
		"CREATE UNIQUE INDEX IF NOT EXISTS blobkeyidx ON aciinfo_tmp (blobkey)",
		"CREATE INDEX IF NOT EXISTS nameidx ON aciinfo_tmp (name)",
		"INSERT INTO aciinfo_tmp (blobkey, name, importtime, latest) SELECT blobkey, appname, importtime, latest from aciinfo",
		"DROP TABLE aciinfo",
		"CREATE TABLE aciinfo (blobkey string, name string, importtime time, latest bool);",
		"CREATE UNIQUE INDEX IF NOT EXISTS blobkeyidx ON aciinfo (blobkey)",
		"CREATE INDEX IF NOT EXISTS nameidx ON aciinfo (name)",
		"INSERT INTO aciinfo SELECT * from aciinfo_tmp",
		"DROP TABLE aciinfo_tmp",
	} {
		_, err := tx.Exec(t)
		if err != nil {
			return err
		}
	}
	return nil
}
