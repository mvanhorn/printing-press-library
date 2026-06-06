// Copyright 2026 NicholasSpisak and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source local

// arbitrageRow is one resale-opportunity row computed from a product's
// profitability sub-block.
type arbitrageRow struct {
	RetailerID    string   `json:"retailer_id"`
	ProductID     string   `json:"product_id"`
	ProductName   *string  `json:"product_name"`
	BuyPrice      *float64 `json:"buy_price"`
	AmazonPrice   *float64 `json:"amazon_price"`
	ProfitPerUnit *float64 `json:"profit_per_unit"`
	Margin        *float64 `json:"margin"`
	ROI           *float64 `json:"roi"`
	Status        *string  `json:"status"`
}

func newNovelArbitrageCmd(flags *rootFlags) *cobra.Command {
	var flagRetailer []string
	var flagMinROI float64
	var flagMinMargin float64
	var flagMaxBuyPrice float64
	var flagInStock bool
	var flagLimit int
	var flagDB string

	cmd := &cobra.Command{
		Use:         "arbitrage",
		Short:       "Rank synced products by what you would net reselling them on Amazon after referral and FBA fees",
		Example:     "  shopping-pp-cli arbitrage --retailer walmart --min-roi 30 --max-buy-price 50 --in-stock --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank resale-arbitrage opportunities from the local store by ROI")
				return nil
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return usageErr(err)
			}

			limit := flagLimit
			if limit <= 0 {
				limit = 25
			}

			ctx := cmd.Context()
			db, err := openShopStore(ctx, resolveShopDBPath(flagDB))
			if err != nil {
				return err
			}
			defer db.Close()

			where := []string{`json_extract(data,'$.profitability.roi') IS NOT NULL`}
			var qArgs []any
			if cmd.Flags().Changed("min-roi") {
				where = append(where, `json_extract(data,'$.profitability.roi') >= ?`)
				qArgs = append(qArgs, flagMinROI)
			}
			if cmd.Flags().Changed("min-margin") {
				where = append(where, `json_extract(data,'$.profitability.margin') >= ?`)
				qArgs = append(qArgs, flagMinMargin)
			}
			if cmd.Flags().Changed("max-buy-price") {
				where = append(where, `json_extract(data,'$.current_price') <= ?`)
				qArgs = append(qArgs, flagMaxBuyPrice)
			}
			if len(flagRetailer) > 0 {
				ph := make([]string, len(flagRetailer))
				for i, r := range flagRetailer {
					ph[i] = "?"
					qArgs = append(qArgs, r)
				}
				where = append(where, `retailers_id IN (`+strings.Join(ph, ",")+`)`)
			}
			if flagInStock {
				where = append(where, `json_extract(data,'$.in_stock') = 1`)
			}

			query := `SELECT retailers_id,
				id,
				json_extract(data,'$.product_name'),
				json_extract(data,'$.current_price'),
				json_extract(data,'$.profitability.amazon_price'),
				json_extract(data,'$.profitability.profit_per_unit'),
				json_extract(data,'$.profitability.margin'),
				json_extract(data,'$.profitability.roi'),
				json_extract(data,'$.profitability.status')
			FROM products
			WHERE ` + strings.Join(where, " AND ") + `
			ORDER BY json_extract(data,'$.profitability.roi') DESC
			LIMIT ?`
			qArgs = append(qArgs, limit)

			rows, err := db.Query(query, qArgs...)
			if err != nil {
				return fmt.Errorf("query arbitrage: %w", err)
			}
			defer rows.Close()

			results := []arbitrageRow{}
			for rows.Next() {
				var (
					rid    string
					pid    string
					name   sql.NullString
					buy    sql.NullFloat64
					amz    sql.NullFloat64
					profit sql.NullFloat64
					margin sql.NullFloat64
					roi    sql.NullFloat64
					status sql.NullString
				)
				if err := rows.Scan(&rid, &pid, &name, &buy, &amz, &profit, &margin, &roi, &status); err != nil {
					return fmt.Errorf("scan arbitrage row: %w", err)
				}
				results = append(results, arbitrageRow{
					RetailerID:    rid,
					ProductID:     pid,
					ProductName:   nullableString(name),
					BuyPrice:      nullableFloat(buy),
					AmazonPrice:   nullableFloat(amz),
					ProfitPerUnit: nullableFloat(profit),
					Margin:        nullableFloat(margin),
					ROI:           nullableFloat(roi),
					Status:        nullableString(status),
				})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating arbitrage rows: %w", err)
			}

			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringSliceVar(&flagRetailer, "retailer", nil, "Restrict to these retailer IDs (repeatable)")
	cmd.Flags().Float64Var(&flagMinROI, "min-roi", 0, "Minimum return on investment (percent)")
	cmd.Flags().Float64Var(&flagMinMargin, "min-margin", 0, "Minimum profit margin (percent)")
	cmd.Flags().Float64Var(&flagMaxBuyPrice, "max-buy-price", 0, "Maximum buy (current) price")
	cmd.Flags().BoolVar(&flagInStock, "in-stock", false, "Only include in-stock products")
	cmd.Flags().IntVar(&flagLimit, "limit", 25, "Maximum number of opportunities to return")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local store (defaults to SHOPPING_DB or the share path)")
	return cmd
}
