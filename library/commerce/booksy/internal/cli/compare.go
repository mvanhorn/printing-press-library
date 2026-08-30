// Copyright 2026 Max Tomago and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: compare — side-by-side barbers on rating + cheapest service.

package cli

import (
	"fmt"
	"sync"

	"github.com/spf13/cobra"
)

type bkCompareRow struct {
	BusinessID   string  `json:"business_id"`
	Name         string  `json:"name"`
	Rating       float64 `json:"rating"`
	ReviewsCount int     `json:"reviews_count"`
	Service      string  `json:"service,omitempty"`
	Price        float64 `json:"price,omitempty"`
	PriceLabel   string  `json:"price_label,omitempty"`
	VariantID    int64   `json:"service_variant_id,omitempty"`
}

type bkFetchFailure struct {
	BusinessID string `json:"business_id"`
	Error      string `json:"error"`
}

func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	var flagService string

	cmd := &cobra.Command{
		Use:   "compare <business_id> <business_id> [business_id...]",
		Short: "Compare several businesses side by side on rating, reviews, and cheapest matching service price",
		Long: "Compare two or more Booksy businesses at once on rating, review count, and\n" +
			"(with --service) the cheapest service matching your query. Public — no token.",
		Example:     "  booksy-pp-cli compare 297360 161624 --service haircut",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "a=297360;b=161624"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "compare")
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("provide at least two business ids to compare"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type result struct {
				idx int
				row bkCompareRow
				err error
			}
			results := make(chan result, len(args))
			var wg sync.WaitGroup
			for idx, id := range args {
				wg.Add(1)
				go func(idx int, id string) {
					defer wg.Done()
					biz, ferr := fetchBusiness(ctx, c, id)
					if ferr != nil {
						results <- result{idx: idx, err: ferr}
						return
					}
					row := bkCompareRow{
						BusinessID:   id,
						Name:         biz.Name,
						Rating:       biz.ReviewsRank,
						ReviewsCount: biz.ReviewsCount,
					}
					if flagService != "" {
						if best := cheapestMatching(biz, flagService); best != nil {
							row.Service = best.Service
							row.Price = best.Price
							row.PriceLabel = best.PriceLabel
							row.VariantID = best.VariantID
						}
					}
					results <- result{idx: idx, row: row}
				}(idx, id)
			}
			go func() { wg.Wait(); close(results) }()

			ordered := make([]bkCompareRow, len(args))
			failures := make([]bkFetchFailure, 0)
			for r := range results {
				if r.err != nil {
					failures = append(failures, bkFetchFailure{BusinessID: args[r.idx], Error: r.err.Error()})
					continue
				}
				ordered[r.idx] = r.row
			}
			rows := make([]bkCompareRow, 0, len(args))
			for _, row := range ordered {
				if row.BusinessID != "" {
					rows = append(rows, row)
				}
			}
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d businesses failed to load; comparing the remaining %d\n", len(failures), len(args), len(rows))
			}

			view := struct {
				Service       string           `json:"service,omitempty"`
				Count         int              `json:"count"`
				Businesses    []bkCompareRow   `json:"businesses"`
				FetchFailures []bkFetchFailure `json:"fetch_failures,omitempty"`
			}{Service: flagService, Count: len(rows), Businesses: rows, FetchFailures: failures}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			out := cmd.OutOrStdout()
			if len(rows) == 0 {
				fmt.Fprintln(out, "No businesses could be compared.")
				return nil
			}
			fmt.Fprintf(out, "%-9s  %-34s  %6s  %7s  %12s\n", "ID", "NAME", "RATING", "REVIEWS", "PRICE")
			for _, r := range rows {
				name := r.Name
				if len([]rune(name)) > 34 {
					name = string([]rune(name)[:33]) + "…"
				}
				price := r.PriceLabel
				if price == "" {
					price = "-"
				}
				fmt.Fprintf(out, "%-9s  %-34s  %6.2f  %7d  %12s\n", r.BusinessID, name, r.Rating, r.ReviewsCount, price)
			}
			if len(failures) > 0 {
				fmt.Fprintf(out, "\npartial: %d of %d failed to load\n", len(failures), len(args))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagService, "service", "", "Match a service to compare cheapest price (e.g. haircut)")
	return cmd
}
