package store

import (
	"database/sql"
	"fmt"
	"time"
)

type migrateFunc func(*sql.Tx) error

var (
	defaultMigrateV1LastUsed = time.Now()
)

var (
	migrateTable = map[int]migrateFunc{
		1: migrateToV1,
		2: migrateToV2,
	}
)

func migrate(tx *sql.Tx, finalVersion int) error {
	version, err := getDBVersion(tx)
	if err != nil {
		return err
	}

	for v := version + 1; v <= finalVersion; v++ {
		err := migrateTable[v](tx)
		if err != nil {
			return fmt.Errorf("failed to migrate db to version %d: %v", v, err)
		}
		err = updateDBVersion(tx, v)
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
	_, err := tx.Exec("ALTER TABLE aciinfo ADD lastused time;")
	if err != nil {
		return err
	}
	// Needs to set a default time for lastused or it will contains nil values
	_, err = tx.Exec("UPDATE aciinfo lastused = $1", defaultMigrateV1LastUsed)
	if err != nil {
		return err
	}
	return nil
}
