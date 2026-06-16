// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// pp:data-source local

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func newNovelRankCmd(flags *rootFlags) *cobra.Command {
	var flagType string
	var flagMaxPrice float64
	var flagMinRevenue float64
	var flagLimit int
	var flagSort string
	var flagDB string

	cmd := &cobra.Command{
		Use:     "rank",
		Short:   "Rank niches across all four buckets at once by a composite of estimated revenue, sales, and price.",
		Example: "  kdpnichefinder-pp-cli rank --max-price 9.99 --sort value --limit 10",
		Long: "Use to rank niches across all four buckets by opportunity. " +
			"Do NOT use for per-folder totals or single-book competitor sets.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			switch flagSort {
			case "opportunity", "value", "sales", "revenue":
			default:
				return usageErr(fmt.Errorf("invalid --sort %q (valid: opportunity, value, sales, revenue)", flagSort))
			}
			if err := validateBucket(flagType); err != nil {
				return err
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, _, missing, err := openKDPLocal(ctx, flags, flagDB, cmd.OutOrStdout())
			if err != nil {
				return err
			}
			if missing {
				return nil
			}
			defer db.Close()

			niches, err := loadNiches(ctx, db, flagType)
			if err != nil {
				return err
			}

			filtered := make([]nicheRow, 0, len(niches))
			for _, n := range niches {
				if flagMaxPrice > 0 && n.Price > flagMaxPrice {
					continue
				}
				if flagMinRevenue > 0 && n.Revenue < flagMinRevenue {
					continue
				}
				filtered = append(filtered, n)
			}

			sort.SliceStable(filtered, func(i, j int) bool {
				a, b := filtered[i], filtered[j]
				switch flagSort {
				case "value":
					return valuePerDollar(a) > valuePerDollar(b)
				case "sales":
					return a.Sales > b.Sales
				case "revenue":
					return a.Revenue > b.Revenue
				default: // opportunity
					if a.Revenue != b.Revenue {
						return a.Revenue > b.Revenue
					}
					return a.Sales > b.Sales
				}
			})

			if flagLimit > 0 && len(filtered) > flagLimit {
				filtered = filtered[:flagLimit]
			}

			type rankRow struct {
				Rank                    int     `json:"rank"`
				ID                      string  `json:"id"`
				Title                   string  `json:"title"`
				Bucket                  string  `json:"bucket"`
				Price                   string  `json:"price"`
				EstimatedMonthlySales   int     `json:"estimated_monthly_sales"`
				EstimatedMonthlyRevenue float64 `json:"estimated_monthly_revenue"`
				ASIN                    string  `json:"asin"`
			}
			out := make([]rankRow, 0, len(filtered))
			for i, n := range filtered {
				out = append(out, rankRow{
					Rank:                    i + 1,
					ID:                      n.ID,
					Title:                   n.Title,
					Bucket:                  n.Bucket,
					Price:                   n.PriceStr,
					EstimatedMonthlySales:   n.Sales,
					EstimatedMonthlyRevenue: n.Revenue,
					ASIN:                    n.ASIN,
				})
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&flagType, "type", "", "Filter to a single bucket (evergreen, fresh_money, hidden_gems, high_ticket)")
	cmd.Flags().Float64Var(&flagMaxPrice, "max-price", 0, "Only rank niches at or below this cover price")
	cmd.Flags().Float64Var(&flagMinRevenue, "min-revenue", 0, "Only rank niches at or above this estimated monthly revenue")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Maximum number of niches to return")
	cmd.Flags().StringVar(&flagSort, "sort", "opportunity", "Sort mode: opportunity, value, sales, or revenue")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local mirror database (defaults to the standard location)")
	return cmd
}

// valuePerDollar is revenue per cover-price dollar; 0 when price is unknown.
func valuePerDollar(n nicheRow) float64 {
	if n.Price <= 0 {
		return 0
	}
	return n.Revenue / n.Price
}
