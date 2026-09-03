// Copyright 2026 mlabrenz and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelBasketPriceCmd(flags *rootFlags) *cobra.Command {
	var flagItems string
	var flagZip string
	var flagLocale string
	var flagLimit int

	cmd := &cobra.Command{
		Use:         "price",
		Short:       "Compare a grocery list across nearby merchants and see the cheapest practical basket.",
		Example:     "  flipp-pp-cli basket price --items milk,eggs,bread --zip 85001 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would search Flipp for basket items near %s\n", flagZip)
				return nil
			}
			items := splitCSV(flagItems)
			if len(items) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--items is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			type basketItem struct {
				Query       string  `json:"query"`
				Name        string  `json:"name"`
				Price       float64 `json:"price"`
				Merchant    string  `json:"merchant"`
				ValidTo     string  `json:"valid_to,omitempty"`
				FlyerID     *int    `json:"flyer_id,omitempty"`
				ImageURL    string  `json:"image_url,omitempty"`
				DiscountPct *int    `json:"discount_pct,omitempty"`
			}
			type merchantBasket struct {
				Merchant     string       `json:"merchant"`
				Total        float64      `json:"total"`
				MatchedItems int          `json:"matched_items"`
				MissingItems []string     `json:"missing_items"`
				Items        []basketItem `json:"items"`
			}
			byMerchant := map[string]map[string]basketItem{}
			failures := []fetchFailure{}
			for _, query := range items {
				res, err := fetchFlippSearch(ctx, c, query, flagZip, flagLocale, "price_low_to_high")
				if err != nil {
					failures = append(failures, fetchFailure{Query: query, Error: err.Error()})
					continue
				}
				matches := append([]flippItem{}, res.Items...)
				matches = append(matches, res.EcomItems...)
				sortByPrice(matches)
				kept := 0
				for _, item := range matches {
					if item.CurrentPrice == nil {
						continue
					}
					if !matchesSearchIntent(item, query) {
						continue
					}
					merchant := merchantName(item)
					if byMerchant[merchant] == nil {
						byMerchant[merchant] = map[string]basketItem{}
					}
					if _, exists := byMerchant[merchant][query]; exists {
						continue
					}
					byMerchant[merchant][query] = basketItem{
						Query:       query,
						Name:        item.Name,
						Price:       *item.CurrentPrice,
						Merchant:    merchant,
						ValidTo:     item.ValidTo,
						FlyerID:     item.FlyerID,
						ImageURL:    itemImage(item),
						DiscountPct: discountPct(item),
					}
					kept++
					if kept >= flagLimit {
						break
					}
				}
			}
			baskets := make([]merchantBasket, 0, len(byMerchant))
			for merchant, found := range byMerchant {
				b := merchantBasket{Merchant: merchant, MissingItems: []string{}, Items: []basketItem{}}
				for _, query := range items {
					item, ok := found[query]
					if !ok {
						b.MissingItems = append(b.MissingItems, query)
						continue
					}
					b.Items = append(b.Items, item)
					b.Total += item.Price
					b.MatchedItems++
				}
				b.Total = math.Round(b.Total*100) / 100
				baskets = append(baskets, b)
			}
			sort.SliceStable(baskets, func(i, j int) bool {
				if baskets[i].MatchedItems != baskets[j].MatchedItems {
					return baskets[i].MatchedItems > baskets[j].MatchedItems
				}
				return baskets[i].Total < baskets[j].Total
			})
			view := struct {
				Zip           string           `json:"zip"`
				Items         []string         `json:"items"`
				Merchants     []merchantBasket `json:"merchants"`
				FetchFailures []fetchFailure   `json:"fetch_failures,omitempty"`
			}{Zip: flagZip, Items: items, Merchants: baskets, FetchFailures: failures}
			rows := [][]string{}
			for _, b := range baskets {
				rows = append(rows, []string{b.Merchant, strconv.Itoa(b.MatchedItems), fmt.Sprintf("%.2f", b.Total), strings.Join(b.MissingItems, ", ")})
			}
			return printRowsOrJSON(cmd, flags, view, []string{"Merchant", "Matched", "Total", "Missing"}, rows)
		},
	}
	cmd.Flags().StringVar(&flagItems, "items", "", "Comma-separated shopping-list items to compare")
	cmd.Flags().StringVar(&flagZip, "zip", "85001", "ZIP or postal code used to find local Flipp deals")
	cmd.Flags().StringVar(&flagLocale, "locale", defaultFlippLocale, "Flipp locale, such as en-us or en-ca")
	cmd.Flags().IntVar(&flagLimit, "limit-per-item", 8, "Maximum low-price matches to keep per basket item")
	return cmd
}
