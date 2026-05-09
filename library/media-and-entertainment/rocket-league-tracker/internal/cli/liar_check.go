// Copyright 2026 addisonk. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newLiarCheckCmd implements `liar-check <player> --claimed-rank <tier>` —
// verifies the claim against the player's MAX MMR across all playlists,
// returning the truthiness and the gap.
func newLiarCheckCmd(flags *rootFlags) *cobra.Command {
	var claimedRank string
	var platform string
	cmd := &cobra.Command{
		Use:         "liar-check <player>",
		Short:       "Verify a player's claimed rank against their actual MAX MMR.",
		Long:        "Calls /ranks/{playerId}, computes the player's max MMR across playlists, and checks whether it meets the lower bound of the claimed tier. Tier names accept short forms (GC, SSL, P2, etc.).",
		Example:     "  rocket-league-tracker-pp-cli liar-check 550e8400-e29b-41d4-a716-446655440000 --claimed-rank GC",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if claimedRank == "" {
				return usageErr(fmt.Errorf("--claimed-rank is required (e.g. GC, Champion 2, SSL)"))
			}
			canonical, ok := resolveTier(claimedRank)
			if !ok {
				return usageErr(fmt.Errorf("unknown rank %q; expected names like 'GC', 'Champion 2', 'SSL'", claimedRank))
			}
			lower := tierLowerBound(canonical)

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
			maxMMR := 0
			maxTier := ""
			for _, s := range snaps {
				if s.MMR > maxMMR {
					maxMMR = s.MMR
					maxTier = s.Tier
				}
			}
			if maxTier == "" {
				maxTier = tierFromMMR(maxMMR)
			}
			truth := maxMMR >= lower
			overstated := lower - maxMMR
			if overstated < 0 {
				overstated = 0
			}

			result := map[string]any{
				"truth":           truth,
				"claimed":         canonical,
				"actual_max_mmr":  maxMMR,
				"actual_max_tier": maxTier,
				"overstated_by":   overstated,
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			if truth {
				fmt.Fprintf(cmd.OutOrStdout(), "True — actually %s (%d MMR)\n", maxTier, maxMMR)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Overstated by %d MMR (actually %s)\n", overstated, maxTier)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&claimedRank, "claimed-rank", "", "Claimed tier name or alias (e.g. GC, Champion 2, SSL)")
	cmd.Flags().StringVar(&platform, "platform", "epic", "Platform: epic, steam, psn, xbox")
	_ = platform
	return cmd
}
