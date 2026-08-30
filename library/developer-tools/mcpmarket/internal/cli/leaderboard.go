// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source computed
// This command computes results from locally stored history (resource_snapshots)
// built up as the user browses; it does not read a single upstream resource type.

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

type leaderboardEntry struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Likes int    `json:"likes"`
}

func newNovelLeaderboardCmd(flags *rootFlags) *cobra.Command {
	var flagAsOf string
	var flagType string
	var limit int

	cmd := &cobra.Command{
		Use:         "leaderboard",
		Short:       "See what the top-100 leaderboard looked like on a past date, not just right now.",
		Example:     "  mcpmarket-pp-cli leaderboard --as-of 2026-08-01 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "leaderboard")
			}
			resourceType := flagType
			if resourceType == "" {
				resourceType = "server"
			}
			wantsNow := flagAsOf == ""
			asOf := flagAsOf
			if asOf == "" {
				asOf = time.Now().UTC().Format("2006-01-02")
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath := defaultDBPath("mcpmarket-pp-cli")
			db, err := storeOpenForNovel(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			if wantsNow {
				if _, _, err := db.CaptureSnapshot(ctx, resourceType); err != nil {
					return fmt.Errorf("capturing today's snapshot: %w", err)
				}
			}

			date, ok, err := db.NearestSnapshotDateOnOrBefore(ctx, asOf)
			if err != nil {
				return err
			}
			if !ok {
				note := fmt.Sprintf("no snapshot exists on or before %s yet. Run `mcpmarket-pp-cli %s leaderboard` (which snapshots today's state) at least once, then retry with an earlier --as-of on a later day.", asOf, resourceType)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"note": note}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), note)
				return nil
			}

			rows, err := db.SnapshotRows(ctx, date, resourceType)
			if err != nil {
				return err
			}
			entries := make([]leaderboardEntry, 0, len(rows))
			for _, r := range rows {
				entries = append(entries, leaderboardEntry{
					ID:    r.ResourceID,
					Name:  entityName(r.Data),
					Likes: entityLikeCount(r.Data),
				})
			}
			sort.Slice(entries, func(i, j int) bool { return entries[i].Likes > entries[j].Likes })
			if limit > 0 && len(entries) > limit {
				entries = entries[:limit]
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"as_of": date, "requested": asOf, "entries": entries}, flags)
		},
	}
	cmd.Flags().StringVar(&flagAsOf, "as-of", "", "date to look back to (YYYY-MM-DD); defaults to today")
	cmd.Flags().StringVar(&flagType, "type", "server", "resource type: server or skill")
	cmd.Flags().IntVar(&limit, "limit", 100, "maximum entries to return")
	return cmd
}
