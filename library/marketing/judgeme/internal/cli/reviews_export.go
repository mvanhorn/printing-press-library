// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"strconv"

	"github.com/spf13/cobra"
)

func newNovelReviewsExportCmd(flags *rootFlags) *cobra.Command {
	var flagPopulation string
	var flagRating int
	var flagProduct string
	var flagDateFrom string
	var flagDateTo string
	var flagFormat string
	var flagDB string

	cmd := &cobra.Command{
		Use:     "export",
		Short:   "Export explicit published, hidden, pending, or all populations as JSON or CSV.",
		Example: "  judgeme-pp-cli reviews export --population published --rating 5 --format csv",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--population=published;--format=json;--dry-run=true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJudgeMeDryRun(cmd, flags, "reviews export", flagPopulation)
			}
			if flagRating < 0 || flagRating > 5 {
				return usageErr(strconv.ErrRange)
			}
			rows, syncedAt, err := loadJudgeMeReviews(cmd.Context(), flagDB, judgeMeReviewFilter{
				Population: flagPopulation,
				Rating:     flagRating,
				Product:    flagProduct,
				DateFrom:   flagDateFrom,
				DateTo:     flagDateTo,
			})
			if err != nil {
				return err
			}
			if flagFormat == "csv" || flags.csv {
				return writeJudgeMeReviewCSV(cmd.OutOrStdout(), rows)
			}
			if flagFormat != "json" {
				return usageErr(strconv.ErrSyntax)
			}
			raw := judgeMeRowsRaw(rows)
			meta := map[string]any{
				"source":        "local",
				"synced_at":     syncedAt,
				"population":    flagPopulation,
				"row_count":     len(rows),
				"unique_bodies": uniqueBodyCount(rows),
			}
			return printOutputWithFlagsMeta(cmd.OutOrStdout(), raw, flags, meta)
		},
	}
	cmd.Flags().StringVar(&flagPopulation, "population", "", "Required population: published, hidden, pending, or all")
	cmd.Flags().IntVar(&flagRating, "rating", 0, "Filter by rating 1-5 (0 = all ratings)")
	cmd.Flags().StringVar(&flagProduct, "product", "", "Filter by product external ID or product handle")
	cmd.Flags().StringVar(&flagDateFrom, "date-from", "", "Include reviews created on or after this ISO date/time")
	cmd.Flags().StringVar(&flagDateTo, "date-to", "", "Include reviews created before this ISO date/time")
	cmd.Flags().StringVar(&flagFormat, "format", "json", "Output format: json or csv")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database file path")
	return cmd
}
