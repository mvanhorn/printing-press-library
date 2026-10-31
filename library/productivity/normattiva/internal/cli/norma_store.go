// Copyright 2026 Luigi Pastore and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/productivity/normattiva/internal/store"
)

// ensureNormaTables creates the watchlist tables used by segui/novita.
// Followed acts live in norma_segui; norma_snapshot keeps the last seen
// vigenza window and text hash of each followed article.
func ensureNormaTables(ctx context.Context, db *store.Store) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS norma_segui (
			urn TEXT PRIMARY KEY,
			etichetta TEXT NOT NULL DEFAULT '',
			articoli TEXT NOT NULL DEFAULT '',
			aggiunto_il TEXT NOT NULL,
			ultimo_controllo TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS norma_snapshot (
			urn TEXT PRIMARY KEY,
			atto_urn TEXT NOT NULL,
			hash TEXT NOT NULL,
			inizio_vigenza TEXT NOT NULL DEFAULT '',
			fine_vigenza TEXT NOT NULL DEFAULT '',
			testo TEXT NOT NULL DEFAULT '',
			visto_il TEXT NOT NULL
		)`,
	}
	for _, q := range stmts {
		if _, err := db.DB().ExecContext(ctx, q); err != nil {
			return fmt.Errorf("creating watchlist tables: %w", err)
		}
	}
	return nil
}
