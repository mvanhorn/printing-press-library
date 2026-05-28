// Copyright 2026 QuantumGlitch and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"

	moba "github.com/mvanhorn/printing-press-library/library/media-and-entertainment/mobalytics-lol/internal/mobalytics"
	"github.com/spf13/cobra"
)

// newFlexCmd finds champions appearing in 2+ roles at ≥A tier on the same
// patch + rank.
func newFlexCmd(flags *rootFlags) *cobra.Command {
	var rank string
	var minRoles int
	var minTier string
	cmd := &cobra.Command{
		Use:         "flex",
		Short:       "Champions that are ≥A-tier in 2+ roles for the same patch and rank.",
		Example:     `  mobalytics-lol-pp-cli flex --rank high-elo --min-roles 2`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if minRoles <= 0 {
				minRoles = 2
			}
			if minTier == "" {
				minTier = "A"
			}
			tierRank := map[string]int{"S+": 0, "S": 1, "A": 2, "B": 3, "C": 4, "D": 5}
			cutoff, ok := tierRank[minTier]
			if !ok {
				return fmt.Errorf("--min-tier %q is not valid; must be one of: S+, S, A, B, C, D", minTier)
			}
			client := moba.NewClient(flags.timeout)
			// One tier-list fetch carries all roles already.
			html, err := client.Fetch(moba.TierListPath("", rank, "", ""))
			if err != nil {
				return err
			}
			rows := moba.ParseTierList(html)
			if rank != "" {
				rows = moba.FilterTierRows(rows, "", rank)
			}
			byChamp := map[string]map[string]string{}
			for _, r := range rows {
				if tierRank[r.Tier] > cutoff {
					continue
				}
				if byChamp[r.Slug] == nil {
					byChamp[r.Slug] = map[string]string{}
				}
				byChamp[r.Slug][r.Role] = r.Tier
			}
			type flexRow struct {
				Slug  string            `json:"slug"`
				Roles map[string]string `json:"roles"`
				Count int               `json:"count"`
			}
			out := []flexRow{}
			for slug, roles := range byChamp {
				if len(roles) >= minRoles {
					out = append(out, flexRow{Slug: slug, Roles: roles, Count: len(roles)})
				}
			}
			sort.SliceStable(out, func(i, j int) bool {
				if out[i].Count != out[j].Count {
					return out[i].Count > out[j].Count
				}
				return out[i].Slug < out[j].Slug
			})
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&rank, "rank", "", "Optional skill-level filter (low-elo, high-elo).")
	cmd.Flags().IntVar(&minRoles, "min-roles", 2, "Minimum roles in which the champion must be ≥minTier.")
	cmd.Flags().StringVar(&minTier, "min-tier", "A", "Minimum tier counted as flex-viable.")
	return cmd
}
