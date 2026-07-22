// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local

package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

type netInvestmentTrendPoint struct {
	Period string  `json:"period"`
	Value  float64 `json:"value"`
}

type netInvestmentTrendView struct {
	Asset         string                    `json:"asset"`
	Period        string                    `json:"period_type"`
	Window        int                       `json:"window"`
	Points        []netInvestmentTrendPoint `json:"points"`
	Direction     string                    `json:"direction"` // rising | falling | flat
	GrowthRatePct float64                   `json:"growth_rate_pct,omitempty"`
	Note          string                    `json:"note,omitempty"`
}

func newNovelNetInvestmentTrendCmd(flags *rootFlags) *cobra.Command {
	var flagAsset string
	var flagPeriod string
	var flagWindow string

	cmd := &cobra.Command{
		Use:         "trend",
		Short:       "See the rolling growth-rate and direction of FPI net flow over a window of recent periods.",
		Example:     "  fpi-india-pp-cli net-investment trend --asset equity --period fy --window 12 --json",
		Long:        "Computes the growth rate and direction of one asset class's net investment across the last N synced periods (financial or calendar year). Requires 'sync --resources net_investment' first; use 'net-investment yoy' for a simple single-year comparison instead.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute rolling net-investment trend")
				return nil
			}
			asset := flagAsset
			if asset == "" {
				asset = "total"
			}
			periodType := flagPeriod
			if periodType == "" {
				periodType = "fy"
			}
			if periodType != "fy" && periodType != "cy" {
				_ = cmd.Usage()
				return usageErrf("--period must be fy or cy")
			}
			window := 12
			if flagWindow != "" {
				w, err := strconv.Atoi(flagWindow)
				if err != nil || w < 2 {
					_ = cmd.Usage()
					return usageErrf("--window must be an integer >= 2")
				}
				window = w
			}

			db, exists, err := openLocalStore(cmd.Context())
			if err != nil {
				return err
			}
			if !exists {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: fpi-india-pp-cli sync --resources net_investment\n", defaultDBPath("fpi-india-pp-cli"))
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}
			defer db.Close()

			rows, err := loadNetInvestmentRows(db, periodType, "INR")
			if err != nil {
				return fmt.Errorf("reading synced net-investment data: %w", err)
			}
			if len(rows) == 0 {
				view := netInvestmentTrendView{Asset: asset, Period: periodType, Window: window,
					Note: "no synced net-investment rows; run sync --resources net_investment --full"}
				return printOutputWithFlagsMeta(cmd.OutOrStdout(), mustJSON(view), flags, map[string]any{"source": "local"})
			}

			start := len(rows) - window
			if start < 0 {
				start = 0
			}
			windowed := rows[start:]

			view := netInvestmentTrendView{Asset: asset, Period: periodType, Window: len(windowed)}
			for _, r := range windowed {
				v, ok := r.AssetValue(asset)
				if !ok {
					_ = cmd.Usage()
					return usageErrf("--asset must be one of equity, debt, hybrid, mutual_funds, aif, total")
				}
				view.Points = append(view.Points, netInvestmentTrendPoint{Period: r.Period, Value: v})
			}
			if len(view.Points) >= 2 {
				first := view.Points[0].Value
				last := view.Points[len(view.Points)-1].Value
				switch {
				case last > first:
					view.Direction = "rising"
				case last < first:
					view.Direction = "falling"
				default:
					view.Direction = "flat"
				}
				if first != 0 {
					view.GrowthRatePct = ((last - first) / abs(first)) * 100
				}
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printOutputWithFlagsMeta(cmd.OutOrStdout(), mustJSON(view), flags, map[string]any{"source": "local"})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s (%s, last %d periods): %s, %.1f%% change\n",
				asset, periodType, len(view.Points), view.Direction, view.GrowthRatePct)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagAsset, "asset", "", "Asset class: equity, debt, hybrid, mutual_funds, aif, or total (default total)")
	cmd.Flags().StringVar(&flagPeriod, "period", "", "Period granularity: fy or cy (default fy)")
	cmd.Flags().StringVar(&flagWindow, "window", "", "Number of most-recent periods to include (default 12)")
	return cmd
}
