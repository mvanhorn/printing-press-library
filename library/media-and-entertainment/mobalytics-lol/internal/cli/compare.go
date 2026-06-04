// Copyright 2026 QuantumGlitch and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"

	moba "github.com/mvanhorn/printing-press-library/library/media-and-entertainment/mobalytics-lol/internal/mobalytics"
	"github.com/spf13/cobra"
)

// newCompareCmd renders two champions side-by-side: tier, WR/PR/BR,
// top-3 counters, top-3 synergies, and the size of the items overlap
// between their MOST_POPULAR builds.
func newCompareCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "compare <c1> <c2>",
		Short:       "Side-by-side join across tier, build, counters, and matchups for two champions.",
		Example:     `  mobalytics-lol-pp-cli compare jinx kaisa`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			client := moba.NewClient(flags.timeout)
			fetch := func(slug string) (moba.ChampionBuildPage, error) {
				html, err := client.Fetch(moba.ChampionPath(slug, "build"))
				if err != nil {
					return moba.ChampionBuildPage{}, err
				}
				cHTML, cErr := client.Fetch(moba.ChampionPath(slug, "counters"))
				if cErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "  warning: failed to fetch counters for %s: %v\n", slug, cErr)
				}
				counters := moba.ParseCounters(cHTML, slug)
				moba.SortCountersByDelta(counters, true) // descending = best counters
				if len(counters) > 3 {
					counters = counters[:3]
				}
				syn := moba.ParseSynergies(html, slug)
				sort.SliceStable(syn, func(i, j int) bool { return syn[i].WinRate > syn[j].WinRate })
				if len(syn) > 3 {
					syn = syn[:3]
				}
				return moba.ChampionBuildPage{
					Slug:      slug,
					Stats:     moba.ParseChampionStats(html, slug),
					Builds:    moba.ParseBuilds(html),
					Counters:  counters,
					Synergies: syn,
				}, nil
			}
			a, err := fetch(args[0])
			if err != nil {
				return fmt.Errorf("fetch %s: %w", args[0], err)
			}
			b, err := fetch(args[1])
			if err != nil {
				return fmt.Errorf("fetch %s: %w", args[1], err)
			}
			itemSet := func(p moba.ChampionBuildPage) map[int]bool {
				out := map[int]bool{}
				if bld, ok := pickBuildType(p.Builds, ""); ok {
					for _, blk := range bld.Items {
						for _, it := range blk.Items {
							out[it] = true
						}
					}
				}
				return out
			}
			ai := itemSet(a)
			bi := itemSet(b)
			intersect := 0
			union := map[int]bool{}
			for k := range ai {
				union[k] = true
				if bi[k] {
					intersect++
				}
			}
			for k := range bi {
				union[k] = true
			}
			overlap := 0.0
			if len(union) > 0 {
				overlap = float64(intersect) * 100.0 / float64(len(union))
			}
			return flags.printJSON(cmd, map[string]any{
				"a":                  a,
				"b":                  b,
				"itemOverlapPercent": overlap,
				"itemOverlapCount":   intersect,
			})
		},
	}
	return cmd
}
