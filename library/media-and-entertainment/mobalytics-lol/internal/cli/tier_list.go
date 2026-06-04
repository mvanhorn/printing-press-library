// Copyright 2026 QuantumGlitch and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	moba "github.com/mvanhorn/printing-press-library/library/media-and-entertainment/mobalytics-lol/internal/mobalytics"
	"github.com/spf13/cobra"
)

// newTierListCmd is the headline command: a flat tier list scraped from
// mobalytics.gg/lol/tier-list, optionally filtered by role/rank/patch and
// optionally pivoted side-by-side across regions.
func newTierListCmd(flags *rootFlags) *cobra.Command {
	var (
		role           string
		rank           string
		patch          string
		region         string
		top            int
		compareRegions string
	)
	cmd := &cobra.Command{
		Use:   "tier-list",
		Short: "Mobalytics LoL tier list, optionally filtered or pivoted across regions.",
		Long: `Fetch https://mobalytics.gg/lol/tier-list and return the flat tier list.

Filters narrow by role (TOP/JUNGLE/MID/ADC/SUPPORT) and rank
(low-elo/high-elo). --compare-regions pivots the same patch + rank across
multiple regions side-by-side — pick-priority drift between KR/EUW/NA is
not surfaced by Mobalytics's default view.`,
		Example: `  mobalytics-lol-pp-cli tier-list --role ADC --rank high-elo --top 20
  mobalytics-lol-pp-cli tier-list --compare-regions kr,euw,na --role MID --top 10`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			client := moba.NewClient(flags.timeout)

			if compareRegions != "" {
				regions := splitCSV(compareRegions)
				type rowKey struct{ slug, role, rank string }
				type pivot struct {
					Slug       string            `json:"slug"`
					Role       string            `json:"role"`
					SkillLevel string            `json:"skillLevel"`
					Tiers      map[string]string `json:"tiers"`
				}
				pivots := map[rowKey]*pivot{}
				for _, r := range regions {
					html, err := client.Fetch(moba.TierListPath(role, rank, patch, r))
					if err != nil {
						return fmt.Errorf("region=%s: %w", r, err)
					}
					rows := moba.FilterTierRows(moba.ParseTierList(html), role, rank)
					for _, row := range rows {
						k := rowKey{row.Slug, row.Role, row.SkillLevel}
						p, ok := pivots[k]
						if !ok {
							p = &pivot{Slug: row.Slug, Role: row.Role, SkillLevel: row.SkillLevel, Tiers: map[string]string{}}
							pivots[k] = p
						}
						p.Tiers[r] = row.Tier
					}
				}
				out := make([]pivot, 0, len(pivots))
				for _, p := range pivots {
					out = append(out, *p)
				}
				sort.Slice(out, func(i, j int) bool {
					return out[i].Slug < out[j].Slug
				})
				if top > 0 && top < len(out) {
					out = out[:top]
				}
				return flags.printJSON(cmd, out)
			}

			html, err := client.Fetch(moba.TierListPath(role, rank, patch, region))
			if err != nil {
				return err
			}
			rows := moba.ParseTierList(html)
			rows = moba.FilterTierRows(rows, role, rank)
			moba.SortTierRows(rows)
			if top > 0 && top < len(rows) {
				rows = rows[:top]
			}
			return flags.printJSON(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&role, "role", "", "Filter by role (TOP, JUNGLE, MID, ADC, SUPPORT).")
	cmd.Flags().StringVar(&rank, "rank", "", "Filter by skill level (low-elo, high-elo).")
	cmd.Flags().StringVar(&patch, "patch", "", "Patch override (e.g. 16.10). Mobalytics defaults to current.")
	cmd.Flags().StringVar(&region, "region", "", "Region (kr, euw, na). Mobalytics defaults to global.")
	cmd.Flags().IntVar(&top, "top", 0, "Limit to top-N rows after sort (0 = all).")
	cmd.Flags().StringVar(&compareRegions, "compare-regions", "", "Pivot the same query across N regions side-by-side, e.g. kr,euw,na.")
	return cmd
}

// splitCSV splits a CSV string and trims spaces. Empty fields are dropped.
func splitCSV(in string) []string {
	parts := strings.Split(in, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
