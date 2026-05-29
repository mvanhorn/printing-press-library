// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"fmt"
)

// migrateExtras runs after the generated store migrations and before the
// schema-version stamp. It is the canonical place for novel-feature auxiliary
// tables that need to live in the local store.
//
// Edit this file when adding tables for novel commands. Keep migrations
// idempotent with CREATE TABLE IF NOT EXISTS / CREATE INDEX IF NOT EXISTS so
// every store open can safely re-run them.
func (s *Store) migrateExtras(ctx context.Context, conn *sql.Conn) error {
	migrations := []string{
		// hubspot_pending_digests backs the dry-run -> digest -> confirm
		// gating used by `contacts bulk-update` (and any future >100-row
		// mutation command). A row is written by the dry-run path and
		// looked up by the confirm path; rows expire after TTL and are
		// lazily GC'd by PurgeExpiredDigests. Keyed on the operator-facing
		// digest itself ("blast-<hex>") so two concurrent dry-runs with
		// identical plans collapse to a single row instead of duplicating.
		`CREATE TABLE IF NOT EXISTS "hubspot_pending_digests" (
			"digest" TEXT PRIMARY KEY,
			"command" TEXT NOT NULL,
			"plan_json" TEXT NOT NULL,
			"row_count" INTEGER NOT NULL,
			"created_at" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			"expires_at" DATETIME NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS "idx_hubspot_pending_digests_expires_at" ON "hubspot_pending_digests"("expires_at")`,
	}
	for _, m := range migrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
	}
	return nil
}
