// Copyright 2026 addisonk. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// sparkRunes are the standard 8-glyph unicode sparkline alphabet, lowest to highest.
var sparkRunes = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// newMMRCurveCmd implements `mmr-curve <player> --playlist <k>` —
// renders an ASCII sparkline of MMR over the look-back window for the
// chosen playlist. With --json, returns the same shape as `trend`.
func newMMRCurveCmd(flags *rootFlags) *cobra.Command {
	var playlist string
	var days int
	cmd := &cobra.Command{
		Use:         "mmr-curve <player>",
		Short:       "ASCII sparkline of MMR over the last N days for one playlist.",
		Long:        "Reads the same series as `trend` and renders it as an 8-glyph unicode sparkline. With --json, emits the per-day MMR series instead of the sparkline.",
		Example:     "  rocket-league-tracker-pp-cli mmr-curve 550e8400-e29b-41d4-a716-446655440000 --playlist 2v2",
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

			byDay := map[string]rankSnapshot{}
			latest := map[string]time.Time{}
			for _, s := range snaps {
				if !strings.EqualFold(s.Playlist, playlist) {
					continue
				}
				day := s.CapturedAt.UTC().Format("2006-01-02")
				if t, ok := latest[day]; !ok || s.CapturedAt.After(t) {
					byDay[day] = s
					latest[day] = s.CapturedAt
				}
			}

			type entry struct {
				Date string `json:"date"`
				MMR  int    `json:"mmr"`
				Tier string `json:"tier"`
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
			values := make([]int, len(out))
			for i, e := range out {
				values[i] = e.MMR
			}
			line := sparkline(values)
			start := out[0].MMR
			end := out[len(out)-1].MMR
			delta := end - start
			sign := "+"
			if delta < 0 {
				sign = ""
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s: %s (%d → %d, %s%d)\n", player, playlist, line, start, end, sign, delta)
			return nil
		},
	}
	cmd.Flags().StringVar(&playlist, "playlist", "", "Playlist: 1v1, 2v2, 3v3, hoops, rumble, dropshot, snowday, tournament")
	cmd.Flags().IntVar(&days, "days", 30, "Look-back window in days")
	return cmd
}

// sparkline maps int values to the 8-glyph unicode sparkline alphabet.
func sparkline(values []int) string {
	if len(values) == 0 {
		return ""
	}
	min := values[0]
	max := values[0]
	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	span := max - min
	var sb strings.Builder
	for _, v := range values {
		if span == 0 {
			sb.WriteRune(sparkRunes[0])
			continue
		}
		idx := (v - min) * (len(sparkRunes) - 1) / span
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sparkRunes) {
			idx = len(sparkRunes) - 1
		}
		sb.WriteRune(sparkRunes[idx])
	}
	return sb.String()
}
