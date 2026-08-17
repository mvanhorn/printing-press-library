// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelReviewsPopulationsCmd(flags *rootFlags) *cobra.Command {
	var flagDB string

	cmd := &cobra.Command{
		Use:         "populations",
		Short:       "Count storefront-visible and internal moderation populations with unambiguous labels.",
		Example:     "  judgeme-pp-cli reviews populations --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJudgeMeDryRun(cmd, flags, "reviews populations", "all")
			}
			rows, syncedAt, err := loadJudgeMeReviews(cmd.Context(), flagDB, judgeMeReviewFilter{Population: "all"})
			if err != nil {
				return err
			}
			counts := map[string]int{"all": len(rows)}
			for _, row := range rows {
				switch {
				case row.Published:
					counts["published"]++
				case row.Hidden:
					counts["hidden"]++
				default:
					counts["pending"]++
				}
			}
			result := []map[string]any{
				{"population": "all", "rows": counts["all"], "description": "all API-returned review rows"},
				{"population": "published", "rows": counts["published"], "description": "storefront-visible reviews"},
				{"population": "hidden", "rows": counts["hidden"], "description": "reviews hidden from the storefront"},
				{"population": "pending", "rows": counts["pending"], "description": "neither published nor hidden"},
			}
			return printJudgeMeLocalResult(cmd, flags, result, syncedAt, "all")
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database file path")
	return cmd
}
