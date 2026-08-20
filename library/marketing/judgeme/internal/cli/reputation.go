// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// Absorbed from the established Judge.me printed CLI and adapted to the
// count-verified local corpus.
// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		root.AddCommand(newReputationCmd(flags))
	})
}

func newReputationCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "reputation",
		Short:       "Trust and reputation views built from the verified Judge.me corpus",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newReputationSummaryCmd(flags))
	cmd.AddCommand(newReputationProductsCmd(flags))
	cmd.AddCommand(newReputationModerationQueueCmd(flags))
	cmd.AddCommand(newReputationSettingsAuditCmd(flags))
	cmd.AddCommand(newReputationProductCmd(flags))
	return cmd
}

func newReputationSummaryCmd(flags *rootFlags) *cobra.Command {
	var population, dbPath string
	cmd := &cobra.Command{
		Use:     "summary",
		Short:   "Summarize ratings and deduplicated bodies for one explicit population",
		Example: "  judgeme-pp-cli reputation summary --population published --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--population=published;--dry-run=true",
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dryRunOK(flags) {
				return printJudgeMeDryRun(cmd, flags, "reputation summary", population)
			}
			rows, syncedAt, err := loadJudgeMeReviews(cmd.Context(), dbPath, judgeMeReviewFilter{Population: population})
			if err != nil {
				return err
			}
			var ratingSum, lowRatings int
			for _, row := range rows {
				ratingSum += row.Rating
				if row.Rating > 0 && row.Rating <= 2 {
					lowRatings++
				}
			}
			average := 0.0
			if len(rows) > 0 {
				average = float64(ratingSum) / float64(len(rows))
			}
			result := []map[string]any{{
				"population":        population,
				"row_count":         len(rows),
				"unique_bodies":     uniqueBodyCount(rows),
				"average_rating":    average,
				"low_rating_rows":   lowRatings,
				"dedupe_difference": len(rows) - uniqueBodyCount(rows),
			}}
			return printJudgeMeLocalResult(cmd, flags, result, syncedAt, population)
		},
	}
	cmd.Flags().StringVar(&population, "population", "", "Required population: published, hidden, pending, or all")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}

func newReputationProductsCmd(flags *rootFlags) *cobra.Command {
	var population, dbPath string
	var limit, minReviews int
	cmd := &cobra.Command{
		Use:     "products",
		Short:   "Rank products by low-rating risk for one explicit population",
		Example: "  judgeme-pp-cli reputation products --population published --limit 20 --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--population=published;--limit=20;--dry-run=true",
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dryRunOK(flags) {
				return printJudgeMeDryRun(cmd, flags, "reputation products", population)
			}
			rows, syncedAt, err := loadJudgeMeReviews(cmd.Context(), dbPath, judgeMeReviewFilter{Population: population})
			if err != nil {
				return err
			}
			type aggregate struct {
				Title       string
				Rows        int
				RatingTotal int
				Low         int
				Hashes      map[string]struct{}
			}
			products := map[string]*aggregate{}
			for _, row := range rows {
				key := row.ProductExternalID
				if key == "" {
					key = row.ProductHandle
				}
				if key == "" {
					continue
				}
				current := products[key]
				if current == nil {
					current = &aggregate{Title: row.ProductTitle, Hashes: map[string]struct{}{}}
					products[key] = current
				}
				current.Rows++
				current.RatingTotal += row.Rating
				if row.Rating > 0 && row.Rating <= 2 {
					current.Low++
				}
				if row.BodyHash != "" {
					current.Hashes[row.BodyHash] = struct{}{}
				}
			}
			var result []map[string]any
			for key, current := range products {
				if current.Rows < minReviews {
					continue
				}
				result = append(result, map[string]any{
					"product":         key,
					"title":           current.Title,
					"population":      population,
					"row_count":       current.Rows,
					"unique_bodies":   len(current.Hashes),
					"average_rating":  float64(current.RatingTotal) / float64(current.Rows),
					"low_rating_rows": current.Low,
				})
			}
			sort.Slice(result, func(i, j int) bool {
				li := result[i]["low_rating_rows"].(int)
				lj := result[j]["low_rating_rows"].(int)
				if li != lj {
					return li > lj
				}
				return result[i]["product"].(string) < result[j]["product"].(string)
			})
			if limit > 0 && len(result) > limit {
				result = result[:limit]
			}
			return printJudgeMeLocalResult(cmd, flags, result, syncedAt, population)
		},
	}
	cmd.Flags().StringVar(&population, "population", "", "Required population: published, hidden, pending, or all")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum products to return")
	cmd.Flags().IntVar(&minReviews, "min-reviews", 1, "Minimum review rows per product")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}

