// Copyright 2026 Todd Dailey and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: cross-institution cash flow — income vs outflow by month and
// top-spend merchants, via SUM/GROUP BY over the local transaction store.
//
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/simplefin/internal/simplefin"
)

type cashflowBucket struct {
	Key     string  `json:"key"`
	Income  float64 `json:"income"`
	Outflow float64 `json:"outflow"`
	Net     float64 `json:"net"`
	Count   int     `json:"count"`
}

func newNovelCashflowCmd(flags *rootFlags) *cobra.Command {
	var flagMonth, flagByMerchant bool
	var since, until, dbPath, account string
	var limit int

	cmd := &cobra.Command{
		Use:   "cashflow",
		Short: "Income vs outflow by month and top-spend merchants across all accounts.",
		Long: "Aggregate signed transaction amounts across every account: income (deposits) vs outflow\n" +
			"(spending) vs net. Default is an overall summary; --month groups by calendar month and\n" +
			"--by-merchant ranks top spend by normalized payee — a cross-institution view no single\n" +
			"bank app provides.\n\n" +
			"Use this command for income-vs-outflow and top-spend merchants. For subscription/recurring\n" +
			"detection use 'recurring'. Pending transactions are excluded.",
		Example:     "  simplefin-pp-cli cashflow --month --agent --select buckets",
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

			now := time.Now().UTC()
			var minP, maxP int64
			if since != "" {
				t, err := simplefin.ParseDate(since, now)
				if err != nil {
					return usageErr(fmt.Errorf("--since: %w", err))
				}
				minP = t.Unix()
			}
			if until != "" {
				t, err := simplefin.ParseDate(until, now)
				if err != nil {
					return usageErr(fmt.Errorf("--until: %w", err))
				}
				maxP = t.Unix()
			}
			// Cash flow excludes pending (they skew totals).
			txns, err := loadTransactions(ctx, db, account, minP, maxP, false)
			if err != nil {
				return err
			}

			if flagByMerchant {
				return cashflowByMerchant(cmd, flags, txns, limit)
			}
			if flagMonth {
				return cashflowByMonth(cmd, flags, txns)
			}
			// Overall summary.
			var b cashflowBucket
			b.Key = "all"
			for _, t := range txns {
				amt, ok := simplefin.ParseAmount(t.Amount)
				if !ok {
					continue
				}
				if amt >= 0 {
					b.Income += amt
				} else {
					b.Outflow += -amt
				}
				b.Count++
			}
			b.Net = b.Income - b.Outflow
			if flags.asJSON || flags.agent || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return flags.printJSON(cmd, b)
			}
			if b.Count == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no transactions in range — run: simplefin-pp-cli sync")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Income   %16s\nOutflow  %16s\nNet      %16s  (%d transactions)\n", humanMoney(b.Income), humanMoney(b.Outflow), humanMoney(b.Net), b.Count)
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagMonth, "month", false, "Group cash flow by calendar month")
	cmd.Flags().BoolVar(&flagByMerchant, "by-merchant", false, "Rank top-spend merchants instead of a time summary")
	cmd.Flags().StringVar(&since, "since", "", "Only include transactions on or after this date (unix epoch, relative like 90d, or YYYY-MM-DD)")
	cmd.Flags().StringVar(&until, "until", "", "Only include transactions before this date")
	cmd.Flags().StringVar(&account, "account", "", "Restrict to a single account (id or name substring)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max merchants to show with --by-merchant")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}

func cashflowByMonth(cmd *cobra.Command, flags *rootFlags, txns []storedTxn) error {
	agg := map[string]*cashflowBucket{}
	for _, t := range txns {
		amt, ok := simplefin.ParseAmount(t.Amount)
		if !ok {
			continue
		}
		key := time.Unix(t.Posted, 0).UTC().Format("2006-01")
		b := agg[key]
		if b == nil {
			b = &cashflowBucket{Key: key}
			agg[key] = b
		}
		if amt >= 0 {
			b.Income += amt
		} else {
			b.Outflow += -amt
		}
		b.Count++
	}
	out := make([]cashflowBucket, 0, len(agg))
	for _, b := range agg {
		b.Net = b.Income - b.Outflow
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	if flags.asJSON || flags.agent || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
		return flags.printJSON(cmd, map[string]any{"buckets": out})
	}
	if len(out) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no transactions in range — run: simplefin-pp-cli sync")
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%-9s %16s %16s %16s\n", "month", "income", "outflow", "net")
	for _, b := range out {
		fmt.Fprintf(cmd.OutOrStdout(), "%-9s %16s %16s %16s\n", b.Key, humanMoney(b.Income), humanMoney(b.Outflow), humanMoney(b.Net))
	}
	return nil
}

func cashflowByMerchant(cmd *cobra.Command, flags *rootFlags, txns []storedTxn, limit int) error {
	type merch struct {
		Merchant string  `json:"merchant"`
		Spend    float64 `json:"spend"`
		Count    int     `json:"count"`
	}
	agg := map[string]*merch{}
	for _, t := range txns {
		amt, ok := simplefin.ParseAmount(t.Amount)
		if !ok || amt >= 0 {
			continue // only outflow (spend)
		}
		key := simplefin.NormalizePayee(simplefin.Transaction{Payee: t.Payee, Description: t.Description})
		if key == "" {
			key = "(unknown)"
		}
		m := agg[key]
		if m == nil {
			m = &merch{Merchant: key}
			agg[key] = m
		}
		m.Spend += -amt
		m.Count++
	}
	out := make([]merch, 0, len(agg))
	for _, m := range agg {
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Spend > out[j].Spend })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	if flags.asJSON || flags.agent || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
		return flags.printJSON(cmd, map[string]any{"merchants": out})
	}
	if len(out) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no spending transactions in range — run: simplefin-pp-cli sync")
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%-34s %16s %6s\n", "merchant", "spend", "txns")
	for _, m := range out {
		fmt.Fprintf(cmd.OutOrStdout(), "%-34s %16s %6d\n", truncate(m.Merchant, 34), humanMoney(m.Spend), m.Count)
	}
	return nil
}
