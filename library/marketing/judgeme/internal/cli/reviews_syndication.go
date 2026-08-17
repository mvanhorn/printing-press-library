// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"sort"

	"github.com/spf13/cobra"
)

func newNovelReviewsSyndicationCmd(flags *rootFlags) *cobra.Command {
	var flagMinProducts int
	var flagPopulation string
	var flagDB string

	cmd := &cobra.Command{
		Use:     "syndication",
		Short:   "Find identical normalized review bodies attached to multiple products.",
		Example: "  judgeme-pp-cli reviews syndication --population all --min-products 2 --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--population=all;--min-products=2;--dry-run=true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJudgeMeDryRun(cmd, flags, "reviews syndication", flagPopulation)
			}
			if flagMinProducts < 2 {
				return usageErr(errMinProducts)
			}
			rows, syncedAt, err := loadJudgeMeReviews(cmd.Context(), flagDB, judgeMeReviewFilter{Population: flagPopulation})
			if err != nil {
				return err
			}
			type group struct {
				rows     []judgeMeReview
				products map[string]struct{}
			}
			groups := map[string]*group{}
			for _, row := range rows {
				if row.BodyHash == "" {
					continue
				}
				current := groups[row.BodyHash]
				if current == nil {
					current = &group{products: map[string]struct{}{}}
					groups[row.BodyHash] = current
				}
				current.rows = append(current.rows, row)
				product := row.ProductExternalID
				if product == "" {
					product = row.ProductHandle
				}
				if product != "" {
					current.products[product] = struct{}{}
				}
			}
			var result []map[string]any
			for hash, current := range groups {
				if len(current.products) < flagMinProducts {
					continue
				}
				products := make([]string, 0, len(current.products))
				for product := range current.products {
					products = append(products, product)
				}
				sort.Strings(products)
				result = append(result, map[string]any{
					"body_hash":     hash,
					"row_count":     len(current.rows),
					"product_count": len(products),
					"products":      products,
					"review_ids":    reviewIDs(current.rows),
				})
			}
			sort.Slice(result, func(i, j int) bool {
				return result[i]["row_count"].(int) > result[j]["row_count"].(int)
			})
			return printJudgeMeLocalResult(cmd, flags, result, syncedAt, flagPopulation)
		},
	}
	cmd.Flags().IntVar(&flagMinProducts, "min-products", 2, "Minimum distinct product associations")
	cmd.Flags().StringVar(&flagPopulation, "population", "", "Required population: published, hidden, pending, or all")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database file path")
	return cmd
}

var errMinProducts = &judgeMeUsageError{"--min-products must be at least 2"}

type judgeMeUsageError struct{ message string }

func (e *judgeMeUsageError) Error() string { return e.message }

func reviewIDs(rows []judgeMeReview) []string {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return numericStringLess(ids[i], ids[j]) })
	return ids
}
