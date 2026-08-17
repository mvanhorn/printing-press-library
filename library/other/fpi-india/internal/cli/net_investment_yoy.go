// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local

package cli

import (
	"fmt"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/other/fpi-india/internal/nsdl"

	"github.com/spf13/cobra"
)

type netInvestmentYoyView struct {
	Asset        string  `json:"asset"`
	CurrentYear  string  `json:"current_year"`
	CurrentValue float64 `json:"current_value"`
	PriorYear    string  `json:"prior_year"`
	PriorValue   float64 `json:"prior_value"`
	Delta        float64 `json:"delta"`
	DeltaPct     float64 `json:"delta_pct,omitempty"`
	Note         string  `json:"note,omitempty"`
}

func newNovelNetInvestmentYoyCmd(flags *rootFlags) *cobra.Command {
	var flagAsset string
	var flagYear string

	cmd := &cobra.Command{
		Use:         "yoy",
		Short:       "See this year's FPI net flow next to the same period last year, delta computed automatically.",
		Example:     "  fpi-india-pp-cli net-investment yoy --asset equity --year 2024 --json",
		Long:        "Compares the requested financial year's net investment against the immediately preceding financial year for one asset class. Requires 'sync --resources net_investment' first; use 'net-investment fy' for the raw historical series instead of a computed delta.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute year-over-year net-investment delta")
				return nil
			}
			asset := flagAsset
			if asset == "" {
				asset = "total"
			}
			if flagYear == "" {
				_ = cmd.Usage()
				return usageErrf("--year is required, e.g. --year 2024")
			}
			yearNum, err := strconv.Atoi(flagYear)
			if err != nil {
				_ = cmd.Usage()
				return usageErrf("--year must be a 4-digit calendar year, e.g. 2024")
			}
			currentFY := fmt.Sprintf("%d-%02d", yearNum, (yearNum+1)%100)
			priorFY := fmt.Sprintf("%d-%02d", yearNum-1, yearNum%100)

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

			rows, err := loadNetInvestmentRows(db, "fy", "INR")
			if err != nil {
				return fmt.Errorf("reading synced net-investment data: %w", err)
			}

			var current, prior *nsdl.NetInvestmentRow
			for i := range rows {
				if rows[i].Period == currentFY {
					current = &rows[i]
				}
				if rows[i].Period == priorFY {
					prior = &rows[i]
				}
			}
			if current == nil || prior == nil {
				view := netInvestmentYoyView{Asset: asset, CurrentYear: currentFY, PriorYear: priorFY,
					Note: fmt.Sprintf("missing synced data for %s and/or %s; run sync --resources net_investment --full", currentFY, priorFY)}
				return printOutputWithFlagsMeta(cmd.OutOrStdout(), mustJSON(view), flags, map[string]any{"source": "local"})
			}

			curVal, ok1 := current.AssetValue(asset)
			priorVal, ok2 := prior.AssetValue(asset)
			if !ok1 || !ok2 {
				_ = cmd.Usage()
				return usageErrf("--asset must be one of equity, debt, hybrid, mutual_funds, aif, total")
			}

			view := netInvestmentYoyView{
				Asset:        asset,
				CurrentYear:  currentFY,
				CurrentValue: curVal,
				PriorYear:    priorFY,
				PriorValue:   priorVal,
				Delta:        curVal - priorVal,
			}
			if priorVal != 0 {
				view.DeltaPct = ((curVal - priorVal) / abs(priorVal)) * 100
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printOutputWithFlagsMeta(cmd.OutOrStdout(), mustJSON(view), flags, map[string]any{"source": "local"})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s=%.0f  %s=%.0f  delta=%.0f (%.1f%%)\n",
				asset, view.CurrentYear, view.CurrentValue, view.PriorYear, view.PriorValue, view.Delta, view.DeltaPct)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagAsset, "asset", "", "Asset class: equity, debt, hybrid, mutual_funds, aif, or total (default total)")
	cmd.Flags().StringVar(&flagYear, "year", "", "Calendar year whose financial year (year -> year+1) to compare against the prior FY")
	return cmd
}
