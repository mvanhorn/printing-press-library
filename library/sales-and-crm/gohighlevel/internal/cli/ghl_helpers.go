// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
//
// Hand-coded helpers shared by the 11 GHL-specific commands (opp, field,
// contact, sql, recruit, config, convo). Kept out of the generator-emitted
// helpers.go so a regen of the press doesn't blow these away.
package cli

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gohighlevel/internal/store"
)

// openGHLStore opens the local SQLite cache and applies the GHL extension
// migrations (stage_transitions, pipelines, stages). It is the canonical
// entry point used by every hand-coded command that reads/writes the local
// cache. Callers are responsible for closing the returned *store.Store.
func openGHLStore(ctx context.Context) (*store.Store, error) {
	dbPath := defaultDBPath("gohighlevel-pp-cli")
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local cache at %s: %w", dbPath, err)
	}
	for _, mig := range store.GHLExtensionMigrations() {
		if _, err := s.DB().ExecContext(ctx, mig); err != nil {
			s.Close()
			return nil, fmt.Errorf("applying GHL extension migration: %w", err)
		}
	}
	return s, nil
}

func openGHLStoreReadOnly() (*store.Store, error) {
	dbPath := defaultDBPath("gohighlevel-pp-cli")
	s, err := store.OpenReadOnly(dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local cache read-only at %s: %w", dbPath, err)
	}
	return s, nil
}

// nullStr returns the underlying string or "" for NULL.
func nullStr(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// nullInt returns the underlying int64 or 0 for NULL.
func nullInt(ni sql.NullInt64) int64 {
	if ni.Valid {
		return ni.Int64
	}
	return 0
}

// nullFloat returns the underlying float64 or 0 for NULL.
func nullFloat(nf sql.NullFloat64) float64 {
	if nf.Valid {
		return nf.Float64
	}
	return 0
}
