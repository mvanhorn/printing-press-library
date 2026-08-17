// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"sort"

	"github.com/spf13/cobra"
)

func newNovelReviewsUniqueBodiesCmd(flags *rootFlags) *cobra.Command {
	var flagPopulation string
	var flagRating int
	var flagProduct string
	var flagDB string

	cmd := &cobra.Command{
		Use:         "unique-bodies",
		Short:       "Return one deterministic representative per normalized review body while retaining source row counts.",
		Example:     "  judgeme-pp-cli reviews unique-bodies --population published --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJudgeMeDryRun(cmd, flags, "reviews unique-bodies", flagPopulation)
			}
			rows, syncedAt, err := loadJudgeMeReviews(cmd.Context(), flagDB, judgeMeReviewFilter{
				Population: flagPopulation,
				Rating:     flagRating,
				Product:    flagProduct,
			})
			if err != nil {
				return err
			}
			grouped := map[string][]judgeMeReview{}
			for _, row := range rows {
				if row.BodyHash == "" {
					continue
				}
				grouped[row.BodyHash] = append(grouped[row.BodyHash], row)
			}
			var result []map[string]any
			for hash, matches := range grouped {
				sort.Slice(matches, func(i, j int) bool { return numericStringLess(matches[i].ID, matches[j].ID) })
				representative := matches[0]
				result = append(result, map[string]any{
					"body_hash":         hash,
					"representative_id": representative.ID,
					"source_row_count":  len(matches),
					"rating":            representative.Rating,
					"created_at":        representative.CreatedAt,
					"body":              representative.Body,
					"product_ids":       distinctProducts(matches),
				})
			}
			sort.Slice(result, func(i, j int) bool {
				return numericStringLess(result[i]["representative_id"].(string), result[j]["representative_id"].(string))
			})
			return printJudgeMeLocalResult(cmd, flags, result, syncedAt, flagPopulation)
		},
	}
	cmd.Flags().StringVar(&flagPopulation, "population", "", "Required population: published, hidden, pending, or all")
	cmd.Flags().IntVar(&flagRating, "rating", 0, "Filter by rating 1-5 (0 = all ratings)")
	cmd.Flags().StringVar(&flagProduct, "product", "", "Filter by product external ID or product handle")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database file path")
	return cmd
}

func distinctProducts(rows []judgeMeReview) []string {
	set := map[string]struct{}{}
	for _, row := range rows {
		product := row.ProductExternalID
		if product == "" {
			product = row.ProductHandle
		}
		if product != "" {
			set[product] = struct{}{}
		}
	}
	products := make([]string, 0, len(set))
	for product := range set {
		products = append(products, product)
	}
	sort.Strings(products)
	return products
}
