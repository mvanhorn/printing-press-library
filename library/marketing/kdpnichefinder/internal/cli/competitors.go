// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// pp:data-source local

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelCompetitorsCmd(flags *rootFlags) *cobra.Command {
	var flagDB string

	cmd := &cobra.Command{
		Use:     "competitors <book-id>",
		Short:   "For a focus book, list same-publisher and same-price-band competitors using the extracted ASIN.",
		Example: "  kdpnichefinder-pp-cli competitors 2584",
		Long: "Use to inspect one book's competitors (same publisher / price band). " +
			"For whole-bucket concentration, use 'saturation'.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("book id or ASIN is required\nUsage: %s <book-id>", cmd.CommandPath()))
			}
			key := args[0]

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, _, missing, err := openKDPLocal(ctx, flags, flagDB, cmd.OutOrStdout())
			if err != nil {
				return err
			}
			if missing {
				return nil
			}
			defer db.Close()

			niches, err := loadNiches(ctx, db, "")
			if err != nil {
				return err
			}

			var focus *nicheRow
			for i := range niches {
				if niches[i].ID == key || (niches[i].ASIN != "" && strings.EqualFold(niches[i].ASIN, key)) {
					focus = &niches[i]
					break
				}
			}
			if focus == nil {
				return notFoundErr(fmt.Errorf("no book with id or ASIN %q in the local mirror", key))
			}

			lo := focus.Price * 0.75
			hi := focus.Price * 1.25

			type compRow struct {
				ID                      string  `json:"id"`
				Title                   string  `json:"title"`
				Publisher               string  `json:"publisher"`
				Price                   string  `json:"price"`
				EstimatedMonthlyRevenue float64 `json:"estimated_monthly_revenue"`
				ASIN                    string  `json:"asin"`
				MatchReason             string  `json:"match_reason"`
			}
			comps := make([]compRow, 0)
			for i := range niches {
				n := niches[i]
				if n.ID == focus.ID {
					continue
				}
				samePub := focus.Publisher != "" && n.Publisher == focus.Publisher
				priceBand := focus.Price > 0 && n.Price >= lo && n.Price <= hi
				if !samePub && !priceBand {
					continue
				}
				reasons := make([]string, 0, 2)
				if samePub {
					reasons = append(reasons, "same_publisher")
				}
				if priceBand {
					reasons = append(reasons, "price_band")
				}
				comps = append(comps, compRow{
					ID:                      n.ID,
					Title:                   n.Title,
					Publisher:               n.Publisher,
					Price:                   n.PriceStr,
					EstimatedMonthlyRevenue: n.Revenue,
					ASIN:                    n.ASIN,
					MatchReason:             strings.Join(reasons, "+"),
				})
			}
			sort.SliceStable(comps, func(i, j int) bool {
				return comps[i].EstimatedMonthlyRevenue > comps[j].EstimatedMonthlyRevenue
			})

			result := map[string]any{
				"focus": map[string]any{
					"id":                        focus.ID,
					"title":                     focus.Title,
					"publisher":                 focus.Publisher,
					"price":                     focus.PriceStr,
					"asin":                      focus.ASIN,
					"bucket":                    focus.Bucket,
					"estimated_monthly_revenue": focus.Revenue,
				},
				"competitors": comps,
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local mirror database (defaults to the standard location)")
	return cmd
}
