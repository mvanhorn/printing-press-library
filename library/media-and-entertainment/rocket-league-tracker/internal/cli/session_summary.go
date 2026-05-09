// Copyright 2026 addisonk. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

// newSessionSummaryCmd implements `session-summary <player>` — closes
// today's play session: per-playlist start vs current MMR, total games
// delta, most-played playlist (highest games delta).
func newSessionSummaryCmd(flags *rootFlags) *cobra.Command {
	var since string
	var platform string
	cmd := &cobra.Command{
		Use:         "session-summary <player>",
		Short:       "Per-playlist start-vs-now MMR and most-played playlist for today's session.",
		Long:        "Reads local rank snapshots since --since (default: today's local 00:00) and reports start-vs-current MMR per playlist plus the total games delta. Most-played playlist is the one with the largest games-played delta within the window.",
		Example:     "  rocket-league-tracker-pp-cli session-summary 550e8400-e29b-41d4-a716-446655440000",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			player := args[0]

			var sinceT time.Time
			if since == "" {
				now := time.Now()
				sinceT = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			} else {
				t, err := time.Parse(time.RFC3339, since)
				if err != nil {
					t2, err2 := time.Parse("2006-01-02", since)
					if err2 != nil {
						return usageErr(fmt.Errorf("--since must be RFC3339 or YYYY-MM-DD"))
					}
					t = t2
				}
				sinceT = t
			}

			db, err := openStoreForRead(cmd.Context(), "rocket-league-tracker-pp-cli")
			if err != nil {
				return err
			}
			if db == nil {
				return notFoundErr(fmt.Errorf("no local data. Run 'rocket-league-tracker-pp-cli sync' first"))
			}
			defer db.Close()

			rows, err := queryRankRowsSince(cmd.Context(), db, player, sinceT)
			if err != nil {
				return err
			}
			snaps := flattenRankRows(rows)
			if len(snaps) == 0 {
				return notFoundErr(fmt.Errorf("no snapshots since %s for %s", sinceT.Format(time.RFC3339), player))
			}

			// Group by playlist; keep earliest and latest within the window.
			type pair struct {
				start, latest rankSnapshot
			}
			by := map[string]*pair{}
			for _, s := range snaps {
				p, ok := by[s.Playlist]
				if !ok {
					p = &pair{start: s, latest: s}
					by[s.Playlist] = p
					continue
				}
				if s.CapturedAt.Before(p.start.CapturedAt) {
					p.start = s
				}
				if s.CapturedAt.After(p.latest.CapturedAt) {
					p.latest = s
				}
			}

			type rec struct {
				Playlist    string `json:"playlist"`
				StartMMR    int    `json:"start_mmr"`
				CurrentMMR  int    `json:"current_mmr"`
				MMRDelta    int    `json:"mmr_delta"`
				GamesDelta  int    `json:"games_delta"`
				CurrentTier string `json:"current_tier"`
				CurrentDiv  string `json:"current_division"`
			}
			out := make([]rec, 0, len(by))
			totalGames := 0
			mostPlayed := ""
			mostPlayedGames := -1
			for k, v := range by {
				gd := v.latest.Games - v.start.Games
				if gd < 0 {
					gd = 0
				}
				out = append(out, rec{
					Playlist:    k,
					StartMMR:    v.start.MMR,
					CurrentMMR:  v.latest.MMR,
					MMRDelta:    v.latest.MMR - v.start.MMR,
					GamesDelta:  gd,
					CurrentTier: v.latest.Tier,
					CurrentDiv:  v.latest.Division,
				})
				totalGames += gd
				if gd > mostPlayedGames {
					mostPlayedGames = gd
					mostPlayed = k
				}
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Playlist < out[j].Playlist })

			summary := map[string]any{
				"player":               player,
				"since":                sinceT.UTC().Format(time.RFC3339),
				"playlists":            out,
				"total_games_delta":    totalGames,
				"most_played_playlist": mostPlayed,
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Session since %s — %d games, most-played: %s\n", sinceT.Format("2006-01-02 15:04"), totalGames, mostPlayed)
			for _, r := range out {
				delta := ""
				switch {
				case r.MMRDelta > 0:
					delta = fmt.Sprintf("+%d", r.MMRDelta)
				default:
					delta = fmt.Sprintf("%d", r.MMRDelta)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s: %d → %d (%s) over %d games\n", r.Playlist, r.StartMMR, r.CurrentMMR, delta, r.GamesDelta)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Override start of session (RFC3339 or YYYY-MM-DD). Defaults to today's local 00:00.")
	cmd.Flags().StringVar(&platform, "platform", "epic", "Platform: epic, steam, psn, xbox")
	_ = platform
	return cmd
}
