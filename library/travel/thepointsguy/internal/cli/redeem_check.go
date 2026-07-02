// Copyright 2026 megumikuo and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: points-vs-cash verdict for one booking against TPG's valuation.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source auto
func newNovelRedeemCheckCmd(flags *rootFlags) *cobra.Command {
	var program string
	var points float64
	var cash float64

	cmd := &cobra.Command{
		Use:   "redeem-check",
		Short: "Decide whether to use points or pay cash for a specific booking",
		Long: strings.TrimSpace(`
Compare a specific redemption against The Points Guy's valuation. Given the
points a booking costs and its cash price, this computes the redemption's
cents-per-point and compares it to TPG's baseline valuation for the program,
returning a "redeem" or "pay cash" verdict.`),
		Example: strings.Trim(`
  thepointsguy-pp-cli redeem-check --program "Chase Ultimate Rewards" --points 60000 --cash 900
  thepointsguy-pp-cli redeem-check --program "United MileagePlus" --points 80000 --cash 700 --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would evaluate a redemption")
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
			if cash <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--cash (the cash price of the booking) must be a positive number"))
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
			redemptionCPP := centsPerPoint(cash, points)
			baseline := v.CentsPerPoint
			verdict := "pay cash"
			reason := "the redemption value is below TPG's baseline, so points are better saved"
			if redemptionCPP >= baseline {
				verdict = "redeem points"
				reason = "the redemption value meets or beats TPG's baseline"
			}
			view := struct {
				Program          string  `json:"program"`
				Points           float64 `json:"points"`
				CashPriceUSD     float64 `json:"cash_price_usd"`
				RedemptionCPP    float64 `json:"redemption_cents_per_point"`
				BaselineCPP      float64 `json:"tpg_baseline_cents_per_point"`
				Verdict          string  `json:"verdict"`
				Reason           string  `json:"reason"`
				CashValueOfPoint float64 `json:"tpg_value_of_points_usd"`
				Month            string  `json:"month"`
			}{
				Program:          v.Program,
				Points:           points,
				CashPriceUSD:     cash,
				RedemptionCPP:    round2(redemptionCPP),
				BaselineCPP:      baseline,
				Verdict:          verdict,
				Reason:           reason,
				CashValueOfPoint: round2(dollarsFromPoints(points, baseline)),
				Month:            month,
			}
			if flags.asJSON || flags.agent {
				return emitJSON(cmd, flags, view)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"Verdict: %s\n  This redemption: %.2f¢/point ($%.0f cash for %.0f points)\n  TPG baseline:    %.2f¢/point (%s)\n  %s\n",
				strings.ToUpper(verdict), redemptionCPP, cash, points, baseline, v.Program, reason)
			return nil
		},
	}
	cmd.Flags().StringVar(&program, "program", "", "Loyalty program used for the redemption, e.g. \"Chase Ultimate Rewards\"")
	cmd.Flags().Float64Var(&points, "points", 0, "Points/miles the award booking costs")
	cmd.Flags().Float64Var(&cash, "cash", 0, "Cash price of the same booking, in USD")
	return cmd
}
