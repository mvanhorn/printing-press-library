// Copyright 2026 addisonk. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newPopulationBestTimeCmd implements `population-best-time --playlist <k>
// [--days 7]` — aggregates population snapshots from the local store by
// hour-of-week and returns the top 5 hours by player count for the playlist.
func newPopulationBestTimeCmd(flags *rootFlags) *cobra.Command {
	var playlist string
	var days int
	cmd := &cobra.Command{
		Use:         "population-best-time",
		Short:       "Hour-of-week with largest active queue for a playlist (top 5).",
		Long:        "Reads all population snapshots from the local store within the last --days, groups by (weekday, hour) UTC, and returns the top 5 buckets by average player count for the chosen --playlist.",
		Example:     "  rocket-league-tracker-pp-cli population-best-time --playlist 2v2 --days 7",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
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
				days = 7
			}
			db, err := openStoreForRead(cmd.Context(), "rocket-league-tracker-pp-cli")
			if err != nil {
				return err
			}
			if db == nil {
				return notFoundErr(fmt.Errorf("no local data. Run 'rocket-league-tracker-pp-cli sync population' first"))
			}
			defer db.Close()

			since := time.Now().UTC().AddDate(0, 0, -days)
			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT data, COALESCE(synced_at, '') FROM resources WHERE resource_type = 'population' AND synced_at >= ? ORDER BY synced_at`,
				since.Format(time.RFC3339))
			if err != nil {
				return fmt.Errorf("querying population: %w", err)
			}
			defer rows.Close()

			type bucket struct {
				weekday time.Weekday
				hour    int
			}
			sums := map[bucket]int64{}
			counts := map[bucket]int{}
			playlistLower := strings.ToLower(playlist)

			for rows.Next() {
				var data, ts string
				if err := rows.Scan(&data, &ts); err != nil {
					return err
				}
				snapTime := parseAnyTime(ts)
				if snapTime.IsZero() {
					continue
				}
				count := extractPlaylistPopulation([]byte(data), playlistLower)
				if count < 0 {
					continue
				}
				b := bucket{weekday: snapTime.UTC().Weekday(), hour: snapTime.UTC().Hour()}
				sums[b] += int64(count)
				counts[b]++
			}
			if err := rows.Err(); err != nil {
				return err
			}

			type entry struct {
				Weekday  string  `json:"weekday"`
				Hour     int     `json:"hour"`
				AvgCount float64 `json:"avg_player_count"`
				Samples  int     `json:"samples"`
			}
			out := []entry{}
			for b, sum := range sums {
				n := counts[b]
				if n == 0 {
					continue
				}
				out = append(out, entry{
					Weekday:  b.weekday.String(),
					Hour:     b.hour,
					AvgCount: float64(sum) / float64(n),
					Samples:  n,
				})
			}
			sort.Slice(out, func(i, j int) bool { return out[i].AvgCount > out[j].AvgCount })
			if len(out) > 5 {
				out = out[:5]
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no population snapshots in the last %d days\n", days)
				return nil
			}
			for _, e := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%s %02d:00\t%.0f players\t(%d samples)\n", e.Weekday, e.Hour, e.AvgCount, e.Samples)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&playlist, "playlist", "", "Playlist: 1v1, 2v2, 3v3, hoops, rumble, dropshot, snowday, tournament")
	cmd.Flags().IntVar(&days, "days", 7, "Look-back window in days")
	return cmd
}

// extractPlaylistPopulation pulls the player count for a specific playlist
// from a single population-snapshot blob. Tries common shapes: an array of
// {playlist, players} entries, an object keyed by playlist, or a flat object
// with a numeric field matching the playlist key.
func extractPlaylistPopulation(data []byte, playlistLower string) int {
	if len(data) == 0 {
		return -1
	}
	var arr []map[string]any
	if err := json.Unmarshal(data, &arr); err == nil {
		for _, item := range arr {
			name := strings.ToLower(firstStr(item, "playlist", "name", "key", "id"))
			if name == playlistLower {
				return firstInt(item, "players", "player_count", "playerCount", "count", "population")
			}
		}
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err == nil {
		for k, v := range obj {
			if strings.ToLower(k) == playlistLower {
				var n int
				if json.Unmarshal(v, &n) == nil {
					return n
				}
				var inner map[string]any
				if json.Unmarshal(v, &inner) == nil {
					return firstInt(inner, "players", "player_count", "playerCount", "count", "population")
				}
			}
		}
		// Nested envelope (population.<playlist>)
		for _, key := range []string{"playlists", "regions", "data"} {
			if raw, ok := obj[key]; ok {
				if v := extractPlaylistPopulation(raw, playlistLower); v >= 0 {
					return v
				}
			}
		}
	}
	return -1
}
