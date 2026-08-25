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

type gapRecord struct {
	ObjectID     string   `json:"objectID"`
	MissingAttrs []string `json:"missing_attrs"`
}

type objectsGapsResult struct {
	Index string      `json:"index"`
	Gaps  []gapRecord `json:"gaps"`
	Count int         `json:"count"`
}

func newNovelObjectsGapsCmd(flags *rootFlags) *cobra.Command {
	var flagIndex string
	var flagDB string
	var flagLimit int

	cmd := &cobra.Command{
		Use:         "gaps",
		Short:       "List records missing attributes required by the index's searchable settings — records search can never return.",
		Example:     "  algolia-pp-cli objects gaps --index algolia_movie_sample_dataset",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "objects gaps")
			}
			if flagIndex == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--index is required"))
			}
			if flagLimit <= 0 {
				flagLimit = 50
			}
			if flagDB == "" {
				flagDB = defaultDBPath("algolia-pp-cli")
			}
			if _, statErr := os.Stat(flagDB); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: algolia-pp-cli sync --resources indexes,browse to populate the local database.\n", flagDB)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), objectsGapsResult{Index: flagIndex, Gaps: make([]gapRecord, 0)}, flags)
				}
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), flagDB)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "indexes") {
				hintIfStale(cmd, db, "indexes", flags.maxAge)
			}
			if !hintIfUnsynced(cmd, db, "browse") {
				hintIfStale(cmd, db, "browse", flags.maxAge)
			}

			settingsRaw, _ := db.Get("indexes", flagIndex)
			settings := unwrapSettingsObject(settingsRaw)
			searchable := stringSliceField(settings, "searchableAttributes")
			if len(searchable) == 0 {
				searchable = []string{"title", "name", "description"}
			}

			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT id, data FROM browse
				WHERE indexes_id = ?
				LIMIT ?`, flagIndex, flagLimit*20)
			if err != nil {
				return fmt.Errorf("querying records: %w", err)
			}
			var records []struct {
				ID   string
				Data json.RawMessage
			}
			for rows.Next() {
				var id string
				var d string
				if err := rows.Scan(&id, &d); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scan record: %w", err)
				}
				records = append(records, struct {
					ID   string
					Data json.RawMessage
				}{ID: id, Data: json.RawMessage(d)})
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterate records: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("close records: %w", err)
			}

			gaps := make([]gapRecord, 0)
			for _, rec := range records {
				var obj map[string]any
				if json.Unmarshal(rec.Data, &obj) != nil {
					continue
				}
				objectID, _ := obj["objectID"].(string)
				if objectID == "" {
					objectID = rec.ID
				}
				var missing []string
				for _, attr := range searchable {
					val, present := obj[attr]
					if !present || isNilOrEmptyValue(val) {
						missing = append(missing, attr)
					}
				}
				if len(missing) > 0 {
					gaps = append(gaps, gapRecord{ObjectID: objectID, MissingAttrs: missing})
					if len(gaps) >= flagLimit {
						break
					}
				}
			}

			res := objectsGapsResult{Index: flagIndex, Gaps: gaps, Count: len(gaps)}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			if len(gaps) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No gap records found for index %q.\n", flagIndex)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Found %d record(s) with missing searchable attributes in %q:\n", len(gaps), flagIndex)
			for _, g := range gaps {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s — missing %v\n", g.ObjectID, g.MissingAttrs)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagIndex, "index", "", "Index name whose records to audit")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum gap records to report (default 50)")
	return cmd
}

func isNilOrEmptyValue(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return t == ""
	case []any:
		return len(t) == 0
	case map[string]any:
		return len(t) == 0
	case bool:
		return false
	case float64:
		return false
	}
	return false
}
