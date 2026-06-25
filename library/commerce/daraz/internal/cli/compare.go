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

type itemBrief struct {
	ItemID  string  `json:"itemId"`
	Name    string  `json:"name"`
	Price   float64 `json:"price"`
	Rating  float64 `json:"rating"`
	Reviews int     `json:"reviews"`
	Sold    int     `json:"sold"`
	Seller  string  `json:"seller"`
	URL     string  `json:"url"`
}

type sellerAgg struct {
	Seller    string  `json:"seller"`
	SellerID  string  `json:"sellerId"`
	Count     int     `json:"count"`
	MinPrice  float64 `json:"minPrice"`
	AvgRating float64 `json:"avgRating"`
}

type compareOut struct {
	Query     string      `json:"query"`
	Scanned   int         `json:"scanned"`
	Cheapest  *itemBrief  `json:"cheapest"`
	BestRated *itemBrief  `json:"bestRated"`
	Sellers   []sellerAgg `json:"sellers"`
}

func briefOf(p darazProduct) *itemBrief {
	return &itemBrief{
		ItemID: p.ItemID, Name: p.Name, Price: p.priceF(), Rating: p.ratingF(),
		Reviews: p.reviewN(), Sold: p.soldN(), Seller: p.SellerName, URL: p.fullURL(),
	}
}

func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	var maxScan, topSellers int

	cmd := &cobra.Command{
		Use:         "compare <query>",
		Short:       "Find the same item across sellers and show the cheapest and best-rated side by side.",
		Long:        "Find the same item across sellers and show the cheapest and best-rated listings side by side, plus a per-seller breakdown.\n\nUse this command to pick the best seller/listing for an item. For a single known product, use 'products get' instead.",
		Example:     "  daraz-pp-cli compare \"airpods pro\" --agent",
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
				return usageErr(fmt.Errorf("a search query is required, e.g. compare \"airpods pro\""))
			}
			query := strings.Join(args, " ")
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			items, _, scanned, err := scanSearch(ctx, c, query, "", "", maxScan, 0)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if s, e := openDarazStore(ctx, flags); e == nil {
				recordProducts(ctx, s, items)
				_ = s.Close()
			}
			out := compareOut{Query: query, Scanned: scanned, Sellers: []sellerAgg{}}
			type acc struct {
				name     string
				id       string
				count    int
				minPrice float64
				ratSum   float64
				ratN     int
			}
			bySeller := map[string]*acc{}
			for _, p := range items {
				if !p.InStock || p.priceF() <= 0 {
					continue
				}
				if out.Cheapest == nil || p.priceF() < out.Cheapest.Price {
					out.Cheapest = briefOf(p)
				}
				// best-rated requires a minimum of 5 reviews to avoid a single
				// 5-star review winning over a well-reviewed 4.8.
				if p.reviewN() >= 5 {
					if out.BestRated == nil || p.ratingF() > out.BestRated.Rating ||
						(p.ratingF() == out.BestRated.Rating && p.reviewN() > out.BestRated.Reviews) {
						out.BestRated = briefOf(p)
					}
				}
				key := p.SellerID
				if key == "" {
					key = p.SellerName
				}
				a := bySeller[key]
				if a == nil {
					a = &acc{name: p.SellerName, id: p.SellerID, minPrice: p.priceF()}
					bySeller[key] = a
				}
				a.count++
				if p.priceF() < a.minPrice {
					a.minPrice = p.priceF()
				}
				if p.ratingF() > 0 {
					a.ratSum += p.ratingF()
					a.ratN++
				}
			}
			if out.BestRated == nil {
				out.BestRated = out.Cheapest // fall back when nothing clears the review floor
			}
			for _, a := range bySeller {
				avg := 0.0
				if a.ratN > 0 {
					avg = math.Round(a.ratSum/float64(a.ratN)*100) / 100
				}
				out.Sellers = append(out.Sellers, sellerAgg{
					Seller: a.name, SellerID: a.id, Count: a.count,
					MinPrice: a.minPrice, AvgRating: avg,
				})
			}
			sort.SliceStable(out.Sellers, func(i, j int) bool { return out.Sellers[i].Count > out.Sellers[j].Count })
			if topSellers > 0 && len(out.Sellers) > topSellers {
				out.Sellers = out.Sellers[:topSellers]
			}
			return emitDaraz(cmd, flags, out)
		},
	}
	cmd.Flags().IntVar(&maxScan, "max-scan-pages", 3, "maximum search pages to scan (40 items per page)")
	cmd.Flags().IntVar(&topSellers, "top-sellers", 10, "maximum sellers to include in the breakdown")
	return cmd
}
