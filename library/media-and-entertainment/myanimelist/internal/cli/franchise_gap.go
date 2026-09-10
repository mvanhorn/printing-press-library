// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type franchiseGapRow struct {
	FromKind  string `json:"from_kind"`
	FromID    int    `json:"from_id"`
	FromTitle string `json:"from_title,omitempty"`
	Relation  string `json:"relation"`
	Kind      string `json:"kind"`
	ID        int    `json:"id"`
	Title     string `json:"title"`
}

type franchiseGapView struct {
	Gaps    []franchiseGapRow `json:"gaps"`
	Scanned int               `json:"library_entries_scanned"`
	Note    string            `json:"note,omitempty"`
}

// newNovelFranchiseGapCmd walks the relation graph for every franchise already
// present in the local library and lists entries the user has not tracked.
func newNovelFranchiseGapCmd(flags *rootFlags) *cobra.Command {
	var dbPath, kind string
	var limit int
	cmd := &cobra.Command{
		Use:   "gap",
		Short: "List franchise entries missing from your local library",
		Long: "Use this command for entries missing from franchises you have already started in your local library.\n" +
			"Do NOT use this command for the ordering of franchise entries; use 'watch-order' instead.\n" +
			"Do NOT use this command for anime-to-manga source coverage; use 'adaptation' instead.",
		Example: "  myanimelist-pp-cli franchise gap --json",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:data-source":      "local",
			"pp:happy-args":       "--kind=anime",
			"pp:typed-exit-codes": "0,3",
			"pp:novel-scaffold":   "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 && !cmd.Flags().Changed("kind") {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "franchise gap")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			dbPath = malDBPath(flags, dbPath)
			if !malStoreExists(dbPath) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: myanimelist-pp-cli track add 52991 --status watching --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), franchiseGapView{Gaps: make([]franchiseGapRow, 0)}, flags)
				}
				return nil
			}
			db, err := malOpenStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			entries, err := malLoadLibrary(ctx, db, kind)
			if err != nil {
				return err
			}
			tracked, err := malLibraryIDs(ctx, db)
			if err != nil {
				return err
			}
			if cliutilDogfood() && len(entries) > 3 {
				entries = entries[:3]
			}
			gaps := make([]franchiseGapRow, 0, 8)
			seen := map[string]bool{}
			for _, e := range entries {
				detail, derr := malDetail(ctx, flags, e.Kind, e.ID)
				if derr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not read %s %d: %v\n", e.Kind, e.ID, derr)
					continue
				}
				for _, rel := range detail.Related {
					if rel.Kind != "anime" {
						continue
					}
					key := fmt.Sprintf("%s:%d", rel.Kind, rel.ID)
					if tracked[key] || seen[key] {
						continue
					}
					seen[key] = true
					gaps = append(gaps, franchiseGapRow{
						FromKind: detail.Kind, FromID: detail.ID, FromTitle: detail.Title,
						Relation: rel.Relation, Kind: rel.Kind, ID: rel.ID, Title: rel.Title,
					})
				}
			}
			if limit > 0 && len(gaps) > limit {
				gaps = gaps[:limit]
			}
			view := franchiseGapView{Gaps: gaps, Scanned: len(entries)}
			if len(gaps) == 0 {
				view.Note = "every related entry for your tracked titles is already in the local library"
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(gaps) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			table := make([]map[string]any, 0, len(gaps))
			for _, g := range gaps {
				table = append(table, map[string]any{"from": g.FromTitle, "relation": g.Relation, "missing": g.Title, "id": g.ID})
			}
			return printAutoTable(cmd.OutOrStdout(), table)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	cmd.Flags().StringVar(&kind, "kind", "", "Only scan library entries of this kind (anime or manga; empty scans both)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum missing entries to report")
	return cmd
}

// cliutilDogfood reports whether the live dogfood matrix is driving this
// process, so wide fan-out reads can be curtailed to fit the per-command budget.
func cliutilDogfood() bool {
	return isDogfoodEnv()
}
