// Copyright 2026 NicholasSpisak and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"
)

// pp:data-source local

// leaderboardRow aggregates one retailer's discount/price posture.
type leaderboardRow struct {
	RetailerID   string   `json:"retailer_id"`
	ProductCount int      `json:"product_count"`
	AvgDiscount  *float64 `json:"avg_discount"`
	OnSaleCount  int      `json:"on_sale_count"`
	AvgPrice     *float64 `json:"avg_price"`
}

func newNovelLeaderboardCmd(flags *rootFlags) *cobra.Command {
	var flagCategory string
	var flagBy string
	var flagLimit int
	var flagDB string

	cmd := &cobra.Command{
		Use:         "leaderboard",
		Short:       "Show which retailers and categories are consistently the deepest-discount buckets in your synced data",
		Example:     "  shopping-pp-cli leaderboard --by avg-discount --limit 15 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank retailers by discount posture from the local store")
				return nil
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return usageErr(err)
			}

			by := flagBy
			if by == "" {
				by = "avg-discount"
			}
			switch by {
			case "avg-discount", "on-sale-count", "avg-price":
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --by %q: must be one of avg-discount, on-sale-count, avg-price", by))
			}

			limit := flagLimit
			if limit <= 0 {
				limit = 15
			}

			ctx := cmd.Context()
			db, err := openShopStore(ctx, resolveShopDBPath(flagDB))
			if err != nil {
				return err
			}
			defer db.Close()

			var qArgs []any
			where := ""
			if flagCategory != "" {
				where = ` WHERE json_extract(data,'$.category') LIKE ?`
				qArgs = append(qArgs, "%"+flagCategory+"%")
			}

			var orderBy string
			switch by {
			case "on-sale-count":
				orderBy = "on_sale_count DESC"
			case "avg-price":
				orderBy = "avg_price DESC"
			default:
				orderBy = "avg_discount DESC"
			}

			query := `SELECT retailers_id,
				COUNT(*) AS product_count,
				AVG(json_extract(data,'$.discount_percentage')) AS avg_discount,
				SUM(CASE WHEN json_extract(data,'$.discount_percentage') > 0 THEN 1 ELSE 0 END) AS on_sale_count,
				AVG(json_extract(data,'$.current_price')) AS avg_price
			FROM products` + where + `
			GROUP BY retailers_id
			ORDER BY ` + orderBy + `
			LIMIT ?`
			qArgs = append(qArgs, limit)

			rows, err := db.Query(query, qArgs...)
			if err != nil {
				return fmt.Errorf("query leaderboard: %w", err)
			}
			defer rows.Close()

			results := []leaderboardRow{}
			for rows.Next() {
				var (
					rid         string
					count       int
					avgDiscount sql.NullFloat64
					onSale      int
					avgPrice    sql.NullFloat64
				)
				if err := rows.Scan(&rid, &count, &avgDiscount, &onSale, &avgPrice); err != nil {
					return fmt.Errorf("scan leaderboard row: %w", err)
				}
				results = append(results, leaderboardRow{
					RetailerID:   rid,
					ProductCount: count,
					AvgDiscount:  nullableFloat(avgDiscount),
					OnSaleCount:  onSale,
					AvgPrice:     nullableFloat(avgPrice),
				})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating leaderboard rows: %w", err)
			}

			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&flagCategory, "category", "", "Restrict to a category (substring match)")
	cmd.Flags().StringVar(&flagBy, "by", "avg-discount", "Ranking metric: avg-discount, on-sale-count, or avg-price")
	cmd.Flags().IntVar(&flagLimit, "limit", 15, "Maximum number of retailers to return")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local store (defaults to SHOPPING_DB or the share path)")
	return cmd
}
