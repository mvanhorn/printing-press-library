// Copyright 2026 addisonk. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"math"
	"sort"

	"github.com/spf13/cobra"
)

// newPromoCmd implements `promo <player>` — for each playlist, computes the
// MMR points needed to reach the next tier and the approximate number of
// wins required (assuming ~9 MMR per win at most tiers).
func newPromoCmd(flags *rootFlags) *cobra.Command {
	var platform string
	cmd := &cobra.Command{
		Use:         "promo <player>",
		Short:       "Per-playlist MMR-to-promote and approximate wins-until-promo.",
		Long:        "Calls /ranks/{playerId} and computes how many MMR points and approximate wins are needed to reach the next tier in each playlist. Wins estimate uses an avg of 9 MMR per win.",
		Example:     "  rocket-league-tracker-pp-cli promo 550e8400-e29b-41d4-a716-446655440000",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			player := args[0]
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			liveRaw, err := c.Get("/ranks/"+pathEscapePlayer(player), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			snaps := extractRankPlaylists(extractResponseData(liveRaw))

			type rec struct {
				Playlist       string `json:"playlist"`
				CurrentMMR     int    `json:"current_mmr"`
				CurrentTier    string `json:"current_tier"`
				NextTier       string `json:"next_tier"`
				NextLowerBound int    `json:"next_lower_bound"`
				MMRDiff        int    `json:"mmr_diff"`
				WinsEstimate   int    `json:"wins_estimate"`
			}
			out := make([]rec, 0, len(snaps))
			for _, s := range snaps {
				curTier := s.Tier
				if curTier == "" {
					curTier = tierFromMMR(s.MMR)
				}
				name, lower := nextTierLowerBound(s.MMR)
				diff := lower - s.MMR
				wins := 0
				if diff > 0 {
					wins = int(math.Ceil(float64(diff) / 9.0))
				}
				out = append(out, rec{
					Playlist:       s.Playlist,
					CurrentMMR:     s.MMR,
					CurrentTier:    curTier,
					NextTier:       name,
					NextLowerBound: lower,
					MMRDiff:        diff,
					WinsEstimate:   wins,
				})
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Playlist < out[j].Playlist })

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no playlists found")
				return nil
			}
			for _, r := range out {
				if r.NextTier == "" {
					fmt.Fprintf(cmd.OutOrStdout(), "%s: %s — at top of ladder (%d MMR)\n", r.Playlist, r.CurrentTier, r.CurrentMMR)
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s @ %d → %s in %d MMR (~%d wins)\n", r.Playlist, r.CurrentTier, r.CurrentMMR, r.NextTier, r.MMRDiff, r.WinsEstimate)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&platform, "platform", "epic", "Platform: epic, steam, psn, xbox")
	_ = platform
	return cmd
}
