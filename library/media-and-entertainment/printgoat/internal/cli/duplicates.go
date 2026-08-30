// Copyright 2026 Nate Olson and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: cross-source duplicate detection.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// duplicateGroup is one cluster of likely-same-design listings found across
// two or more of the three sources.
type duplicateGroup struct {
	NormalizedName string        `json:"normalized_name"`
	Sources        []string      `json:"sources"`
	Listings       []novelResult `json:"listings"`
}

// groupDuplicates clusters results whose normalized-name token sets overlap
// at or above threshold (Jaccard similarity), then keeps only clusters that
// span more than one source — a same-source near-duplicate (two unrelated
// remixes on the same site) is not what this command is for.
func groupDuplicates(results []novelResult, threshold float64) []duplicateGroup {
	n := len(results)
	if n == 0 {
		return nil
	}
	tokens := make([]map[string]struct{}, n)
	for i, r := range results {
		tokens[i] = tokenSet(r.Name)
	}

	// Union-find over result indices.
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if jaccardSimilarity(tokens[i], tokens[j]) >= threshold {
				union(i, j)
			}
		}
	}

	byRoot := map[int][]int{}
	for i := 0; i < n; i++ {
		root := find(i)
		byRoot[root] = append(byRoot[root], i)
	}

	var groups []duplicateGroup
	for _, idxs := range byRoot {
		if len(idxs) < 2 {
			continue
		}
		sourceSet := map[string]struct{}{}
		listings := make([]novelResult, 0, len(idxs))
		for _, i := range idxs {
			sourceSet[results[i].Source] = struct{}{}
			listings = append(listings, results[i])
		}
		if len(sourceSet) < 2 {
			continue // same-source cluster, not a cross-source duplicate
		}
		sources := make([]string, 0, len(sourceSet))
		for s := range sourceSet {
			sources = append(sources, s)
		}
		groups = append(groups, duplicateGroup{
			NormalizedName: normalizeName(listings[0].Name),
			Sources:        sources,
			Listings:       listings,
		})
	}
	return groups
}

// pp:data-source computed
func newNovelDuplicatesCmd(flags *rootFlags) *cobra.Command {
	var limitPerSource int
	var threshold float64

	cmd := &cobra.Command{
		Use:         "duplicates <query>",
		Short:       "See instantly when the same design is listed free on one site and paid on another, before you download the wrong one.",
		Example:     `  printgoat-pp-cli duplicates "gopro mount" --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("missing required argument <query>\nUsage: %s <query>", cmd.CommandPath()))
			}
			query := args[0]

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			results, statuses := searchAllSourcesLive(ctx, c, query, limitPerSource)
			groups := groupDuplicates(results, threshold)

			out := map[string]any{
				"query":        query,
				"sources":      statuses,
				"duplicates":   groups,
				"group_count":  len(groups),
				"result_count": len(results),
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().IntVar(&limitPerSource, "limit", 20, "Maximum results to fetch per source before matching")
	cmd.Flags().Float64Var(&threshold, "threshold", 0.6, "Minimum Jaccard token-overlap similarity to treat two listings as the same design")
	return cmd
}
