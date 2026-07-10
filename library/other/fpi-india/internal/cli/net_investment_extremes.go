// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/spf13/cobra"
)

type netInvestmentExtreme struct {
	Period string  `json:"period"`
	Value  float64 `json:"value"`
	Kind   string  `json:"kind"` // inflow | outflow
}

type netInvestmentExtremesView struct {
	Asset    string                 `json:"asset"`
	Extremes []netInvestmentExtreme `json:"extremes"`
	Note     string                 `json:"note,omitempty"`
}

func newNovelNetInvestmentExtremesCmd(flags *rootFlags) *cobra.Command {
	var flagAsset string
	var flagTop string

	cmd := &cobra.Command{
		Use:         "extremes",
		Short:       "Find the largest single-period FPI inflows or outflows across the full 1992-93-to-present series.",
		Example:     "  fpi-india-pp-cli net-investment extremes --asset equity --top 10 --json",
		Long:        "Ranks every synced financial-year period by absolute magnitude of net investment for one asset class, returning the largest inflows and outflows. Requires 'sync --resources net_investment' first.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank historical net-investment extremes")
				return nil
			}
			asset := flagAsset
			if asset == "" {
				asset = "total"
			}
			top := 10
			if flagTop != "" {
				n, err := strconv.Atoi(flagTop)
				if err != nil || n < 1 {
					_ = cmd.Usage()
					return usageErrf("--top must be a positive integer")
				}
				top = n
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

			rows, err := loadNetInvestmentRows(db, "fy", "INR")
			if err != nil {
				return fmt.Errorf("reading synced net-investment data: %w", err)
			}
			if len(rows) == 0 {
				view := netInvestmentExtremesView{Asset: asset, Note: "no synced net-investment rows; run sync --resources net_investment --full"}
				return printOutputWithFlagsMeta(cmd.OutOrStdout(), mustJSON(view), flags, map[string]any{"source": "local"})
			}

			var extremes []netInvestmentExtreme
			for _, r := range rows {
				v, ok := r.AssetValue(asset)
				if !ok {
					_ = cmd.Usage()
					return usageErrf("--asset must be one of equity, debt, hybrid, mutual_funds, aif, total")
				}
				if v == 0 {
					continue
				}
				kind := "inflow"
				if v < 0 {
					kind = "outflow"
				}
				extremes = append(extremes, netInvestmentExtreme{Period: r.Period, Value: v, Kind: kind})
			}
			sort.Slice(extremes, func(i, j int) bool { return abs(extremes[i].Value) > abs(extremes[j].Value) })
			if len(extremes) > top {
				extremes = extremes[:top]
			}

			view := netInvestmentExtremesView{Asset: asset, Extremes: extremes}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printOutputWithFlagsMeta(cmd.OutOrStdout(), mustJSON(view), flags, map[string]any{"source": "local"})
			}
			for _, e := range view.Extremes {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%.0f\n", e.Period, e.Kind, e.Value)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagAsset, "asset", "", "Asset class: equity, debt, hybrid, mutual_funds, aif, or total (default total)")
	cmd.Flags().StringVar(&flagTop, "top", "", "Number of extremes to return (default 10)")
	return cmd
}
