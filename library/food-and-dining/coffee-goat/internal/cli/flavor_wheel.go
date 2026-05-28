// Copyright 2026 Justin Fu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/refdata"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
	"github.com/spf13/cobra"
)

// flavorWheelReport is the structured output. Tree maps top-level
// SCA category -> middle -> leaf -> weight; preferred / avoided
// list (top, middle, leaf) triples with weight stats.
type flavorWheelReport struct {
	Tree           map[string]map[string]map[string]float64 `json:"tree"`
	Preferred      []flavorWheelHit                         `json:"preferred"`
	Avoided        []flavorWheelHit                         `json:"avoided"`
	SparseCoverage []flavorWheelHit                         `json:"sparse_coverage"`
	BrewsScored    int                                      `json:"brews_scored"`
	Sources        []string                                 `json:"sources"`
}

type flavorWheelHit struct {
	Section string  `json:"section"`
	Weight  float64 `json:"weight"`
	Touches int     `json:"touches"`
}

func newFlavorWheelCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "flavor-wheel",
		Short: "Map user brew ratings onto the SCA Coffee Tasters' Flavor Wheel",
		Example: `  coffee-goat-pp-cli flavor-wheel --agent
  coffee-goat-pp-cli flavor-wheel --select preferred,avoided,sparse_coverage`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()

			report, err := buildFlavorWheelReport(db)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "flavor wheel (%d brews):\n", report.BrewsScored)
			fmt.Fprintln(cmd.OutOrStdout(), "  preferred:")
			for _, h := range report.Preferred {
				fmt.Fprintf(cmd.OutOrStdout(), "    %s (weight %.1f, %d touches)\n", h.Section, h.Weight, h.Touches)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "  avoided:")
			for _, h := range report.Avoided {
				fmt.Fprintf(cmd.OutOrStdout(), "    %s (weight %.1f, %d touches)\n", h.Section, h.Weight, h.Touches)
			}
			return nil
		},
	}
	return cmd
}

func buildFlavorWheelReport(db *store.Store) (*flavorWheelReport, error) {
	type brewRow struct {
		Rating    int
		TagsLower string
	}
	rows, err := db.DB().Query(
		`SELECT COALESCE(b.rating,0), COALESCE(rp.tags_json,'') || ' ' || COALESCE(rp.body_text,'') || ' ' || COALESCE(b.descriptors_json,'')
		 FROM brews b
		 LEFT JOIN beans bn ON b.bean_id = bn.id
		 LEFT JOIN roaster_products rp ON bn.roaster_slug = rp.roaster_slug AND bn.product_slug = rp.handle`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var all []brewRow
	for rows.Next() {
		var r brewRow
		var combined string
		if err := rows.Scan(&r.Rating, &combined); err != nil {
			return nil, err
		}
		r.TagsLower = strings.ToLower(combined)
		all = append(all, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate brews rows: %w", err)
	}

	weights := map[string]float64{}
	touches := map[string]int{}
	sections := refdata.FlattenSections()
	type key = string
	for _, b := range all {
		if b.TagsLower == "" {
			continue
		}
		for _, s := range sections {
			label := s.Top
			if s.Middle != "" {
				label += "/" + s.Middle
				if s.Leaf != "" {
					label += "/" + s.Leaf
				}
			}
			needle := strings.ToLower(s.Leaf)
			if needle == "" {
				needle = strings.ToLower(s.Middle)
			}
			if needle == "" {
				needle = strings.ToLower(s.Top)
			}
			if strings.Contains(b.TagsLower, needle) {
				if b.Rating >= 7 {
					weights[label] += float64(b.Rating - 6) // 7->1, 10->4
				} else if b.Rating > 0 && b.Rating <= 4 {
					weights[label] += -float64(5 - b.Rating) // 4->-1, 1->-4
				}
				touches[label]++
			}
		}
	}

	// Build tree.
	tree := map[string]map[string]map[string]float64{}
	for label, w := range weights {
		parts := strings.SplitN(label, "/", 3)
		top := parts[0]
		mid := ""
		leaf := ""
		if len(parts) > 1 {
			mid = parts[1]
		}
		if len(parts) > 2 {
			leaf = parts[2]
		}
		if tree[top] == nil {
			tree[top] = map[string]map[string]float64{}
		}
		if tree[top][mid] == nil {
			tree[top][mid] = map[string]float64{}
		}
		tree[top][mid][leaf] = w
	}

	var hits []flavorWheelHit
	for label, w := range weights {
		hits = append(hits, flavorWheelHit{Section: label, Weight: w, Touches: touches[label]})
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Weight > hits[j].Weight })

	var preferred, avoided, sparse []flavorWheelHit
	for _, h := range hits {
		switch {
		case h.Weight > 0 && h.Touches >= 1:
			if len(preferred) < 5 {
				preferred = append(preferred, h)
			}
		case h.Weight < 0:
			avoided = append(avoided, h)
		case h.Touches > 0 && h.Touches < 2:
			sparse = append(sparse, h)
		}
	}

	return &flavorWheelReport{
		Tree:           tree,
		Preferred:      preferred,
		Avoided:        avoided,
		SparseCoverage: sparse,
		BrewsScored:    len(all),
		Sources:        []string{"SCA Coffee Tasters' Flavor Wheel (sca.coffee)"},
	}, nil
}
