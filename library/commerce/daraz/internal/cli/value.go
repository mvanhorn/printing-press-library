// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command. Safe to edit.
//
// pp:data-source live

package cli

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type valueRow struct {
	ItemID             string  `json:"itemId"`
	Name               string  `json:"name"`
	Price              float64 `json:"price"`
	OriginalPrice      float64 `json:"originalPrice"`
	DiscountPct        float64 `json:"discountPct"`
	Rating             float64 `json:"rating"`
	Sold               int     `json:"sold"`
	Seller             string  `json:"seller"`
	MarketMedian       float64 `json:"marketMedian"`
	PriceVsMedianPct   float64 `json:"priceVsMedianPct"`
	SuspiciousDiscount bool    `json:"suspiciousDiscount"`
	ValueScore         float64 `json:"valueScore"`
	URL                string  `json:"url"`
}

func newNovelValueCmd(flags *rootFlags) *cobra.Command {
	var limit, maxScan int
	var onlySuspicious bool

	cmd := &cobra.Command{
		Use:         "value <query>",
		Short:       "Flag inflated original-price discounts by comparing each item's claimed original price to the local median for that query.",
		Long:        "Flag inflated original-price discounts by comparing each item's claimed original price to the local median for that query, and rank items by a rating-weighted value score.\n\nUse this command to detect misleading discounts and rank true value. It is the inverse of 'deals': 'deals' ranks good deals, 'value' exposes bad ones.",
		Example:     "  daraz-pp-cli value \"power bank\" --agent\n  daraz-pp-cli value \"smart watch\" --only-suspicious",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search query is required, e.g. value \"power bank\""))
			}
			query := strings.Join(args, " ")
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			items, _, _, err := scanSearch(ctx, c, query, "", "", maxScan, 0)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if s, e := openDarazStore(ctx, flags); e == nil {
				recordProducts(ctx, s, items)
				_ = s.Close()
			}
			prices := make([]float64, 0, len(items))
			for _, p := range items {
				if p.InStock && p.priceF() > 0 {
					prices = append(prices, p.priceF())
				}
			}
			median := medianFloat(prices)
			rows := make([]valueRow, 0, len(items))
			for _, p := range items {
				if !p.InStock || p.priceF() <= 0 {
					continue
				}
				price := p.priceF()
				vsMedian := 0.0
				if median > 0 {
					vsMedian = math.Round((price-median)/median*1000) / 10
				}
				// A discount is suspicious when a steep markdown is measured
				// against an "original" price well above the market median for
				// the same query — the classic inflated-original trick.
				suspicious := p.discountPct() >= 50 && median > 0 && p.origF() > median*2.0
				value := 0.0
				if median > 0 && price > 0 {
					value = math.Round((median/price)*(0.5+p.ratingF()/10)*100) / 100
				}
				if onlySuspicious && !suspicious {
					continue
				}
				rows = append(rows, valueRow{
					ItemID: p.ItemID, Name: p.Name, Price: price, OriginalPrice: p.origF(),
					DiscountPct: p.discountPct(), Rating: p.ratingF(), Sold: p.soldN(), Seller: p.SellerName,
					MarketMedian: median, PriceVsMedianPct: vsMedian, SuspiciousDiscount: suspicious,
					ValueScore: value, URL: p.fullURL(),
				})
			}
			sort.SliceStable(rows, func(i, j int) bool { return rows[i].ValueScore > rows[j].ValueScore })
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			return emitDaraz(cmd, flags, rows)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 15, "maximum items to return")
	cmd.Flags().IntVar(&maxScan, "max-scan-pages", 3, "maximum search pages to scan (40 items per page)")
	cmd.Flags().BoolVar(&onlySuspicious, "only-suspicious", false, "only show items with a suspicious (likely inflated) discount")
	return cmd
}
