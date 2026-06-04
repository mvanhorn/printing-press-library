// Copyright 2026 QuantumGlitch and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"

	moba "github.com/mvanhorn/printing-press-library/library/media-and-entertainment/mobalytics-lol/internal/mobalytics"
	"github.com/spf13/cobra"
)

// newMetaShiftCmd diffs the current tier list against a prior patch and
// reports champions that moved up or down by ≥1 tier.
func newMetaShiftCmd(flags *rootFlags) *cobra.Command {
	var sincePatch, role, rank string
	cmd := &cobra.Command{
		Use:   "meta-shift",
		Short: "Champions that moved up or down in tier since a prior patch.",
		Long: `Fetch the current tier list, fetch tier-list?patch=<sincePatch>,
and report champions whose tier letter changed by at least one level.
Note: Mobalytics's historical tier endpoint is occasionally restricted to
recent patches; older values may return the current tier (no diff). If
that happens, switch --since-patch to a recent value (e.g. 16.9).`,
		Example:     `  mobalytics-lol-pp-cli meta-shift --since-patch 16.9 --role ADC`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if sincePatch == "" {
				return fmt.Errorf("--since-patch is required (e.g. 16.9)")
			}
			if dryRunOK(flags) {
				return nil
			}
			client := moba.NewClient(flags.timeout)
			nowHTML, err := client.Fetch(moba.TierListPath(role, rank, "", ""))
			if err != nil {
				return fmt.Errorf("fetch current tier list: %w", err)
			}
			thenHTML, err := client.Fetch(moba.TierListPath(role, rank, sincePatch, ""))
			if err != nil {
				return fmt.Errorf("fetch patch=%s tier list: %w", sincePatch, err)
			}
			now := moba.FilterTierRows(moba.ParseTierList(nowHTML), role, rank)
			then := moba.FilterTierRows(moba.ParseTierList(thenHTML), role, rank)

			key := func(r moba.TierRow) string { return r.Slug + "|" + r.Role + "|" + r.SkillLevel }
			thenMap := map[string]string{}
			for _, t := range then {
				thenMap[key(t)] = t.Tier
			}
			tierRank := map[string]int{"S+": 0, "S": 1, "A": 2, "B": 3, "C": 4, "D": 5}
			type shift struct {
				Slug       string `json:"slug"`
				Role       string `json:"role"`
				SkillLevel string `json:"skillLevel"`
				From       string `json:"fromTier"`
				To         string `json:"toTier"`
				Direction  string `json:"direction"`
			}
			out := []shift{}
			identical := 0
			for _, r := range now {
				prev, ok := thenMap[key(r)]
				if !ok {
					continue
				}
				if prev == r.Tier {
					identical++
					continue
				}
				dir := "side"
				if tierRank[r.Tier] < tierRank[prev] {
					dir = "up"
				} else if tierRank[r.Tier] > tierRank[prev] {
					dir = "down"
				}
				out = append(out, shift{r.Slug, r.Role, r.SkillLevel, prev, r.Tier, dir})
			}
			sort.SliceStable(out, func(i, j int) bool {
				if out[i].Direction != out[j].Direction {
					return out[i].Direction < out[j].Direction
				}
				return out[i].Slug < out[j].Slug
			})
			// Distinguish "no shifts" from "prior-patch endpoint returned
			// current tier" (the documented failure mode). When zero
			// shifts surface AND every champion matched on both sides
			// with identical tiers, the most likely cause is that the
			// historical endpoint silently fell back to current data.
			if len(out) == 0 && identical > 0 && identical == len(now) {
				fmt.Fprintf(cmd.ErrOrStderr(), "meta-shift: 0 tier changes detected across %d champions vs patch %s. Mobalytics's historical tier endpoint may be returning the current tier (documented quirk). Try a more recent --since-patch value (e.g. one or two patches back).\n", identical, sincePatch)
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&sincePatch, "since-patch", "", "Compare current tier list against this earlier patch (required).")
	cmd.Flags().StringVar(&role, "role", "", "Optional role filter (TOP, JUNGLE, MID, ADC, SUPPORT).")
	cmd.Flags().StringVar(&rank, "rank", "", "Optional skill-level filter (low-elo, high-elo).")
	return cmd
}
