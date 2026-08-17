// Copyright 2026 David Bryson and contributors. Licensed under Apache-2.0. See LICENSE.
//
// `cluster` — group nearby wonders into walkable clusters (hand-authored).
// Greedy spatial clustering: seed at the densest unclustered place, absorb every
// place within --walk miles, repeat. Clusters with >= --min places are returned.
package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func newNovelClusterCmd(flags *rootFlags) *cobra.Command {
	var radius float64
	var min int
	var walk float64
	var limit int
	var maxScanPages int

	cmd := &cobra.Command{
		Use:   "cluster <place-or-latlng>",
		Short: "Group nearby wonders into spatially tight clusters that make a walkable half-day.",
		Long: "Group wonders near a point into tight clusters you can walk between. Each cluster\n" +
			"absorbs places within --walk miles of its seed; clusters with at least --min\n" +
			"places are returned, densest first. Community-sourced from atlasobscura.com; not an official API.",
		Example: "  atlas-obscura-pp-cli cluster \"Edinburgh\" --walk 0.6 --min 3 --json\n" +
			"  atlas-obscura-pp-cli cluster \"55.95,-3.19\" --radius 5",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would cluster nearby wonders")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a place name or \"lat,lng\" is required"))
			}
			if min < 2 {
				min = 2
			}
			if walk <= 0 {
				walk = 0.6
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

			pool, _, _, err := collectNear(cmd.Context(), c, lat, lng, nearFilter{
				radius:       radius,
				limit:        150,
				maxScanPages: maxScanPages,
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if s, derr := aoDB(cmd.Context()); derr == nil {
				for _, p := range pool {
					cachePlace(s, p)
				}
				_ = s.Close()
			}

			clusters := buildClusters(pool, walk, min)
			if limit > 0 && len(clusters) > limit {
				clusters = clusters[:limit]
			}

			out := map[string]any{
				"source":        aoSourceNote,
				"origin":        label,
				"walk_miles":    walk,
				"cluster_count": len(clusters),
				"clusters":      clusters,
			}
			return aoEmit(cmd, flags, out)
		},
	}
	cmd.Flags().Float64Var(&radius, "radius", 0, "Only consider places within this many miles of the origin (0 = no limit)")
	cmd.Flags().IntVar(&min, "min", 3, "Minimum places for a cluster to be reported")
	cmd.Flags().Float64Var(&walk, "walk", 0.6, "Walking radius in miles that defines a cluster")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum clusters to return (0 = all)")
	cmd.Flags().IntVar(&maxScanPages, "max-scan-pages", 5, "Maximum result pages to scan")
	return cmd
}

type aoCluster struct {
	CenterLat float64   `json:"center_lat"`
	CenterLng float64   `json:"center_lng"`
	Size      int       `json:"size"`
	SpanMiles float64   `json:"span_miles"`
	Places    []AOPlace `json:"places"`
}

// buildClusters greedily groups places: repeatedly pick the unclustered place
// whose --walk neighborhood is densest, form a cluster from it, remove its members.
func buildClusters(pool []AOPlace, walk float64, min int) []aoCluster {
	type node struct {
		p    AOPlace
		used bool
	}
	nodes := make([]*node, 0, len(pool))
	for _, p := range pool {
		if p.Lat == 0 && p.Lng == 0 {
			continue
		}
		nodes = append(nodes, &node{p: p})
	}
	var clusters []aoCluster
	for {
		// Find the densest seed among unused nodes.
		bestIdx, bestCount := -1, 0
		for i, n := range nodes {
			if n.used {
				continue
			}
			count := 0
			for _, m := range nodes {
				if m.used {
					continue
				}
				if haversineMiles(n.p.Lat, n.p.Lng, m.p.Lat, m.p.Lng) <= walk {
					count++
				}
			}
			if count > bestCount {
				bestCount, bestIdx = count, i
			}
		}
		if bestIdx < 0 || bestCount < min {
			break
		}
		seed := nodes[bestIdx]
		var members []AOPlace
		var sumLat, sumLng, span float64
		for _, m := range nodes {
			if m.used {
				continue
			}
			if haversineMiles(seed.p.Lat, seed.p.Lng, m.p.Lat, m.p.Lng) <= walk {
				m.used = true
				members = append(members, m.p)
				sumLat += m.p.Lat
				sumLng += m.p.Lng
			}
		}
		// Compute span (max pairwise distance) for the cluster.
		for i := range members {
			for j := i + 1; j < len(members); j++ {
				d := haversineMiles(members[i].Lat, members[i].Lng, members[j].Lat, members[j].Lng)
				if d > span {
					span = d
				}
			}
		}
		clusters = append(clusters, aoCluster{
			CenterLat: sumLat / float64(len(members)),
			CenterLng: sumLng / float64(len(members)),
			Size:      len(members),
			SpanMiles: span,
			Places:    members,
		})
	}
	sort.SliceStable(clusters, func(i, j int) bool { return clusters[i].Size > clusters[j].Size })
	return clusters
}
