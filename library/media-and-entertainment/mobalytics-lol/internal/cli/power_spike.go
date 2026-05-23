// Copyright 2026 quantumglitch. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	moba "github.com/mvanhorn/printing-press-library/library/media-and-entertainment/mobalytics-lol/internal/mobalytics"
)

// newPowerSpikeCmd ranks champions by early/mid/late spike strength.
//
// Mobalytics's HTML pages don't ship a flat "powerSpike" field — the data
// is rendered client-side from skillOrder / item time-to-target. We
// approximate phase strength from the MOST_POPULAR build's item
// time-to-target buckets:
//   - early:  earliest Core item completion < 800s
//   - mid:    Core completed 800–1200s
//   - late:   Core completed > 1200s
//
// This is a coarse proxy; the README annotates it as such.
func newPowerSpikeCmd(flags *rootFlags) *cobra.Command {
	var phase, role string
	var top int
	cmd := &cobra.Command{
		Use:   "power-spike",
		Short: "Rank champions by early/mid/late spike strength (approximated from item time-to-target).",
		Long: `Phase is approximated from MOST_POPULAR build Core item
time-to-target: <800s ≈ early, 800–1200s ≈ mid, >1200s ≈ late. Mobalytics
does not expose a flat power-spike field on its HTML pages.`,
		Example:     `  mobalytics-lol-pp-cli power-spike --phase early --role ADC --top 10`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			phase = strings.ToLower(phase)
			if phase != "early" && phase != "mid" && phase != "late" {
				return fmt.Errorf("--phase must be one of: early, mid, late")
			}
			if dryRunOK(flags) {
				return nil
			}
			client := moba.NewClient(flags.timeout)
			html, err := client.Fetch(moba.TierListPath(role, "", "", ""))
			if err != nil {
				return err
			}
			rows := moba.FilterTierRows(moba.ParseTierList(html), role, "")
			moba.SortTierRows(rows)
			// Limit candidates: top tier list, top-30 by default before per-champ scan
			candidates := rows
			if len(candidates) > 30 {
				candidates = candidates[:30]
			}
			type entry struct {
				Slug       string `json:"slug"`
				Tier       string `json:"tier"`
				Role       string `json:"role"`
				CoreTimeSec int    `json:"coreTimeToTargetSec"`
				Phase      string `json:"phase"`
			}
			out := []entry{}
			seen := map[string]bool{}
			for _, r := range candidates {
				if seen[r.Slug] {
					continue
				}
				seen[r.Slug] = true
				bhtml, err := client.Fetch(moba.ChampionPath(r.Slug, "build"))
				if err != nil {
					continue
				}
				builds := moba.ParseBuilds(bhtml)
				b, ok := pickBuildType(builds, "")
				if !ok {
					continue
				}
				ttt := 0
				for _, blk := range b.Items {
					if strings.EqualFold(blk.Type, "Core") {
						ttt = blk.TimeToT
						break
					}
				}
				// Skip champions with no surfaced Core data. ttt == 0 is
				// the parser's "no Core block found" signal (Go's int
				// zero value), not a real 0-second time-to-target.
				// Without this guard, every champion missing Core data
				// would silently classify as "late".
				if ttt <= 0 {
					continue
				}
				var p string
				switch {
				case ttt < 800:
					p = "early"
				case ttt <= 1200:
					p = "mid"
				default:
					p = "late"
				}
				if p != phase {
					continue
				}
				out = append(out, entry{Slug: r.Slug, Tier: r.Tier, Role: r.Role, CoreTimeSec: ttt, Phase: p})
			}
			sort.SliceStable(out, func(i, j int) bool { return out[i].CoreTimeSec < out[j].CoreTimeSec })
			if top > 0 && top < len(out) {
				out = out[:top]
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&phase, "phase", "early", "Game phase: early, mid, or late.")
	cmd.Flags().StringVar(&role, "role", "", "Optional role filter.")
	cmd.Flags().IntVar(&top, "top", 10, "Top-N rows after sort.")
	return cmd
}
