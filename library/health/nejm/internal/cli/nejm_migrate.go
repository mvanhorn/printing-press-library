// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-coded: additive, idempotent migration for the NEJM-specific `feed` column.

package cli

import (
	"strings"

	"github.com/mvanhorn/printing-press-library/library/health/nejm/internal/store"
)

// nejmEnsureFeedColumn adds the `feed` TEXT column to the article table if it
// is missing, then backfills existing rows from the data JSON blob (which has
// always carried the feed name), falling back to 'etoc' for any legacy row that
// predates feed tracking.
//
// This runs on every store open (see nejmOpenStore) and at the start of a sync,
// so any command — including `current` — sees the column whether or not the user
// has re-synced since upgrading. It is deliberately kept out of the generated
// store.go (DO NOT EDIT) and does NOT touch the generated schema_version; the
// migration is additive and safe to run repeatedly.
func nejmEnsureFeedColumn(db *store.Store) error {
	if db == nil {
		return nil
	}

	var cnt int
	if err := db.DB().QueryRow(
		`SELECT COUNT(*) FROM pragma_table_info('article') WHERE name = 'feed'`,
	).Scan(&cnt); err != nil {
		// Article table may not exist yet on a brand-new DB; nothing to migrate.
		if nejmIsBenignMigrationErr(err) {
			return nil
		}
		return err
	}

	if cnt == 0 {
		if _, err := db.DB().Exec(`ALTER TABLE "article" ADD COLUMN "feed" TEXT`); err != nil {
			// Tolerate a concurrent migration that already added the column.
			if !nejmIsBenignMigrationErr(err) {
				return err
			}
		}
	}

	// Backfill: prefer the per-row feed recorded in the data JSON; only legacy
	// rows with no feed in JSON fall back to the 'etoc' default.
	if _, err := db.DB().Exec(
		`UPDATE "article"
		   SET "feed" = COALESCE(json_extract("data", '$.feed'), 'etoc')
		 WHERE "feed" IS NULL OR "feed" = ''`,
	); err != nil {
		if !nejmIsBenignMigrationErr(err) {
			return err
		}
	}

	return nil
}

// nejmIsBenignMigrationErr reports whether a migration error is safe to ignore
// (table not yet created, or column already present from a prior/concurrent run).
func nejmIsBenignMigrationErr(err error) bool {
	if err == nil {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate column") ||
		strings.Contains(msg, "no such table") ||
		strings.Contains(msg, "already exists")
}
