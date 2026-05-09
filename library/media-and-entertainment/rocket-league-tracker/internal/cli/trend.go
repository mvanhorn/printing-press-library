// Copyright 2026 addisonk. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newTrendCmd implements `trend <player> --playlist <key> --days <N>` —
// per-day MMR series for one playlist over the last N days. Reads only
// from the local rank store, so this command requires that the user has
// been writing snapshots (via `sync` or `import-collector-snapshot`).
func newTrendCmd(flags *rootFlags) *cobra.Command {
	var playlist string
	var days int
	var platform string
	cmd := &cobra.Command{
		Use:         "trend <player>",
		Short:       "Daily MMR series for one playlist over the last N days.",
		Long:        "Reads local rank snapshots for the given player and playlist, groups by UTC day, and returns the latest MMR observed for each day. Use --days to set the window (default 30). Use --playlist to pick a competitive mode.",
		Example:     "  rocket-league-tracker-pp-cli trend 550e8400-e29b-41d4-a716-446655440000 --playlist 2v2 --days 14",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if playlist == "" {
				return usageErr(fmt.Errorf("--playlist is required"))
			}
			if err := validatePlaylist(playlist); err != nil {
				return usageErr(err)
			}
			if days <= 0 {
				days = 30
			}
			player := args[0]
			since := time.Now().UTC().AddDate(0, 0, -days)

			db, err := openStoreForRead(cmd.Context(), "rocket-league-tracker-pp-cli")
			if err != nil {
				return err
			}
			if db == nil {
				return notFoundErr(fmt.Errorf("no local data. Run 'rocket-league-tracker-pp-cli sync' first"))
			}
			defer db.Close()

			rows, err := queryRankRowsSince(cmd.Context(), db, player, since)
			if err != nil {
				return err
			}
			snaps := flattenRankRows(rows)

			// Group by UTC day, keeping the latest snapshot of the day.
			type entry struct {
				Date string `json:"date"`
				MMR  int    `json:"mmr"`
				Tier string `json:"tier"`
			}
			byDay := map[string]rankSnapshot{}
			latestStamp := map[string]time.Time{}
			for _, s := range snaps {
				if !strings.EqualFold(s.Playlist, playlist) {
					continue
				}
				day := s.CapturedAt.UTC().Format("2006-01-02")
				if prev, ok := latestStamp[day]; !ok || s.CapturedAt.After(prev) {
					byDay[day] = s
					latestStamp[day] = s.CapturedAt
				}
			}
			out := make([]entry, 0, len(byDay))
			for d, s := range byDay {
				out = append(out, entry{Date: d, MMR: s.MMR, Tier: s.Tier})
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no %s snapshots in the last %d days for %s\n", playlist, days, player)
				return nil
			}
			for _, e := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%d\t%s\n", e.Date, e.MMR, e.Tier)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&playlist, "playlist", "", "Playlist: 1v1, 2v2, 3v3, hoops, rumble, dropshot, snowday, tournament")
	cmd.Flags().IntVar(&days, "days", 30, "Look-back window in days")
	cmd.Flags().StringVar(&platform, "platform", "epic", "Platform: epic, steam, psn, xbox")
	_ = platform
	return cmd
}
