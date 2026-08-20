// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// execStoreRecipe is the canonical recipe upsert. Keeps ID/slug/tag denormalized
// columns up to date so the local browse and articles-for-recipe queries are
// indexable without scanning the JSON blob.
func execStoreRecipe(db *sql.DB, id, slug, tag string, data json.RawMessage) error {
	if db == nil {
		return fmt.Errorf("store handle is nil")
	}
	if id == "" {
		// Fall back to slug if the SSR didn't include _id (rare but defensive).
		id = slug
	}
	_, err := db.Exec(
		`INSERT INTO recipes (id, data, slug, tag) VALUES (?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET data=excluded.data, slug=excluded.slug, tag=excluded.tag, synced_at=CURRENT_TIMESTAMP`,
		id, []byte(data), slug, tag,
	)
	return err
}

// execStoreArticle is the canonical article upsert.
func execStoreArticle(db *sql.DB, id, slug, vertical string, data json.RawMessage) error {
	if db == nil {
		return fmt.Errorf("store handle is nil")
	}
	if id == "" {
		id = slug
	}
	_, err := db.Exec(
		`INSERT INTO articles (id, data, slug, vertical) VALUES (?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET data=excluded.data, slug=excluded.slug, vertical=excluded.vertical, synced_at=CURRENT_TIMESTAMP`,
		id, []byte(data), slug, vertical,
	)
	return err
}

// pantrySchema creates the local pantry table on first use. Idempotent.
const pantrySchema = `CREATE TABLE IF NOT EXISTS pantry (
	ingredient TEXT PRIMARY KEY,
	added_at DATETIME DEFAULT CURRENT_TIMESTAMP
)`

func ensurePantryTable(db *sql.DB) error {
	_, err := db.Exec(pantrySchema)
	return err
}
