// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command. Safe to edit.
//
// pp:data-source local

package cli

import (
	"database/sql"
	"fmt"
	"math"
	"sort"

	"github.com/spf13/cobra"
)

type sellerStatsOut struct {
	SellerID       string   `json:"sellerId"`
	SellerName     string   `json:"sellerName"`
	Listings       int      `json:"listings"`
	AvgRating      float64  `json:"avgRating"`
	TotalReviews   int      `json:"totalReviews"`
	MinPrice       float64  `json:"minPrice"`
	MaxPrice       float64  `json:"maxPrice"`
	AvgDiscountPct float64  `json:"avgDiscountPct"`
	TopBrands      []string `json:"topBrands"`
}

func newNovelSellerStatsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "stats <sellerId>",
		Short:       "Aggregate a seller's catalog from the local store: average rating, price range, listing count, discount pattern.",
		Long:        "Aggregate a seller's catalog from the local store: average rating, total reviews, price range, average discount, and top brands.\n\nUse this command to vet a seller's track record before trusting a listing. For their raw listings, use 'seller listings'. The local store is populated by deals/value/compare/watch/since runs.",
		Example:     "  daraz-pp-cli seller stats 1066739 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a seller ID is required, e.g. seller stats 1066739"))
			}
			sellerID := args[0]
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			s, err := openDarazStore(ctx, flags)
			if err != nil {
				return err
			}
			defer s.Close()

			rows, err := s.DB().QueryContext(ctx, `
				SELECT seller_name, price, original_price, discount_pct, rating, review_count, brand
				FROM daraz_products_seen WHERE seller_id=?`, sellerID)
			if err != nil {
				return fmt.Errorf("reading seller stats: %w", err)
			}
			defer rows.Close()

			out := sellerStatsOut{SellerID: sellerID, TopBrands: []string{}}
			var ratSum float64
			var ratN int
			var discSum float64
			var discN int
			brandCount := map[string]int{}
			first := true
			for rows.Next() {
				var sellerName, brand sql.NullString
				var price, origPrice, disc, rating sql.NullFloat64
				var reviews sql.NullInt64
				if err := rows.Scan(&sellerName, &price, &origPrice, &disc, &rating, &reviews, &brand); err != nil {
					continue
				}
				out.Listings++
				if sellerName.String != "" {
					out.SellerName = sellerName.String
				}
				p := price.Float64
				if first || p < out.MinPrice {
					out.MinPrice = p
				}
				if first || p > out.MaxPrice {
					out.MaxPrice = p
				}
				first = false
				if rating.Float64 > 0 {
					ratSum += rating.Float64
					ratN++
				}
				if disc.Float64 > 0 {
					discSum += disc.Float64
					discN++
				}
				out.TotalReviews += int(reviews.Int64)
				if brand.String != "" {
					brandCount[brand.String]++
				}
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading seller stats: %w", err)
			}
			if out.Listings == 0 {
				return emptyMirrorHint(cmd, flags, fmt.Sprintf("no local listings for seller %s yet. Run deals/value/compare/watch on relevant queries first, then retry.", sellerID))
			}
			if ratN > 0 {
				out.AvgRating = math.Round(ratSum/float64(ratN)*100) / 100
			}
			if discN > 0 {
				out.AvgDiscountPct = math.Round(discSum/float64(discN)*10) / 10
			}
			type bc struct {
				brand string
				n     int
			}
			brands := make([]bc, 0, len(brandCount))
			for b, n := range brandCount {
				brands = append(brands, bc{b, n})
			}
			sort.SliceStable(brands, func(i, j int) bool { return brands[i].n > brands[j].n })
			for i, b := range brands {
				if i >= 5 {
					break
				}
				out.TopBrands = append(out.TopBrands, b.brand)
			}
			return emitDaraz(cmd, flags, out)
		},
	}
	return cmd
}
