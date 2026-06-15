// Copyright 2026 Dev Basu and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored transcendence command (Phase 3). Safe to edit.
// pp:data-source live

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type compareRow struct {
	Shop            string  `json:"shop"`
	Name            string  `json:"name"`
	AvgServiceCents int     `json:"avg_service_cost_cents"`
	MinServiceCents int     `json:"min_service_cost_cents"`
	Rating          float64 `json:"rating"`
	NumRatings      int     `json:"num_ratings"`
	BarberCount     int     `json:"barber_count"`
	BookingFeeCents int     `json:"booking_fee_cents"`
	ServiceCount    int     `json:"service_count"`
}

type compareResult struct {
	Shops         []compareRow   `json:"shops"`
	FetchFailures []fetchFailure `json:"fetch_failures"`
}

func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compare <shopA> <shopB> [shopC...]",
		Short: "Put two or more named shops side by side on average price, rating, review count, and staff size.",
		Example: strings.Trim(`
  squire-pp-cli compare barber-theory-toronto another-shop-route
  squire-pp-cli compare barber-theory-toronto another-shop third-shop --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would compare %d shop(s)\n", len(args))
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("provide at least two shops to compare"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			rows := make([]compareRow, 0)
			failures := make([]fetchFailure, 0)
			for _, shop := range args {
				uuid, name, _, barberCount, bookingFee, _, err := resolveShop(ctx, c, shop)
				if err != nil {
					failures = append(failures, fetchFailure{Shop: shop, Error: err.Error()})
					continue
				}
				row := compareRow{Shop: shop, Name: name, BarberCount: barberCount, BookingFeeCents: bookingFee}
				if svcs, err := fetchServices(ctx, c, uuid); err == nil {
					total, n, min := 0, 0, 0
					for _, s := range svcs {
						if b, _ := s["addonOnly"].(bool); b {
							continue
						}
						cost := sqInt(s, "cost")
						total += cost
						n++
						if min == 0 || cost < min {
							min = cost
						}
					}
					row.ServiceCount = n
					row.MinServiceCents = min
					if n > 0 {
						row.AvgServiceCents = total / n
					}
				}
				if avg, num, _, err := fetchReviewMeta(ctx, c, uuid); err == nil {
					row.Rating = avg
					row.NumRatings = num
				}
				rows = append(rows, row)
			}
			if len(rows) == 0 {
				return apiErr(fmt.Errorf("all %d shop fetches failed", len(args)))
			}
			return printJSONFiltered(cmd.OutOrStdout(), compareResult{Shops: rows, FetchFailures: failures}, flags)
		},
	}
	return cmd
}
