// Copyright 2026 paul-bockewitz. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/education/ankiweb/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/education/ankiweb/internal/store"
	"github.com/spf13/cobra"
)

// defaultSharedSearchTerms is the category set synced when `sync` is run with
// no --search and no --resources. Chosen to cover the most common AnkiWeb
// catalogs so offline search has useful coverage out of the box.
func defaultSharedSearchTerms() []string {
	return []string{"spanish", "japanese", "french", "german", "anatomy", "chemistry"}
}

// runSharedSync fetches the shared-deck catalog for each term and upserts the
// decks into the typed `shared` table (and the per-term watch snapshot) so the
// offline `search` command can full-text them. Emits NDJSON sync events on
// stdout, mirroring the generic sync command's contract.
func runSharedSync(cmd *cobra.Command, flags *rootFlags, dbPath string, terms []string) error {
	w := cmd.OutOrStdout()

	if dbPath == "" {
		dbPath = defaultDBPath("ankiweb-pp-cli")
	}
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return fmt.Errorf("opening local database: %w", err)
	}
	defer db.Close()

	if cliutil.IsVerifyEnv() {
		fmt.Fprintf(w, `{"event":"sync_summary","total_records":0,"resources":%d}`+"\n", len(terms))
		return nil
	}

	c, _, err := flags.newSvcClient()
	if err != nil {
		return err
	}

	total := 0
	for _, term := range terms {
		fmt.Fprintf(w, `{"event":"sync_start","resource":"shared","search":%q}`+"\n", term)
		decks, err := listDecks(cmd.Context(), c, term)
		if err != nil {
			fmt.Fprintf(w, `{"event":"sync_error","resource":"shared","search":%q,"error":%q}`+"\n", term, err.Error())
			continue
		}
		stored := 0
		snapshot := watchResourceType(term)
		for _, d := range decks {
			raw, _ := json.Marshal(d)
			if err := db.UpsertShared(raw); err != nil {
				continue
			}
			_ = db.Upsert(snapshot, d.ID, raw)
			stored++
		}
		total += stored
		fmt.Fprintf(w, `{"event":"sync_complete","resource":"shared","search":%q,"total":%d}`+"\n", term, stored)
	}

	// Persist the cumulative total once, after all terms. A per-term call would
	// overwrite the single PRIMARY KEY(resource_type) row in sync_state, leaving
	// total_count reflecting only the last term instead of the sum.
	_ = db.SaveSyncState("shared", "", total)

	fmt.Fprintf(w, `{"event":"sync_summary","total_records":%d,"resources":%d}`+"\n", total, len(terms))
	return nil
}
