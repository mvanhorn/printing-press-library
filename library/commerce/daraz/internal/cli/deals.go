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

type dealRow struct {
	ItemID        string  `json:"itemId"`
	Name          string  `json:"name"`
	Price         float64 `json:"price"`
	OriginalPrice float64 `json:"originalPrice"`
	DiscountPct   float64 `json:"discountPct"`
	Rating        float64 `json:"rating"`
	Reviews       int     `json:"reviews"`
	Sold          int     `json:"sold"`
	Seller        string  `json:"seller"`
	DealScore     float64 `json:"dealScore"`
	URL           string  `json:"url"`
}

func newNovelDealsCmd(flags *rootFlags) *cobra.Command {
	var limit, maxScan int
	var minRating float64

	cmd := &cobra.Command{
		Use:         "deals <query>",
		Short:       "Rank a search by a composite of discount, rating, and units sold to surface genuinely good deals.",
		Long:        "Rank a search by a composite of discount, rating, and units sold to surface genuinely good deals.\n\nUse this command to find the best-value items for a search. Do NOT use it for raw listing order; use 'products --sort' instead.",
		Example:     "  daraz-pp-cli deals \"power bank\" --agent\n  daraz-pp-cli deals \"gaming laptop\" --min-rating 4 --limit 10",
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
				return usageErr(fmt.Errorf("a search query is required, e.g. deals \"power bank\""))
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
			rows := make([]dealRow, 0, len(items))
			for _, p := range items {
				if !p.InStock {
					continue
				}
				if minRating > 0 && p.ratingF() < minRating {
					continue
				}
				score := math.Round(dealScore(p.discountPct(), p.ratingF(), p.soldN())*100) / 100
				rows = append(rows, dealRow{
					ItemID: p.ItemID, Name: p.Name, Price: p.priceF(), OriginalPrice: p.origF(),
					DiscountPct: p.discountPct(), Rating: p.ratingF(), Reviews: p.reviewN(), Sold: p.soldN(),
					Seller: p.SellerName, DealScore: score, URL: p.fullURL(),
				})
			}
			sort.SliceStable(rows, func(i, j int) bool { return rows[i].DealScore > rows[j].DealScore })
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			return emitDaraz(cmd, flags, rows)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 15, "maximum deals to return")
	cmd.Flags().IntVar(&maxScan, "max-scan-pages", 3, "maximum search pages to scan (40 items per page)")
	cmd.Flags().Float64Var(&minRating, "min-rating", 0, "only include items rated at least this (0-5)")
	return cmd
}
