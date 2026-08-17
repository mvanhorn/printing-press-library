// Copyright 2026 Adrian Horning and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Novel command: credit burn rate and runway projection.

package cli

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/spf13/cobra"
)

func newNovelAccountBudgetCmd(flags *rootFlags) *cobra.Command {
	var daysWindow int

	cmd := &cobra.Command{
		Use:         "budget",
		Short:       "See how fast you're spending API credits and how many days remain at the current pace.",
		Example:     "  scrape-creators-pp-cli account budget",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// account budget takes no positional input, so it always runs.
			if dryRunOK(flags) {
				return nil
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			failures := make([]fetchFailure, 0)

			// Credit balance is the core signal; a failure here is fatal.
			balRaw, err := c.Get(ctx, "/v1/account/credit-balance", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var bal struct {
				CreditCount json.Number `json:"creditCount"`
			}
			_ = json.Unmarshal(balRaw, &bal)
			creditBalance, _ := toInt64(bal.CreditCount)

			// Daily usage drives the burn-rate average; degrade gracefully.
			var avgDaily float64
			var daysSampled int
			if dailyRaw, derr := c.Get(ctx, "/v1/account/get-daily-usage-count", nil); derr != nil {
				failures = append(failures, fetchFailure{Source: "get-daily-usage-count", Error: sanitizeFetchErr(derr)})
			} else {
				var daily []struct {
					TotalCredits json.Number `json:"total_credits"`
				}
				_ = json.Unmarshal(dailyRaw, &daily)
				if daysWindow > 0 && len(daily) > daysWindow {
					daily = daily[:daysWindow] // API returns most-recent first
				}
				var sum int64
				for _, d := range daily {
					if v, ok := toInt64(d.TotalCredits); ok {
						sum += v
						daysSampled++
					}
				}
				if daysSampled > 0 {
					avgDaily = float64(sum) / float64(daysSampled)
				}
			}

			// Most-used routes are a cheap extra; degrade gracefully.
			topRoutes := make([]json.RawMessage, 0)
			if routesRaw, rerr := c.Get(ctx, "/v1/account/get-most-used-routes", nil); rerr != nil {
				failures = append(failures, fetchFailure{Source: "get-most-used-routes", Error: sanitizeFetchErr(rerr)})
			} else {
				routes := resultArray(routesRaw, "routes")
				for i, r := range routes {
					if i >= 5 {
						break
					}
					topRoutes = append(topRoutes, r)
				}
			}

			daysRemaining := math.Inf(1)
			if avgDaily > 0 {
				daysRemaining = float64(creditBalance) / avgDaily
			}

			warnFetchFailures(cmd, "account budget", failures)

			daysRemainingOut := any(nil)
			if !math.IsInf(daysRemaining, 1) {
				daysRemainingOut = math.Round(daysRemaining*10) / 10
			}

			if novelWantsMachine(cmd.OutOrStdout(), flags) {
				envelope := map[string]any{
					"credit_balance":   creditBalance,
					"avg_daily_spend":  int64(math.Round(avgDaily)),
					"days_sampled":     daysSampled,
					"days_remaining":   daysRemainingOut,
					"most_used_routes": topRoutes,
					"fetch_failures":   failures,
				}
				return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Credit balance:   %d\n", creditBalance)
			fmt.Fprintf(w, "Avg daily spend:  %d (over %d days)\n", int64(math.Round(avgDaily)), daysSampled)
			if math.IsInf(daysRemaining, 1) {
				fmt.Fprintf(w, "Days remaining:   n/a (no recent spend)\n")
			} else {
				fmt.Fprintf(w, "Days remaining:   %.1f\n", daysRemaining)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&daysWindow, "days", 0, "limit the burn-rate average to the most recent N days (0 = all available)")
	return cmd
}
