// Copyright 2026 mlabrenz and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"fmt"
	"math"
	"sort"
	"strconv"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelUnitPriceCmd(flags *rootFlags) *cobra.Command {
	var flagZip string
	var flagLocale string
	var flagLimit int

	cmd := &cobra.Command{
		Use:         "unit-price <query>",
		Short:       "Normalize item prices by package size when Flipp listings include parseable quantities.",
		Example:     "  flipp-pp-cli unit-price milk --zip 85001 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("query is required"))
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would search Flipp for %q near %s and compute unit prices\n", args[0], flagZip)
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			res, err := fetchFlippSearch(ctx, c, args[0], flagZip, flagLocale, "price_low_to_high")
			if err != nil {
				return err
			}
			type unitRow struct {
				Name        string        `json:"name"`
				Merchant    string        `json:"merchant"`
				Price       *float64      `json:"price"`
				UnitPrice   unitPriceInfo `json:"unit_price"`
				ValidTo     string        `json:"valid_to,omitempty"`
				FlyerID     *int          `json:"flyer_id,omitempty"`
				ImageURL    string        `json:"image_url,omitempty"`
				DiscountPct *int          `json:"discount_pct,omitempty"`
			}
			rows := []unitRow{}
			for _, item := range append(append([]flippItem{}, res.Items...), res.EcomItems...) {
				if !matchesSearchIntent(item, args[0]) {
					continue
				}
				rows = append(rows, unitRow{
					Name:        item.Name,
					Merchant:    merchantName(item),
					Price:       item.CurrentPrice,
					UnitPrice:   parseUnitPrice(item.Name, item.CurrentPrice),
					ValidTo:     item.ValidTo,
					FlyerID:     item.FlyerID,
					ImageURL:    itemImage(item),
					DiscountPct: discountPct(item),
				})
			}
			sort.SliceStable(rows, func(i, j int) bool {
				iv, jv := math.Inf(1), math.Inf(1)
				if rows[i].UnitPrice.Value != nil {
					iv = *rows[i].UnitPrice.Value
				}
				if rows[j].UnitPrice.Value != nil {
					jv = *rows[j].UnitPrice.Value
				}
				return iv < jv
			})
			if flagLimit > 0 && len(rows) > flagLimit {
				rows = rows[:flagLimit]
			}
			view := struct {
				Zip     string    `json:"zip"`
				Query   string    `json:"query"`
				Results []unitRow `json:"results"`
			}{Zip: flagZip, Query: args[0], Results: rows}
			tableRows := [][]string{}
			for _, row := range rows {
				price := ""
				if row.Price != nil {
					price = fmt.Sprintf("%.2f", *row.Price)
				}
				unit := row.UnitPrice.Warning
				if row.UnitPrice.Value != nil {
					unit = fmt.Sprintf("%.2f/%s", *row.UnitPrice.Value, row.UnitPrice.Unit)
				}
				tableRows = append(tableRows, []string{row.Merchant, row.Name, price, unit, strconv.Itoa(valueOrZero(row.DiscountPct))})
			}
			return printRowsOrJSON(cmd, flags, view, []string{"Merchant", "Name", "Price", "Unit Price", "Discount%"}, tableRows)
		},
	}
	cmd.Flags().StringVar(&flagZip, "zip", "85001", "ZIP or postal code used to find local deals")
	cmd.Flags().StringVar(&flagLocale, "locale", defaultFlippLocale, "Flipp locale, such as en-us or en-ca")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Maximum rows to return")
	return cmd
}
