// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/passive-indices/internal/indiapassivefunds"
)

func newFundScreenCmd(flags *rootFlags) *cobra.Command {
	var amc, underlyingIndex, sortBy, sortOrder string
	var limit int
	var etfOnly, indexFundOnly bool

	cmd := &cobra.Command{
		Use:     "screen",
		Short:   "Screen every published ETF and index fund with combined overview, returns, and risk data (benchmark category, tracking error/difference, AUM, TER, 1Y/3Y/5Y/since-inception returns, std deviation, Sharpe, beta), optionally narrowed by AMC/underlying index/fund type",
		Example: "  passive-indices-pp-cli fund screen --amc \"HDFC\" --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would screen funds")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c := newIndiaPassiveFundsClient(flags)
			p := indiapassivefunds.ScreenParams{
				SortBy:    sortBy,
				SortOrder: sortOrder,
			}
			if etfOnly {
				p.FundTypeID = 3
			} else if indexFundOnly {
				p.FundTypeID = 9
			}

			var filters *indiapassivefunds.ScreenerFilterTaxonomy
			if amc != "" || underlyingIndex != "" {
				var err error
				filters, err = c.ScreenerFilters(ctx)
				if err != nil {
					return classifyAPIError(err, flags)
				}
			}
			if amc != "" {
				val, matched, found := indiapassivefunds.FindAMCValue(filters, amc)
				if !found {
					return usageErr(fmt.Errorf("no AMC filter matched %q", amc))
				}
				p.AMC = val
				fmt.Fprintf(cmd.ErrOrStderr(), "matched AMC: %s\n", matched)
			}
			if underlyingIndex != "" {
				val, matched, found := indiapassivefunds.FindUnderlyingIndexValue(filters, underlyingIndex)
				if !found {
					return usageErr(fmt.Errorf("no underlying-index filter matched %q; try the exact niftyindices name (e.g. \"NIFTY 50\")", underlyingIndex))
				}
				p.UnderlyingIndex = val
				fmt.Fprintf(cmd.ErrOrStderr(), "matched underlying index: %s\n", matched)
			}

			rows, err := c.ScreenAll(ctx, p)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			return flags.printJSON(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&amc, "amc", "", "filter by AMC name")
	cmd.Flags().StringVar(&underlyingIndex, "underlying-index", "", "filter by tracked index name, e.g. \"NIFTY 50\"")
	cmd.Flags().BoolVar(&etfOnly, "etf-only", false, "only ETFs")
	cmd.Flags().BoolVar(&indexFundOnly, "index-fund-only", false, "only index funds")
	cmd.Flags().StringVar(&sortBy, "sort-by", "", "sort field")
	cmd.Flags().StringVar(&sortOrder, "sort-order", "", "sort order (asc|desc)")
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum results to return (0 = all matching funds)")
	return cmd
}
