// Copyright 2026 mlabrenz and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelDealsScanCmd(flags *rootFlags) *cobra.Command {
	var flagCategory string
	var flagZip string
	var flagMinDiscount string
	var flagLocale string
	var flagQueries string
	var flagLimit int

	cmd := &cobra.Command{
		Use:         "scan",
		Short:       "Scan curated staple categories and rank local deals by discount, urgency, and merchant.",
		Example:     "  flipp-pp-cli deals scan --category groceries --zip 85001 --min-discount 25 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would scan Flipp category %q near %s\n", flagCategory, flagZip)
				return nil
			}
			minDiscount := 0
			if flagMinDiscount != "" {
				n, err := strconv.Atoi(flagMinDiscount)
				if err != nil {
					return usageErr(fmt.Errorf("--min-discount must be an integer"))
				}
				minDiscount = n
			}
			queries := splitCSV(flagQueries)
			if len(queries) == 0 {
				cats := splitCSV(flagCategory)
				if len(cats) == 0 {
					cats = []string{"groceries"}
				}
				for _, cat := range cats {
					pack, ok := staplePacks[cat]
					if !ok {
						return usageErr(fmt.Errorf("unknown --category %q", cat))
					}
					queries = append(queries, pack...)
				}
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			type dealRow struct {
				Query       string   `json:"query"`
				Name        string   `json:"name"`
				Merchant    string   `json:"merchant"`
				Price       *float64 `json:"price"`
				Was         *float64 `json:"was"`
				DiscountPct *int     `json:"discount_pct,omitempty"`
				ValidTo     string   `json:"valid_to,omitempty"`
				FlyerID     *int     `json:"flyer_id,omitempty"`
				SaleStory   string   `json:"sale_story,omitempty"`
				ImageURL    string   `json:"image_url,omitempty"`
			}
			rowsOut := []dealRow{}
			failures := []fetchFailure{}
			seen := map[string]bool{}
			for _, query := range queries {
				res, err := fetchFlippSearch(ctx, c, query, flagZip, flagLocale, "")
				if err != nil {
					failures = append(failures, fetchFailure{Query: query, Error: err.Error()})
					continue
				}
				matches := append([]flippItem{}, res.Items...)
				matches = append(matches, res.EcomItems...)
				for _, item := range matches {
					if !matchesSearchIntent(item, query) {
						continue
					}
					dp := discountPct(item)
					if minDiscount > 0 && (dp == nil || *dp < minDiscount) {
						continue
					}
					key := fmt.Sprintf("%s|%s|%s", query, merchantName(item), item.Name)
					if seen[key] {
						continue
					}
					seen[key] = true
					rowsOut = append(rowsOut, dealRow{
						Query:       query,
						Name:        item.Name,
						Merchant:    merchantName(item),
						Price:       item.CurrentPrice,
						Was:         item.OriginalPrice,
						DiscountPct: dp,
						ValidTo:     item.ValidTo,
						FlyerID:     item.FlyerID,
						SaleStory:   item.SaleStory,
						ImageURL:    itemImage(item),
					})
				}
			}
			sort.SliceStable(rowsOut, func(i, j int) bool {
				ai, aj := -1, -1
				if rowsOut[i].DiscountPct != nil {
					ai = *rowsOut[i].DiscountPct
				}
				if rowsOut[j].DiscountPct != nil {
					aj = *rowsOut[j].DiscountPct
				}
				if ai != aj {
					return ai > aj
				}
				return itemPrice(flippItem{CurrentPrice: rowsOut[i].Price}) < itemPrice(flippItem{CurrentPrice: rowsOut[j].Price})
			})
			if flagLimit > 0 && len(rowsOut) > flagLimit {
				rowsOut = rowsOut[:flagLimit]
			}
			view := struct {
				Zip            string         `json:"zip"`
				Category       string         `json:"category"`
				ScannedQueries int            `json:"scanned_queries"`
				Deals          []dealRow      `json:"deals"`
				FetchFailures  []fetchFailure `json:"fetch_failures,omitempty"`
			}{Zip: flagZip, Category: flagCategory, ScannedQueries: len(queries), Deals: rowsOut, FetchFailures: failures}
			tableRows := [][]string{}
			for _, row := range rowsOut {
				disc := ""
				if row.DiscountPct != nil {
					disc = strconv.Itoa(*row.DiscountPct)
				}
				price := ""
				if row.Price != nil {
					price = fmt.Sprintf("%.2f", *row.Price)
				}
				tableRows = append(tableRows, []string{row.Query, row.Merchant, row.Name, price, disc})
			}
			return printRowsOrJSON(cmd, flags, view, []string{"Query", "Merchant", "Name", "Price", "Discount%"}, tableRows)
		},
	}
	cmd.Flags().StringVar(&flagCategory, "category", "groceries", "Comma-separated category packs to scan")
	cmd.Flags().StringVar(&flagZip, "zip", "85001", "ZIP or postal code used to find local deals")
	cmd.Flags().StringVar(&flagMinDiscount, "min-discount", "", "Minimum computed discount percentage")
	cmd.Flags().StringVar(&flagLocale, "locale", defaultFlippLocale, "Flipp locale, such as en-us or en-ca")
	cmd.Flags().StringVar(&flagQueries, "queries", "", "Comma-separated custom search terms; overrides --category")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Maximum ranked deals to return")
	return cmd
}
