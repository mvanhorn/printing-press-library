// Copyright 2026 NicholasSpisak and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"
)

// pp:data-source local

// compareRow is one retailer's listing for the compared identifier.
type compareRow struct {
	RetailerID      string   `json:"retailer_id"`
	ProductID       string   `json:"product_id"`
	ProductName     *string  `json:"product_name"`
	CurrentPrice    *float64 `json:"current_price"`
	InStock         *bool    `json:"in_stock"`
	DeltaToCheapest *float64 `json:"delta_to_cheapest"`
	ProductURL      *string  `json:"product_url"`
}

// compareResult is the typed envelope returned by `compare`.
type compareResult struct {
	Identifier string       `json:"identifier"`
	LookupType string       `json:"lookup_type"`
	Count      int          `json:"count"`
	Cheapest   *float64     `json:"cheapest"`
	Results    []compareRow `json:"results"`
}

func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	var flagLookupType string
	var flagInStock bool
	var flagLimit int
	var flagDB string

	cmd := &cobra.Command{
		Use:     "compare <identifier>",
		Short:   "See which synced retailer sells the exact same item (by UPC/EAN/GTIN/ASIN) cheapest right now, ranked",
		Example: "  shopping-pp-cli compare 012345678905 --lookup-type upc --in-stock --json",
		// pp:no-error-path-probe: an unknown identifier legitimately yields zero
		// matches (empty results, exit 0), not an error — the command cannot
		// distinguish a bad identifier from a valid one with no synced rows.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compare retailers for the given identifier from the local store")
				return nil
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return usageErr(err)
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("identifier is required"))
			}
			identifier := args[0]

			lookupType := flagLookupType
			if lookupType == "" {
				lookupType = "upc"
			}
			switch lookupType {
			case "upc", "ean", "gtin", "asin":
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --lookup-type %q: must be one of upc, ean, gtin, asin", lookupType))
			}

			limit := flagLimit
			if limit <= 0 {
				limit = 50
			}

			ctx := cmd.Context()
			db, err := openShopStore(ctx, resolveShopDBPath(flagDB))
			if err != nil {
				return err
			}
			defer db.Close()

			var (
				query string
				qArgs []any
			)
			selectCols := `SELECT retailers_id,
				id,
				json_extract(data,'$.product_name'),
				json_extract(data,'$.current_price'),
				json_extract(data,'$.in_stock'),
				json_extract(data,'$.product_url')
			FROM products`

			if lookupType == "asin" {
				query = selectCols + `
				WHERE json_extract(data,'$.asin') = ?
				   OR EXISTS (SELECT 1 FROM json_each(data,'$.asins') WHERE value = ?)`
				qArgs = append(qArgs, identifier, identifier)
			} else {
				query = selectCols + `
				WHERE json_extract(data,'$.` + lookupType + `') = ?`
				qArgs = append(qArgs, identifier)
			}
			if flagInStock {
				query += ` AND json_extract(data,'$.in_stock') = 1`
			}
			// NULLS LAST: rows with a price sort ahead of price-less rows.
			query += ` ORDER BY (json_extract(data,'$.current_price') IS NULL), json_extract(data,'$.current_price') ASC LIMIT ?`
			qArgs = append(qArgs, limit)

			rows, err := db.Query(query, qArgs...)
			if err != nil {
				return fmt.Errorf("query products: %w", err)
			}
			defer rows.Close()

			result := compareResult{
				Identifier: identifier,
				LookupType: lookupType,
				Results:    []compareRow{},
			}
			for rows.Next() {
				var (
					rid   string
					pid   string
					name  sql.NullString
					price sql.NullFloat64
					stock sql.NullInt64
					url   sql.NullString
				)
				if err := rows.Scan(&rid, &pid, &name, &price, &stock, &url); err != nil {
					return fmt.Errorf("scan product row: %w", err)
				}
				result.Results = append(result.Results, compareRow{
					RetailerID:   rid,
					ProductID:    pid,
					ProductName:  nullableString(name),
					CurrentPrice: nullableFloat(price),
					InStock:      nullableBool(stock),
					ProductURL:   nullableString(url),
				})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating product rows: %w", err)
			}

			result.Count = len(result.Results)
			// First priced row (already sorted ascending, NULLs last) is the cheapest.
			for i := range result.Results {
				if result.Results[i].CurrentPrice != nil {
					cheapest := *result.Results[i].CurrentPrice
					result.Cheapest = &cheapest
					break
				}
			}
			if result.Cheapest != nil {
				for i := range result.Results {
					if result.Results[i].CurrentPrice != nil {
						d := *result.Results[i].CurrentPrice - *result.Cheapest
						result.Results[i].DeltaToCheapest = &d
					}
				}
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&flagLookupType, "lookup-type", "upc", "Identifier type: upc, ean, gtin, or asin")
	cmd.Flags().BoolVar(&flagInStock, "in-stock", false, "Only include rows that are currently in stock")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum number of retailer rows to return")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local store (defaults to SHOPPING_DB or the share path)")
	return cmd
}
