package store

import (
	"database/sql"
	"fmt"
)

type migrateFunc func(*sql.Tx) error

var (
	migrateTable = map[int]migrateFunc{
		1: migrateToV1,
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
