// Copyright 2026 NicholasSpisak and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"os"

	"github.com/mvanhorn/printing-press-library/library/commerce/shopping/internal/store"
)

// shopStoreDBEnv lets tests and operators point the novel commands at a
// throwaway SQLite file without touching the default share path.
const shopStoreDBEnv = "SHOPPING_DB"

// resolveShopDBPath picks the SQLite path for the novel commands. Precedence:
// explicit --db flag, then the SHOPPING_DB env var, then the default share
// path. An empty result is never returned for a real open.
func resolveShopDBPath(flagDB string) string {
	if flagDB != "" {
		return flagDB
	}
	if env := os.Getenv(shopStoreDBEnv); env != "" {
		return env
	}
	return defaultDBPath("shopping-pp-cli")
}

// openShopStore opens (creating if needed) the local store for novel
// commands and guarantees the novel-feature auxiliary tables exist. Callers
// must Close the returned store.
func openShopStore(ctx context.Context, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("shopping-pp-cli")
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	if err := db.EnsureShoppingExtras(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// nullableFloat returns a *float64 (nil for SQL NULL) so JSON renders an
// honest null rather than a misleading 0 for missing numeric fields.
func nullableFloat(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	f := v.Float64
	return &f
}

// nullableString returns a *string (nil for SQL NULL) so JSON renders null
// for missing text fields.
func nullableString(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	str := v.String
	return &str
}

// nullableBool maps an integer-or-boolean json_extract result to *bool.
// SQLite json_extract returns 1/0 for JSON booleans; a NULL stays nil.
func nullableBool(v sql.NullInt64) *bool {
	if !v.Valid {
		return nil
	}
	b := v.Int64 != 0
	return &b
}
