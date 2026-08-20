// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

// gaps cross-tabulates two tag facets across the cached catalog to surface
// under-served template combinations — a local pivot no Placeit call exposes.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelGapsCmd(flags *rootFlags) *cobra.Command {
	var facet, by, category string
	var minCount int

	cmd := &cobra.Command{
		Use:   "gaps",
		Short: "Pivot the catalog across two tag facets to surface under-served combinations.",
		Long: strings.Trim(`
Cross-tabulate two tag facets (e.g. device_tags by ethnicity_tags) across your
local mirror to find under-served combinations — coverage gaps the Placeit UI
never surfaces. Run 'sync' first. Combinations at or below --min are reported
as gaps. Tag facets: device_tags, stage_tags, color_tags, gender_tags,
age_tags, ethnicity_tags, bundle_tags.`, "\n"),
		Example: strings.Trim(`
  placeit-pp-cli gaps --facet device_tags --by ethnicity_tags
  placeit-pp-cli gaps --facet device_tags --by color_tags --category mockups --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if facet == "" || by == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("both --facet and --by are required"))
			}
			db, ok, err := openCatalogStore(cmd, flags)
			if err != nil || !ok {
				return err
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, templateResource) {
				hintIfStale(cmd, db, templateResource, flags.maxAge)
			}
			all, err := loadCatalog(db)
			if err != nil {
				return apiErr(err)
			}
			wantCat, _ := catFacetOrEmpty(category)
			// pivot[aValue][bValue] = count
			pivot := map[string]map[string]int{}
			aTotals := map[string]int{}
			bValuesSet := map[string]struct{}{}
			for _, m := range all {
				if wantCat != "" && fmt.Sprint(m["category_name"]) != wantCat {
					continue
				}
				as := tagValues(m, facet)
				bs := tagValues(m, by)
				if len(as) == 0 || len(bs) == 0 {
					continue
				}
				for _, a := range as {
					if pivot[a] == nil {
						pivot[a] = map[string]int{}
					}
					for _, b := range bs {
						pivot[a][b]++
						aTotals[a]++
						bValuesSet[b] = struct{}{}
					}
				}
			}
			if len(pivot) == 0 {
				return flags.printJSON(cmd, map[string]any{
					"facet": facet, "by": by, "rows": []any{}, "gaps": []any{},
					"note": "no templates carried both facets; widen --category or run a broader sync",
				})
			}
			bValues := make([]string, 0, len(bValuesSet))
			for b := range bValuesSet {
				bValues = append(bValues, b)
			}
			sort.Strings(bValues)

			// Rank A values by total volume; report the most prominent ones.
			aValues := make([]string, 0, len(aTotals))
			for a := range aTotals {
				aValues = append(aValues, a)
			}
			sort.SliceStable(aValues, func(i, j int) bool { return aTotals[aValues[i]] > aTotals[aValues[j]] })
			if len(aValues) > 25 {
				aValues = aValues[:25]
			}

			type cell struct {
				A     string `json:"a"`
				B     string `json:"b"`
				Count int    `json:"count"`
			}
			rows := make([]map[string]any, 0, len(aValues))
			gaps := make([]cell, 0)
			for _, a := range aValues {
				counts := map[string]int{}
				for _, b := range bValues {
					c := pivot[a][b]
					counts[b] = c
					if c <= minCount {
						gaps = append(gaps, cell{A: a, B: b, Count: c})
					}
				}
				rows = append(rows, map[string]any{"value": a, "total": aTotals[a], "by": counts})
			}
			sort.SliceStable(gaps, func(i, j int) bool { return gaps[i].Count < gaps[j].Count })
			return flags.printJSON(cmd, map[string]any{
				"facet":     facet,
				"by":        by,
				"b_values":  bValues,
				"rows":      rows,
				"gaps":      gaps,
				"gap_count": len(gaps),
			})
		},
	}
	cmd.Flags().StringVar(&facet, "facet", "", "Primary tag facet (rows), e.g. device_tags")
	cmd.Flags().StringVar(&by, "by", "", "Secondary tag facet (columns), e.g. ethnicity_tags")
	cmd.Flags().StringVar(&category, "category", "", "Restrict the pivot to a category")
	cmd.Flags().IntVar(&minCount, "min", 0, "Combinations with count <= this are reported as gaps")
	return cmd
}
