// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/algolia/internal/store"
	"github.com/spf13/cobra"
)

type objectsDiffResult struct {
	Left    string   `json:"left"`
	Right   string   `json:"right"`
	Added   []string `json:"added"`
	Removed []string `json:"removed"`
	Changed []string `json:"changed"`
	Counts  struct {
		Added   int `json:"added"`
		Removed int `json:"removed"`
		Changed int `json:"changed"`
	} `json:"counts"`
}

func newNovelObjectsDiffCmd(flags *rootFlags) *cobra.Command {
	var flagDB string
	var flagLimit int

	cmd := &cobra.Command{
		Use:         "diff <index-a> <index-b>",
		Short:       "Compare records of two indices (added/removed/changed by objectID) to verify prod/staging parity.",
		Example:     "  algolia-pp-cli objects diff algolia_movie_sample_dataset staging_movies",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "index-a=algolia_movie_sample_dataset;index-b=algolia_movie_sample_dataset", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "objects diff")
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("two index names are required"))
			}
			leftName, rightName := args[0], args[1]
			if flagLimit <= 0 {
				flagLimit = 100
			}
			if flagDB == "" {
				flagDB = defaultDBPath("algolia-pp-cli")
			}
			if _, statErr := os.Stat(flagDB); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: algolia-pp-cli sync --resources indexes,browse to populate the local database.\n", flagDB)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), objectsDiffResult{Left: leftName, Right: rightName}, flags)
				}
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), flagDB)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "browse") {
				hintIfStale(cmd, db, "browse", flags.maxAge)
			}

			loadIndexRecords := func(indexName string) (map[string]json.RawMessage, error) {
				out := make(map[string]json.RawMessage)
				rows, err := db.DB().QueryContext(cmd.Context(), `
					SELECT id, data FROM browse
					WHERE indexes_id = ?`, indexName)
				if err != nil {
					return nil, err
				}
				for rows.Next() {
					var id string
					var d string
					if err := rows.Scan(&id, &d); err != nil {
						_ = rows.Close()
						return nil, err
					}
					// browse is a per-index dependent resource: the store
					// composites id as "<objectID>\x00<indexName>" so the
					// same objectID in two different indices doesn't
					// collide on the same primary key. Strip that suffix so
					// records with the same objectID in both indices are
					// recognized as the same record, not counted as both
					// added and removed.
					out[store.BareResourceID(id)] = stripSyncMetadataFields(json.RawMessage(d))
				}
				if err := rows.Err(); err != nil {
					_ = rows.Close()
					return nil, err
				}
				if err := rows.Close(); err != nil {
					return nil, err
				}
				return out, nil
			}

			left, err := loadIndexRecords(leftName)
			if err != nil {
				return fmt.Errorf("loading records for %q: %w", leftName, err)
			}
			right, err := loadIndexRecords(rightName)
			if err != nil {
				return fmt.Errorf("loading records for %q: %w", rightName, err)
			}

			res := objectsDiffResult{Left: leftName, Right: rightName}
			res.Added = make([]string, 0)
			res.Removed = make([]string, 0)
			res.Changed = make([]string, 0)
			for id := range right {
				if _, ok := left[id]; !ok {
					res.Added = append(res.Added, id)
				}
			}
			for id := range left {
				r, ok := right[id]
				if !ok {
					res.Removed = append(res.Removed, id)
					continue
				}
				if string(left[id]) != string(r) {
					res.Changed = append(res.Changed, id)
				}
			}
			res.Counts.Added = len(res.Added)
			res.Counts.Removed = len(res.Removed)
			res.Counts.Changed = len(res.Changed)
			if len(res.Added) > flagLimit {
				res.Added = res.Added[:flagLimit]
			}
			if len(res.Removed) > flagLimit {
				res.Removed = res.Removed[:flagLimit]
			}
			if len(res.Changed) > flagLimit {
				res.Changed = res.Changed[:flagLimit]
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Records diff %q vs %q: +%d added, -%d removed, ~%d changed\n",
				leftName, rightName, res.Counts.Added, res.Counts.Removed, res.Counts.Changed)
			for _, id := range res.Added {
				fmt.Fprintf(cmd.OutOrStdout(), "  + %s\n", id)
			}
			for _, id := range res.Removed {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", id)
			}
			for _, id := range res.Changed {
				fmt.Fprintf(cmd.OutOrStdout(), "  ~ %s\n", id)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.Flags().IntVar(&flagLimit, "limit", 100, "Maximum IDs to list per category (default 100)")
	return cmd
}

// stripSyncMetadataFields removes fields the sync layer injects into a
// stored browse row that are not part of the underlying record itself
// (indexes_id). Without this, a byte-for-byte comparison of two indices'
// copies of an otherwise-identical record always reports it as changed,
// since indexes_id necessarily differs between the two sides being
// compared. Falls back to the original bytes if the row is not a JSON
// object (should not happen for browse rows, but comparing raw bytes is
// safer than dropping the record from the diff).
func stripSyncMetadataFields(data json.RawMessage) json.RawMessage {
	obj, err := store.DecodeJSONObject(data)
	if err != nil {
		return data
	}
	delete(obj, "indexes_id")
	normalized, err := json.Marshal(obj)
	if err != nil {
		return data
	}
	return normalized
}
