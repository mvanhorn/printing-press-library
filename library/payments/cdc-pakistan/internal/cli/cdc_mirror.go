// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/payments/cdc-pakistan/internal/store"
)

// cdcMirrorReady reports whether the local store actually holds the CDC tables.
//
// WHY A FILE-EXISTENCE CHECK IS NOT ENOUGH. The generated store CREATES the
// database file (with the framework's own tables) the first time any command
// opens it. So on a fresh install the file exists while the CDC tables do not,
// and an os.Stat guard never fires -- the query then fails with a raw
// "no such table: cdc_documents" SQL error, which is a bad answer to "have I
// synced yet?".
//
// This checks for the table itself, so an unsynced store reports an honest
// empty mirror instead of leaking a SQL logic error.
func cdcMirrorReady(ctx context.Context, db *store.Store, table string) (bool, error) {
	var name string
	err := db.DB().QueryRowContext(ctx,
		`SELECT name FROM sqlite_master WHERE type='table' AND name = ?`, table).Scan(&name)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking for table %s: %w", table, err)
	}
	return true, nil
}

// openCDCMirror opens the store read-only and reports whether the named CDC
// table is present. A caller gets (nil, false, nil) when there is nothing to
// read, and must emit an empty typed result rather than an error.
func openCDCMirror(ctx context.Context, dbPath, table string) (*store.Store, bool, error) {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, false, nil
	}
	db, err := store.OpenReadOnlyContext(ctx, dbPath)
	if err != nil {
		return nil, false, configErr(fmt.Errorf("opening store: %w", err))
	}
	ready, err := cdcMirrorReady(ctx, db, table)
	if err != nil {
		db.Close()
		return nil, false, apiErr(err)
	}
	if !ready {
		db.Close()
		return nil, false, nil
	}
	return db, true, nil
}
