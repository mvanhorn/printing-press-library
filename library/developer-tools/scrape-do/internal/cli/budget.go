// Copyright 2026 Charles Garrison and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command `budget` — the credit governor's read side.
// Joins the local credit ledger (debited from the authoritative per-call cost
// header) with the live /info account state to show remaining credits,
// concurrency headroom, month-to-date spend attributed by mode and agent, and
// a burn-rate forecast against days left in the month. `budget set` configures
// the spend ceiling enforced by every billed command. Hand file (no generator
// header) so it survives regeneration.

package cli

import (
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-do/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-do/internal/config"
	"github.com/spf13/cobra"
)

func newNovelBudgetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "budget",
		Short: "Credit & concurrency governor: remaining credits, headroom, spend attribution, burn forecast",
		Long: `Show the local credit ledger joined with live account state: remaining
credits, concurrency headroom, month-to-date spend attributed by mode and
agent, and a burn-rate forecast against the days left in the month. Use
'budget set' to configure the spend ceiling every billed command enforces.`,
		Example:     "  scrape-do-pp-cli budget\n  scrape-do-pp-cli budget --agent\n  scrape-do-pp-cli budget set --max-credits 500",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return usageErr(fmt.Errorf("unknown budget subcommand %q (did you mean 'budget set'?)", args[0]))
			}
			if dryRunOK(flags) {
				return nil
			}
			st, ext, err := openExtras(cmd.Context(), "")
			if err != nil {
				return err
			}
			defer st.Close()

			now := time.Now().UTC()

			// Live /info: refresh unless under verify (which must not hit network).
			usage, _ := ext.LatestUsage(cmd.Context())
			if !cliutil.IsVerifyEnv() {
				if cfg, cerr := config.Load(flags.configPath); cerr == nil {
					if u, ferr := fetchInfo(cmd.Context(), cfg, flags.govHTTPClient()); ferr == nil {
						_ = ext.SaveUsage(cmd.Context(), u)
						usage = &u
					}
				}
			}

			monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
			spend, _ := ext.Spend(cmd.Context(), monthStart)
			budget, _ := ext.GetBudget(cmd.Context())

			daysInMonth := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
			dayOfMonth := now.Day()
			daysLeft := daysInMonth - dayOfMonth
			burnPerDay := 0.0
			if dayOfMonth > 0 {
				burnPerDay = float64(spend.TotalCredits) / float64(dayOfMonth)
			}
			projected := burnPerDay * float64(daysInMonth)

			payload := map[string]any{
				"month_spend_ledger": spend.TotalCredits,
				"calls_this_month":   spend.Calls,
				"by_mode":            spend.ByMode,
				"by_agent":           spend.ByAgent,
				"burn_per_day":       round2(burnPerDay),
				"projected_month":    round2(projected),
				"days_left":          daysLeft,
			}
			if usage != nil {
				payload["remaining_credits"] = usage.RemainingMonthly
				payload["max_monthly"] = usage.MaxMonthly
				payload["concurrent_cap"] = usage.ConcurrentRequest
				payload["remaining_concurrent"] = usage.RemainingConcurrent
				payload["is_active"] = usage.IsActive
			}
			if budget.MaxCredits.Valid {
				payload["ceiling_max_credits"] = budget.MaxCredits.Int64
			}
			if budget.MaxMonthlyPct.Valid {
				payload["ceiling_max_monthly_pct"] = budget.MaxMonthlyPct.Float64
			}

			rem := "?"
			cap := "?"
			head := "?"
			if usage != nil {
				rem = fmt.Sprintf("%d/%d", usage.RemainingMonthly, usage.MaxMonthly)
				cap = fmt.Sprintf("%d", usage.ConcurrentRequest)
				head = fmt.Sprintf("%d", usage.RemainingConcurrent)
			}
			text := fmt.Sprintf("credits remaining: %s\nconcurrency: %s free of %s cap\nmonth spend (ledger): %d credits across %d calls\nburn: %.1f/day  projected month: %.0f  days left: %d",
				rem, head, cap, spend.TotalCredits, spend.Calls, burnPerDay, projected, daysLeft)
			return emitGov(cmd, flags, payload, text)
		},
	}
	cmd.AddCommand(newBudgetSetCmd(flags))
	return cmd
}

func newBudgetSetCmd(flags *rootFlags) *cobra.Command {
	var maxCredits int
	var maxMonthlyPct float64
	cmd := &cobra.Command{
		Use:         "set",
		Short:       "Set the spend ceiling enforced by every billed command",
		Long:        "Configure the month-to-date credit ceiling. Once set, scrape/google/batch refuse to dispatch a call whose estimate would push month spend past the ceiling (exit code 4).",
		Example:     "  scrape-do-pp-cli budget set --max-credits 500",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("max-credits") && !cmd.Flags().Changed("max-monthly-pct") {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitGov(cmd, flags, map[string]any{
					"would_set_max_credits":     maxCredits,
					"would_set_max_monthly_pct": maxMonthlyPct,
				}, fmt.Sprintf("would set budget ceiling (max-credits=%d)", maxCredits))
			}
			st, ext, err := openExtras(cmd.Context(), "")
			if err != nil {
				return err
			}
			defer st.Close()
			var mc *int
			var mp *float64
			if cmd.Flags().Changed("max-credits") {
				mc = &maxCredits
			}
			if cmd.Flags().Changed("max-monthly-pct") {
				mp = &maxMonthlyPct
			}
			if err := ext.SetBudget(cmd.Context(), mc, mp, time.Now()); err != nil {
				return err
			}
			b, _ := ext.GetBudget(cmd.Context())
			payload := map[string]any{"status": "saved"}
			if b.MaxCredits.Valid {
				payload["max_credits"] = b.MaxCredits.Int64
			}
			if b.MaxMonthlyPct.Valid {
				payload["max_monthly_pct"] = b.MaxMonthlyPct.Float64
			}
			return emitGov(cmd, flags, payload, "budget ceiling saved")
		},
	}
	cmd.Flags().IntVar(&maxCredits, "max-credits", 0, "Month-to-date credit ceiling (0 to clear is not supported; set a positive value)")
	cmd.Flags().Float64Var(&maxMonthlyPct, "max-monthly-pct", 0, "Reserved: ceiling as a percent of the monthly cap")
	return cmd
}

func round2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}
