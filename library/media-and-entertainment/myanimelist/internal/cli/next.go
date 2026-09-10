// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type nextRow struct {
	Kind      string  `json:"kind"`
	ID        int     `json:"id"`
	Title     string  `json:"title,omitempty"`
	Progress  int     `json:"progress"`
	Total     int     `json:"total,omitempty"`
	Next      int     `json:"next_episode"`
	Remaining int     `json:"remaining,omitempty"`
	Ratio     float64 `json:"progress_ratio,omitempty"`
}

// newNovelNextCmd picks up where the local library left off.
func newNovelNextCmd(flags *rootFlags) *cobra.Command {
	var dbPath, kind string
	var limit int
	cmd := &cobra.Command{
		Use:   "next",
		Short: "What to watch next, from your local library",
		Long: "Use this command to continue a tracked show. It reads only the local library, so it works offline.\n" +
			"Do NOT use this command to browse the catalog; use 'season list' or 'ranking anime' instead.",
		Example: "  myanimelist-pp-cli next --json",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:data-source":      "local",
			"pp:happy-args":       "--kind=anime",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "next")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			dbPath = malDBPath(flags, dbPath)
			if !malStoreExists(dbPath) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: myanimelist-pp-cli track add 52991 --status watching --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]nextRow, 0), flags)
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
			rows := make([]nextRow, 0, len(entries))
			for _, e := range entries {
				if e.Status != "watching" {
					continue
				}
				row := nextRow{Kind: e.Kind, ID: e.ID, Title: e.Title, Progress: e.Progress, Next: e.Progress + 1, Total: e.Total}
				if e.Total > 0 {
					row.Remaining = e.Total - e.Progress
					row.Ratio = float64(e.Progress) / float64(e.Total)
				}
				rows = append(rows, row)
			}
			// Least-finished first: the show you are closest to starting is the
			// one most likely to have stalled.
			for i := 1; i < len(rows); i++ {
				for j := i; j > 0 && rows[j].Ratio < rows[j-1].Ratio; j-- {
					rows[j], rows[j-1] = rows[j-1], rows[j]
				}
			}
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Nothing is marked watching locally. Add one with `myanimelist-pp-cli track add <id> --status watching`.")
				return nil
			}
			table := make([]map[string]any, 0, len(rows))
			for _, r := range rows {
				table = append(table, map[string]any{"title": r.Title, "next_episode": r.Next, "progress": fmt.Sprintf("%d/%d", r.Progress, r.Total)})
			}
			return printAutoTable(cmd.OutOrStdout(), table)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	cmd.Flags().StringVar(&kind, "kind", "", "Only consider this kind (anime or manga)")
	cmd.Flags().IntVar(&limit, "limit", 5, "How many candidates to show")
	return cmd
}
