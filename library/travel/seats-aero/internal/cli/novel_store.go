// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/seats-aero/internal/store"
	"os"
)

var novelDBPathOverride string

func novelStorePath() string {
	return resolveNovelDBPath("")
}

func resolveNovelDBPath(flagDB string) string {
	if flagDB != "" {
		return flagDB
	}
	if novelDBPathOverride != "" {
		return novelDBPathOverride
	}
	return defaultDBPath("seats-aero-pp-cli")
}

func openNovelStoreAt(ctx context.Context, path string) (*store.Store, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}
	db, err := store.OpenReadOnlyContext(ctx, path)
	if err != nil {
		return nil, err
	}
	// Novel commands never migrate; availability_all is created by a read-write
	// store open from sync or doctor.
	var one int
	err = db.DB().QueryRowContext(ctx, `SELECT 1 FROM sqlite_master WHERE type='view' AND name='availability_all'`).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		_ = db.Close()
		return nil, nil
	}
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	var extrasVersion int
	if err := db.DB().QueryRowContext(ctx, `SELECT CAST(value AS INTEGER) FROM store_extras_meta WHERE key='extras_version'`).Scan(&extrasVersion); err != nil || extrasVersion < 2 {
		_ = db.Close()
		return nil, nil
	}
	return db, nil
}

func openNovelStore(ctx context.Context) (*store.Store, error) {
	return openNovelStoreAt(ctx, novelStorePath())
}

func novelStoreMissingHint(path string) string {
	if _, err := os.Stat(path); err == nil {
		return "run any read-write command (sync) once to finish the store upgrade"
	}
	return fmt.Sprintf("run: seats-aero-pp-cli sync --db %s --resources availability --resource-param availability:source=<program> --since 7d", path)
}
