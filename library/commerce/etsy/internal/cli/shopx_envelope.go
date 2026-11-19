// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
	"github.com/spf13/cobra"
)

type shopxEnvelope struct {
	Results any      `json:"results"`
	Notes   []string `json:"notes"`
}

const shopxNoteNoStore = "no local store found; run 'etsy-pp-cli sync' first"

func shopxEmit(cmd *cobra.Command, flags *rootFlags, results any, notes []string) error {
	unique := []string{}
	seen := map[string]bool{}
	for _, note := range notes {
		if !seen[note] {
			unique = append(unique, note)
			seen[note] = true
		}
	}
	shopxPrintNotes(cmd, unique)
	// Selection runs before compaction and ignores its keep-list. Preserve notes
	// explicitly without mutating the shared root flags.
	if flags != nil && flags.selectFields != "" {
		copied := *flags
		copied.selectFields = flags.selectFields + ",notes"
		flags = &copied
	}
	return printJSONFilteredKeep(cmd.OutOrStdout(), shopxEnvelope{Results: results, Notes: unique}, flags, "notes")
}

func shopxFamilyHintNotes(db *store.Store, names ...string) []string {
	notes := []string{}
	if db == nil {
		return notes
	}
	for _, name := range names {
		if syncHintCommand != "" && !shopxFamilySynced(db, shopxFamilyTypes(name)) {
			notes = append(notes, fmt.Sprintf("%s not synced yet. Run '%s' before trusting local results.", name, syncHintCommand))
		}
	}
	return notes
}

// Compatibility for the low-stock command, outside this task's editable scope.
func shopxEmitHints(cmd *cobra.Command, db *store.Store, names ...string) {
	shopxPrintNotes(cmd, shopxFamilyHintNotes(db, names...))
}

func shopxCommandNotes(db *store.Store, names ...string) []string {
	if db == nil {
		return []string{shopxNoteNoStore}
	}
	return shopxFamilyHintNotes(db, names...)
}
