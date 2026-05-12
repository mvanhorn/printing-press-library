// Copyright 2026 mani. Licensed under Apache-2.0. See LICENSE.
// PATCH: novel budget-watch and cost-report commands.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/ai/tavily/internal/store"
)

func newBudgetWatchCmd(flags *rootFlags) *cobra.Command {
	var dailyLimit float64
	var watchInterval time.Duration

	cmd := &cobra.Command{
		Use:   "budget-watch",
		Short: "Monitor credit burn rate against a daily limit with terminal alerts",
		Long: `Continuously monitor your Tavily credit spend rate against a daily
budget limit. Reads spend history from the local SQLite store and shows
current burn rate, projected daily spend, and an alert if the limit will
be exceeded at the current rate.

Run once for a snapshot, or combine with watch(1) for continuous monitoring.`,
		Example: `  tavily-pp-cli budget-watch --limit 100
  tavily-pp-cli budget-watch --limit 500 --interval 60s
  watch -n 30 tavily-pp-cli budget-watch --limit 100`,
		Annotations: map[string]string{"pp:novel": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dailyLimit <= 0 {
				dailyLimit = 1000
			}

			st, err := store.Open()
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer st.Close()

			printSnapshot := func() error {
				ratePerHour, err := st.RatePerHour(24)
				if err != nil {
					return fmt.Errorf("reading credit rate: %w", err)
				}
				projectedDaily := ratePerHour * 24

				todaySince := startOfLocalDay(time.Now())
				todayByEndpoint, err := st.CreditsSince(todaySince)
				if err != nil {
					return fmt.Errorf("reading today credits: %w", err)
				}
				var todayTotal float64
				for _, v := range todayByEndpoint {
					todayTotal += v
				}

				if flags.asJSON {
					out := map[string]any{
						"daily_limit":      dailyLimit,
						"today_used":       todayTotal,
						"rate_per_hour":    ratePerHour,
						"projected_daily":  projectedDaily,
						"headroom":         dailyLimit - todayTotal,
						"alert":            projectedDaily > dailyLimit,
						"by_endpoint":      todayByEndpoint,
					}
					data, _ := json.MarshalIndent(out, "", "  ")
					return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
				}

				fmt.Fprintf(cmd.OutOrStdout(), "Budget watch  (limit: %.0f cr/day)\n", dailyLimit)
				fmt.Fprintf(cmd.OutOrStdout(), "  Today used:       %.1f cr\n", todayTotal)
				fmt.Fprintf(cmd.OutOrStdout(), "  Burn rate:        %.2f cr/hr\n", ratePerHour)
				fmt.Fprintf(cmd.OutOrStdout(), "  Projected daily:  %.1f cr\n", projectedDaily)
				fmt.Fprintf(cmd.OutOrStdout(), "  Headroom:         %.1f cr\n", dailyLimit-todayTotal)
				if projectedDaily > dailyLimit {
					fmt.Fprintf(cmd.OutOrStdout(), "\n  ALERT: Projected spend (%.0f) exceeds limit (%.0f)\n",
						projectedDaily, dailyLimit)
				}
				if len(todayByEndpoint) > 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "\n  By endpoint:")
					for ep, cr := range todayByEndpoint {
						fmt.Fprintf(cmd.OutOrStdout(), "    %-12s %.1f cr\n", ep, cr)
					}
				}
				return nil
			}

			if err := printSnapshot(); err != nil {
				return err
			}

			// Optional continuous loop
			if watchInterval > 0 {
				for {
					time.Sleep(watchInterval)
					fmt.Fprintln(cmd.OutOrStdout(), "\n---")
					if err := printSnapshot(); err != nil {
						return err
					}
				}
			}
			return nil
		},
	}

	cmd.Flags().Float64Var(&dailyLimit, "limit", 1000, "Daily credit limit (alert threshold)")
	cmd.Flags().DurationVar(&watchInterval, "interval", 0, "Refresh interval for continuous monitoring (0=one-shot)")
	return cmd
}

func newCostReportCmd(flags *rootFlags) *cobra.Command {
	var window string
	var session string

	cmd := &cobra.Command{
		Use:   "cost-report",
		Short: "Credit spend breakdown by endpoint and time window",
		Long: `Show a credit spend breakdown from the local SQLite store. Supports
time windows (today, week, month, all) and optional session filtering.`,
		Example: `  tavily-pp-cli cost-report
  tavily-pp-cli cost-report --window week
  tavily-pp-cli cost-report --session my-agent --json`,
		Annotations: map[string]string{"pp:novel": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := store.Open()
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer st.Close()

			var since time.Time
			switch window {
			case "today":
				since = startOfLocalDay(time.Now())
			case "week":
				since = time.Now().AddDate(0, 0, -7)
			case "month":
				since = time.Now().AddDate(0, -1, 0)
			default: // all
				since = time.Time{}
			}

			byEndpoint, err := st.CreditsSince(since)
			if err != nil {
				return fmt.Errorf("reading credits: %w", err)
			}

			var sessionTotal float64
			if session != "" {
				sessionTotal, err = st.CreditsBySession(session)
				if err != nil {
					return fmt.Errorf("reading session credits: %w", err)
				}
			}

			var total float64
			for _, v := range byEndpoint {
				total += v
			}

			if flags.asJSON {
				out := map[string]any{
					"window":       window,
					"total":        total,
					"by_endpoint":  byEndpoint,
					"session":      session,
					"session_total": sessionTotal,
				}
				data, _ := json.MarshalIndent(out, "", "  ")
				return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
			}

			windowLabel := window
			if windowLabel == "" {
				windowLabel = "all time"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Cost report (%s)\n\n", windowLabel)
			if len(byEndpoint) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No spend recorded yet. Run some API calls with tavily-pp-cli first.")
				return nil
			}
			for ep, cr := range byEndpoint {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-12s %6.1f cr\n", ep, cr)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %-12s %6.1f cr\n", "TOTAL", total)
			if session != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\n  Session %q: %.1f cr\n", session, sessionTotal)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&window, "window", "all", "Time window: today, week, month, all")
	cmd.Flags().StringVar(&session, "session", "", "Filter by session label")
	return cmd
}

// startOfLocalDay returns midnight in the caller's local timezone for the
// calendar day containing t. time.Time.Truncate(24h) operates on absolute
// time and snaps to UTC midnight, which would slip "today" by up to ±12h
// for users outside UTC and undercount same-day spend.
func startOfLocalDay(t time.Time) time.Time {
	loc := t.Location()
	if loc == nil {
		loc = time.Local
	}
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}
