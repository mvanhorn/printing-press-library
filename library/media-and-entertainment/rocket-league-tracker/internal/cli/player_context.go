// Copyright 2026 addisonk. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newPlayerContextCmd implements `player-context <player>` — produces a
// single agent-shaped JSON envelope: identity + per-playlist MMR series
// over the last N days + current rank/tier. Honors --select for dotted-path
// field selection so token usage can be controlled.
//
// Renamed from `agent-context` to avoid colliding with the framework's
// existing `agent-context` command (which describes the CLI itself).
func newPlayerContextCmd(flags *rootFlags) *cobra.Command {
	var days int
	var platform string
	cmd := &cobra.Command{
		Use:         "player-context <player>",
		Short:       "Single agent-shaped JSON envelope: identity + per-playlist MMR series + current rank.",
		Long:        "Calls /profile/{playerId} for identity, reads the last --days days of local rank snapshots, and joins them into a JSON envelope keyed by playlist. Use --select to keep only the fields the agent needs.",
		Example:     "  rocket-league-tracker-pp-cli player-context 550e8400-e29b-41d4-a716-446655440000 --days 30 --select identity,playlists.current",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if days <= 0 {
				days = 30
			}
			player := args[0]

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			profileRaw, err := c.Get("/profile/"+pathEscapePlayer(player), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			profile := extractResponseData(profileRaw)
			var identity any
			_ = json.Unmarshal(profile, &identity)

			db, _ := openStoreForRead(cmd.Context(), "rocket-league-tracker-pp-cli")
			var allSnaps []rankSnapshot
			if db != nil {
				defer db.Close()
				since := time.Now().UTC().AddDate(0, 0, -days)
				rows, qerr := queryRankRowsSince(cmd.Context(), db, player, since)
				if qerr == nil {
					allSnaps = flattenRankRows(rows)
				}
			}

			// Group snapshots by playlist; per-day latest reading.
			type dayPoint struct {
				Date string `json:"date"`
				MMR  int    `json:"mmr"`
			}
			type playlistContext struct {
				Key       string         `json:"key"`
				MMRSeries []dayPoint     `json:"mmr_series"`
				Current   map[string]any `json:"current"`
			}
			byKey := map[string]*playlistContext{}
			latestByDay := map[string]map[string]rankSnapshot{}
			latestOverall := map[string]rankSnapshot{}
			for _, s := range allSnaps {
				k := strings.ToLower(s.Playlist)
				if _, ok := byKey[k]; !ok {
					byKey[k] = &playlistContext{Key: k}
					latestByDay[k] = map[string]rankSnapshot{}
				}
				day := s.CapturedAt.UTC().Format("2006-01-02")
				prev, exists := latestByDay[k][day]
				if !exists || s.CapturedAt.After(prev.CapturedAt) {
					latestByDay[k][day] = s
				}
				cur, hadCur := latestOverall[k]
				if !hadCur || s.CapturedAt.After(cur.CapturedAt) {
					latestOverall[k] = s
				}
			}
			// Build sorted series.
			for k, pc := range byKey {
				days := make([]string, 0, len(latestByDay[k]))
				for d := range latestByDay[k] {
					days = append(days, d)
				}
				sort.Strings(days)
				for _, d := range days {
					pc.MMRSeries = append(pc.MMRSeries, dayPoint{Date: d, MMR: latestByDay[k][d].MMR})
				}
				if cur, ok := latestOverall[k]; ok {
					pc.Current = map[string]any{
						"tier":     cur.Tier,
						"division": cur.Division,
						"mmr":      cur.MMR,
					}
				}
			}
			// Stable order.
			out := make([]playlistContext, 0, len(byKey))
			for _, pc := range byKey {
				out = append(out, *pc)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })

			envelope := map[string]any{
				"identity":     identity,
				"playlists":    out,
				"generated_at": time.Now().UTC().Format(time.RFC3339),
			}
			return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "Look-back window in days for the per-playlist MMR series")
	cmd.Flags().StringVar(&platform, "platform", "epic", "Platform: epic, steam, psn, xbox")
	_ = platform
	return cmd
}
