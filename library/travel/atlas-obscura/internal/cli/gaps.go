// Copyright 2026 David Bryson and contributors. Licensed under Apache-2.0. See LICENSE.
//
// `gaps` — unvisited wonders near a point, ranked by interestingness (hand-authored).
// Joins the live geo search against the local visited table.
package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func newNovelGapsCmd(flags *rootFlags) *cobra.Command {
	var radius float64
	var minScore int
	var limit int
	var maxScanPages int

	cmd := &cobra.Command{
		Use:   "gaps <place-or-latlng>",
		Short: "Show good wonders near a point that you haven't visited yet, ranked by interestingness.",
		Long: "List worthwhile wonders near a place that are NOT in your visited log, ranked by\n" +
			"interestingness score. Combines a live geo search with your local visited table.\n" +
			"Community-sourced from atlasobscura.com; not an official API.",
		Example: "  atlas-obscura-pp-cli gaps \"Portland, Oregon\" --radius 40 --min-score 6 --json\n" +
			"  atlas-obscura-pp-cli gaps \"45.52,-122.68\"",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list unvisited wonders near a point")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a place name or \"lat,lng\" is required"))
			}
			if limit < 1 {
				limit = 15
			}
			if maxScanPages < 1 {
				maxScanPages = 5
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			lat, lng, label, err := resolvePoint(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			s, err := aoDB(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			if err := ensureAOTables(s); err != nil {
				return err
			}
			visited, err := visitedIDs(s)
			if err != nil {
				return err
			}

			// Collect a generous nearby pool, then exclude visited and rank.
			pool, _, _, err := collectNear(cmd.Context(), c, lat, lng, nearFilter{
				radius:       radius,
				minScore:     0,
				limit:        limit * 4,
				maxScanPages: maxScanPages,
				images:       false,
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			gaps := make([]AOPlace, 0, len(pool))
			for _, p := range pool {
				if visited[p.ID] {
					continue
				}
				if minScore > 0 && p.Score < minScore {
					continue
				}
				gaps = append(gaps, p)
				cachePlace(s, p)
			}
			sort.SliceStable(gaps, func(i, j int) bool {
				return gaps[i].Score > gaps[j].Score
			})
			if len(gaps) > limit {
				gaps = gaps[:limit]
			}

			return aoEmitPlaces(cmd, flags, map[string]any{
				"origin":        label,
				"visited_count": len(visited),
			}, gaps)
		},
	}
	cmd.Flags().Float64Var(&radius, "radius", 0, "Only include places within this many miles (0 = no limit)")
	cmd.Flags().IntVar(&minScore, "min-score", 0, "Only include places with at least this interestingness score (0-10)")
	cmd.Flags().IntVar(&limit, "limit", 15, "Maximum number of unvisited places to return")
	cmd.Flags().IntVar(&maxScanPages, "max-scan-pages", 5, "Maximum result pages to scan")
	return cmd
}
