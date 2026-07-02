// Copyright 2026 megumikuo and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: value a points/miles balance using TPG's monthly valuation.
package cli

import (
	"fmt"
	"math"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source auto
func newNovelWorthCmd(flags *rootFlags) *cobra.Command {
	var program string
	var points float64

	cmd := &cobra.Command{
		Use:   "worth",
		Short: "Estimate the dollar value of a points/miles balance using TPG's valuation",
		Long: strings.TrimSpace(`
Convert a points or miles balance into an estimated dollar value using The
Points Guy's current cents-per-point valuation for the program. For a single
booking decision (points vs cash), use 'redeem-check'. For many programs at
once, use 'portfolio'.`),
		Example: strings.Trim(`
  thepointsguy-pp-cli worth --program "American AAdvantage" --points 75000
  thepointsguy-pp-cli worth --program "Chase Ultimate Rewards" --points 100000 --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would value a points balance")
				return nil
			}
			if program == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--program is required"))
			}
			if points <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--points must be a positive number"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := &tpgClientCtx{client: newTPGClient(flags), ctx: ctx}
			byProg, month, err := currentValuations(cmd, flags, c)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			v, cands, ok := resolveValuation(byProg, program)
			if !ok {
				if flags.asJSON || flags.agent {
					_ = emitJSON(cmd, flags, map[string]any{"error": "program not found", "program": program, "candidates": cands})
				}
				return notFoundErr(fmt.Errorf("no valuation for %q; did you mean one of: %s", program, strings.Join(cands, ", ")))
			}
			value := dollarsFromPoints(points, v.CentsPerPoint)
			view := struct {
				Program       string  `json:"program"`
				Type          string  `json:"type"`
				Points        float64 `json:"points"`
				CentsPerPoint float64 `json:"cents_per_point"`
				ValueUSD      float64 `json:"value_usd"`
				Month         string  `json:"month"`
			}{v.Program, v.Type, points, v.CentsPerPoint, round2(value), month}

			if flags.asJSON || flags.agent {
				return emitJSON(cmd, flags, view)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%.0f %s points ≈ $%.2f (at %.2f¢/point, TPG %s)\n",
				points, v.Program, value, v.CentsPerPoint, month)
			return nil
		},
	}
	cmd.Flags().StringVar(&program, "program", "", "Loyalty program name, e.g. \"American AAdvantage\" (fuzzy match)")
	cmd.Flags().Float64Var(&points, "points", 0, "Number of points/miles to value")
	return cmd
}

// round2 rounds to two decimals using math.Round so small negative values
// (e.g. valuation drift deltas) round correctly instead of truncating to zero.
func round2(f float64) float64 {
	return math.Round(f*100) / 100
}
