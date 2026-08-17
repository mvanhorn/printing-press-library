// Copyright 2026 Todd Dailey and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: recurring/subscription detection. Groups transactions by
// normalized payee, then keeps groups with consistent amounts and a regular
// cadence — interval rules, not an LLM. Cross-institution by construction.
//
// pp:data-source local

package cli

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/simplefin/internal/simplefin"
)

type recurringItem struct {
	Merchant     string  `json:"merchant"`
	Occurrences  int     `json:"occurrences"`
	AvgAmount    float64 `json:"avg_amount"`
	Cadence      string  `json:"cadence"`
	IntervalDays int     `json:"median_interval_days"`
	LastSeen     string  `json:"last_seen"`
	EstMonthly   float64 `json:"est_monthly_cost"`
}

func newNovelRecurringCmd(flags *rootFlags) *cobra.Command {
	var minOccurrences, limit int
	var dbPath string
	var includeIncome bool

	cmd := &cobra.Command{
		Use:   "recurring",
		Short: "Surfaces subscriptions and regular obligations by detecting repeated payees with regular cadence.",
		Long: "Detect subscriptions and regular obligations across ALL accounts at once by grouping\n" +
			"transactions on a normalized payee key, then keeping groups whose amounts are consistent\n" +
			"and whose intervals are regular (weekly/biweekly/monthly/quarterly/yearly). Deterministic\n" +
			"interval rules — no LLM.\n\n" +
			"Use this command for finding subscriptions and recurring charges. For one-off or total\n" +
			"spend use 'cashflow'. By default only outflows (charges) are considered.",
		Example:     "  simplefin-pp-cli recurring --min-occurrences 3 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if dryRunOK(flags) {
				return nil
			}
			db, ok, err := resolveSimplefinDB(ctx, cmd, flags, dbPath)
			if err != nil || !ok {
				return err
			}
			defer db.Close()
			hintIfUnsynced(cmd, db, "transactions")

			txns, err := loadTransactions(ctx, db, "", 0, 0, false)
			if err != nil {
				return err
			}

			type grp struct {
				amounts []float64
				posted  []int64
			}
			groups := map[string]*grp{}
			for _, t := range txns {
				amt, ok := simplefin.ParseAmount(t.Amount)
				if !ok {
					continue
				}
				if !includeIncome && amt >= 0 {
					continue
				}
				key := simplefin.NormalizePayee(simplefin.Transaction{Payee: t.Payee, Description: t.Description})
				if key == "" {
					continue
				}
				g := groups[key]
				if g == nil {
					g = &grp{}
					groups[key] = g
				}
				g.amounts = append(g.amounts, amt)
				g.posted = append(g.posted, t.Posted)
			}

			out := make([]recurringItem, 0)
			for key, g := range groups {
				if len(g.amounts) < minOccurrences {
					continue
				}
				mean, cv := meanCV(g.amounts)
				if cv > 0.40 { // amounts too inconsistent to be a fixed subscription
					continue
				}
				sort.Slice(g.posted, func(i, j int) bool { return g.posted[i] < g.posted[j] })
				intervals := make([]float64, 0, len(g.posted)-1)
				for i := 1; i < len(g.posted); i++ {
					d := float64(g.posted[i]-g.posted[i-1]) / 86400.0
					if d > 0 {
						intervals = append(intervals, d)
					}
				}
				if len(intervals) == 0 {
					continue
				}
				med := median(intervals)
				cadence := cadenceLabel(med)
				if cadence == "irregular" {
					continue
				}
				monthly := math.Abs(mean) * (30.4 / med)
				out = append(out, recurringItem{
					Merchant:     key,
					Occurrences:  len(g.amounts),
					AvgAmount:    mean,
					Cadence:      cadence,
					IntervalDays: int(math.Round(med)),
					LastSeen:     time.Unix(g.posted[len(g.posted)-1], 0).UTC().Format("2006-01-02"),
					EstMonthly:   monthly,
				})
			}
			sort.Slice(out, func(i, j int) bool { return out[i].EstMonthly > out[j].EstMonthly })
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}

			if flags.asJSON || flags.agent || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return flags.printJSON(cmd, map[string]any{"recurring": out})
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no recurring charges detected (try --min-occurrences 2, or run: simplefin-pp-cli sync)")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-34s %5s %14s %-10s %14s\n", "merchant", "n", "avg", "cadence", "~monthly")
			for _, r := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%-34s %5d %14s %-10s %14s\n", truncate(r.Merchant, 34), r.Occurrences, humanMoney(r.AvgAmount), r.Cadence, humanMoney(r.EstMonthly))
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&minOccurrences, "min-occurrences", 3, "Minimum times a payee must recur to count")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max recurring items to return")
	cmd.Flags().BoolVar(&includeIncome, "include-income", false, "Also detect recurring deposits (paychecks), not just charges")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}

func meanCV(xs []float64) (mean, cv float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	mean = sum / float64(len(xs))
	if mean == 0 {
		return 0, 0
	}
	var ss float64
	for _, x := range xs {
		ss += (x - mean) * (x - mean)
	}
	std := math.Sqrt(ss / float64(len(xs)))
	return mean, math.Abs(std / mean)
}

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

func cadenceLabel(days float64) string {
	switch {
	case days >= 5 && days <= 9:
		return "weekly"
	case days >= 12 && days <= 16:
		return "biweekly"
	case days >= 26 && days <= 35:
		return "monthly"
	case days >= 55 && days <= 65:
		return "bimonthly"
	case days >= 84 && days <= 96:
		return "quarterly"
	case days >= 350 && days <= 380:
		return "yearly"
	default:
		return "irregular"
	}
}
