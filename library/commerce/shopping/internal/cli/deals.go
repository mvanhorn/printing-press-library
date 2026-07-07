// Copyright 2026 NicholasSpisak and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source local

// dealRow is one ranked deal row.
type dealRow struct {
	RetailerID         string   `json:"retailer_id"`
	ProductID          string   `json:"product_id"`
	ProductName        *string  `json:"product_name"`
	Brand              *string  `json:"brand"`
	CurrentPrice       *float64 `json:"current_price"`
	OriginalPrice      *float64 `json:"original_price"`
	DiscountPercentage *float64 `json:"discount_percentage"`
	Rating             *float64 `json:"rating"`
	InStock            *bool    `json:"in_stock"`
	ProductURL         *string  `json:"product_url"`
}

func newNovelDealsCmd(flags *rootFlags) *cobra.Command {
	var flagRetailer []string
	var flagCategory string
	var flagMinDiscount float64
	var flagMaxPrice float64
	var flagMinRating float64
	var flagMinReviews int
	var flagBrand string
	var flagInStock bool
	var flagSort string
	var flagLimit int
	var flagDB string

	cmd := &cobra.Command{
		Use:         "deals",
		Short:       "Surface deals that clear several bars at once — deep discount, under a price ceiling, well-reviewed",
		Example:     "  shopping-pp-cli deals --min-discount 30 --max-price 100 --min-rating 4 --in-stock --sort discount --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank deals from the local store by discount or price")
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

			var where []string
			var qArgs []any
			if cmd.Flags().Changed("min-discount") {
				where = append(where, `json_extract(data,'$.discount_percentage') >= ?`)
				qArgs = append(qArgs, flagMinDiscount)
			}
			if cmd.Flags().Changed("max-price") {
				where = append(where, `json_extract(data,'$.current_price') <= ?`)
				qArgs = append(qArgs, flagMaxPrice)
			}
			if cmd.Flags().Changed("min-rating") {
				where = append(where, `json_extract(data,'$.rating') >= ?`)
				qArgs = append(qArgs, flagMinRating)
			}
			if cmd.Flags().Changed("min-reviews") {
				where = append(where, `json_extract(data,'$.review_count') >= ?`)
				qArgs = append(qArgs, flagMinReviews)
			}
			if flagCategory != "" {
				where = append(where, `json_extract(data,'$.category') LIKE ?`)
				qArgs = append(qArgs, "%"+flagCategory+"%")
			}
			if flagBrand != "" {
				where = append(where, `json_extract(data,'$.brand') = ?`)
				qArgs = append(qArgs, flagBrand)
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
				json_extract(data,'$.brand'),
				json_extract(data,'$.current_price'),
				json_extract(data,'$.original_price'),
				json_extract(data,'$.discount_percentage'),
				json_extract(data,'$.rating'),
				json_extract(data,'$.in_stock'),
				json_extract(data,'$.product_url')
			FROM products`
			if len(where) > 0 {
				query += " WHERE " + strings.Join(where, " AND ")
			}
			switch flagSort {
			case "price":
				query += ` ORDER BY (json_extract(data,'$.current_price') IS NULL), json_extract(data,'$.current_price') ASC`
			case "discount", "":
				query += ` ORDER BY json_extract(data,'$.discount_percentage') DESC`
			default:
				return usageErr(fmt.Errorf("--sort must be 'discount' or 'price', got %q", flagSort))
			}
			query += ` LIMIT ?`
			qArgs = append(qArgs, limit)

			rows, err := db.Query(query, qArgs...)
			if err != nil {
				return fmt.Errorf("query deals: %w", err)
			}
			defer rows.Close()

			results := []dealRow{}
			for rows.Next() {
				var (
					rid      string
					pid      string
					name     sql.NullString
					brand    sql.NullString
					price    sql.NullFloat64
					orig     sql.NullFloat64
					discount sql.NullFloat64
					rating   sql.NullFloat64
					stock    sql.NullInt64
					url      sql.NullString
				)
				if err := rows.Scan(&rid, &pid, &name, &brand, &price, &orig, &discount, &rating, &stock, &url); err != nil {
					return fmt.Errorf("scan deal row: %w", err)
				}
				results = append(results, dealRow{
					RetailerID:         rid,
					ProductID:          pid,
					ProductName:        nullableString(name),
					Brand:              nullableString(brand),
					CurrentPrice:       nullableFloat(price),
					OriginalPrice:      nullableFloat(orig),
					DiscountPercentage: nullableFloat(discount),
					Rating:             nullableFloat(rating),
					InStock:            nullableBool(stock),
					ProductURL:         nullableString(url),
				})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating deal rows: %w", err)
			}

			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringSliceVar(&flagRetailer, "retailer", nil, "Restrict to these retailer IDs (repeatable)")
	cmd.Flags().StringVar(&flagCategory, "category", "", "Filter by category (substring match)")
	cmd.Flags().Float64Var(&flagMinDiscount, "min-discount", 0, "Minimum discount percentage")
	cmd.Flags().Float64Var(&flagMaxPrice, "max-price", 0, "Maximum current price")
	cmd.Flags().Float64Var(&flagMinRating, "min-rating", 0, "Minimum product rating")
	cmd.Flags().IntVar(&flagMinReviews, "min-reviews", 0, "Minimum number of reviews")
	cmd.Flags().StringVar(&flagBrand, "brand", "", "Filter by brand (exact match)")
	cmd.Flags().BoolVar(&flagInStock, "in-stock", false, "Only include in-stock products")
	cmd.Flags().StringVar(&flagSort, "sort", "discount", "Sort order: discount or price")
	cmd.Flags().IntVar(&flagLimit, "limit", 25, "Maximum number of deals to return")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local store (defaults to SHOPPING_DB or the share path)")
	return cmd
}
