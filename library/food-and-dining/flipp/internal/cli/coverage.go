// Copyright 2026 mlabrenz and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelCoverageCmd(flags *rootFlags) *cobra.Command {
	var flagZip string
	var flagLocale string

	cmd := &cobra.Command{
		Use:         "coverage",
		Short:       "Show which nearby merchants have active food flyers, searchable items, and coupon coverage.",
		Example:     "  flipp-pp-cli coverage --zip 85001 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch Flipp coverage near %s\n", flagZip)
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := fetchFlippData(ctx, c, flagZip, flagLocale)
			if err != nil {
				return err
			}
			type merchantCoverage struct {
				Merchant       string   `json:"merchant"`
				FlyerCount     int      `json:"flyer_count"`
				GroceryFlyers  int      `json:"grocery_flyers"`
				LatestValidTo  string   `json:"latest_valid_to,omitempty"`
				Categories     []string `json:"categories"`
				CouponPresence bool     `json:"coupon_presence"`
			}
			byMerchant := map[string]*merchantCoverage{}
			for _, flyer := range data.Flyers {
				entry := byMerchant[flyer.Merchant]
				if entry == nil {
					entry = &merchantCoverage{Merchant: flyer.Merchant, Categories: []string{}}
					byMerchant[flyer.Merchant] = entry
				}
				entry.FlyerCount++
				if hasCategory(flyer.Categories, "groceries") {
					entry.GroceryFlyers++
				}
				if flyer.ValidTo > entry.LatestValidTo {
					entry.LatestValidTo = flyer.ValidTo
				}
				for _, cat := range flyer.Categories {
					if !containsStringFold(entry.Categories, cat) {
						entry.Categories = append(entry.Categories, cat)
					}
				}
			}
			for _, coupon := range append(append([]flippCoupon{}, data.Coupons...), append(data.LoyaltyProgramCoupons, data.FlyerItemCoupons...)...) {
				if coupon.Merchant == "" {
					continue
				}
				entry := byMerchant[coupon.Merchant]
				if entry == nil {
					entry = &merchantCoverage{Merchant: coupon.Merchant, Categories: []string{}}
					byMerchant[coupon.Merchant] = entry
				}
				entry.CouponPresence = true
			}
			rowsOut := make([]merchantCoverage, 0, len(byMerchant))
			for _, entry := range byMerchant {
				sort.Strings(entry.Categories)
				rowsOut = append(rowsOut, *entry)
			}
			sort.SliceStable(rowsOut, func(i, j int) bool {
				if rowsOut[i].GroceryFlyers != rowsOut[j].GroceryFlyers {
					return rowsOut[i].GroceryFlyers > rowsOut[j].GroceryFlyers
				}
				return rowsOut[i].FlyerCount > rowsOut[j].FlyerCount
			})
			view := struct {
				Zip          string             `json:"zip"`
				MerchantRows []merchantCoverage `json:"merchants"`
				RefreshedAt  string             `json:"refreshed_at,omitempty"`
			}{Zip: flagZip, MerchantRows: rowsOut, RefreshedAt: data.RefreshedAt}
			tableRows := [][]string{}
			for _, row := range rowsOut {
				coupons := "no"
				if row.CouponPresence {
					coupons = "yes"
				}
				tableRows = append(tableRows, []string{row.Merchant, strconv.Itoa(row.FlyerCount), strconv.Itoa(row.GroceryFlyers), coupons, strings.Join(row.Categories, ", ")})
			}
			return printRowsOrJSON(cmd, flags, view, []string{"Merchant", "Flyers", "Grocery", "Coupons", "Categories"}, tableRows)
		},
	}
	cmd.Flags().StringVar(&flagZip, "zip", "85001", "ZIP or postal code used to find local coverage")
	cmd.Flags().StringVar(&flagLocale, "locale", defaultFlippLocale, "Flipp locale, such as en-us or en-ca")
	return cmd
}