func newReputationModerationQueueCmd(flags *rootFlags) *cobra.Command {
	var population, dbPath string
	var limit, maxRating int
	cmd := &cobra.Command{
		Use:     "moderation-queue",
		Short:   "List low-rated or uncurated reviews from one explicit population",
		Example: "  judgeme-pp-cli reputation moderation-queue --population all --max-rating 2 --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--population=all;--max-rating=2;--dry-run=true",
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dryRunOK(flags) {
				return printJudgeMeDryRun(cmd, flags, "reputation moderation-queue", population)
			}
			rows, syncedAt, err := loadJudgeMeReviews(cmd.Context(), dbPath, judgeMeReviewFilter{Population: population})
			if err != nil {
				return err
			}
			var result []map[string]any
			for _, row := range rows {
				curated := strings.ToLower(scalarString(row.Object["curated"]))
				if row.Rating > maxRating && curated != "" && curated != "not-yet" && curated != "pending" {
					continue
				}
				result = append(result, map[string]any{
					"id":            row.ID,
					"population":    population,
					"rating":        row.Rating,
					"curated":       curated,
					"product_title": row.ProductTitle,
					"created_at":    row.CreatedAt,
					"body_hash":     row.BodyHash,
				})
			}
			if limit > 0 && len(result) > limit {
				result = result[:limit]
			}
			return printJudgeMeLocalResult(cmd, flags, result, syncedAt, population)
		},
	}
	cmd.Flags().StringVar(&population, "population", "", "Required population: published, hidden, pending, or all")
	cmd.Flags().IntVar(&maxRating, "max-rating", 2, "Maximum rating included automatically")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum reviews to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}

func newReputationSettingsAuditCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "settings-audit",
		Short:       "Read the live settings that affect trust presentation",
		Example:     "  judgeme-pp-cli reputation settings-audit --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get(cmd.Context(), "/settings", map[string]string{"setting_keys[]": "widget_title"})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var result any
			if err := json.Unmarshal(data, &result); err != nil {
				return fmt.Errorf("decoding settings: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	return cmd
}

func newReputationProductCmd(flags *rootFlags) *cobra.Command {
	var population, product, dbPath string
	cmd := &cobra.Command{
		Use:     "product",
		Short:   "Show count and dedupe evidence for one product and population",
		Example: "  judgeme-pp-cli reputation product --product <external-id-or-handle> --population published --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--product=dogfood-sample;--population=published;--dry-run=true",
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dryRunOK(flags) {
				return printJudgeMeDryRun(cmd, flags, "reputation product", population)
			}
			if product == "" {
				return usageErr(fmt.Errorf("--product is required"))
			}
			rows, syncedAt, err := loadJudgeMeReviews(cmd.Context(), dbPath, judgeMeReviewFilter{Population: population, Product: product})
			if err != nil {
				return err
			}
			result := []map[string]any{{
				"product":       product,
				"population":    population,
				"row_count":     len(rows),
				"unique_bodies": uniqueBodyCount(rows),
				"review_ids":    reviewIDs(rows),
			}}
			return printJudgeMeLocalResult(cmd, flags, result, syncedAt, population)
		},
	}
	cmd.Flags().StringVar(&product, "product", "", "Product external ID or handle")
	cmd.Flags().StringVar(&population, "population", "", "Required population: published, hidden, pending, or all")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}
