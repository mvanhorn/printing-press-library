// Copyright 2026 Todd Dailey and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: cross-institution net worth + balance trajectory over time.
// The protocol only returns point-in-time balances; the balance_snapshots table
// (stamped on every sync) is what gives the trend no single bank app can show.
//
// pp:data-source local

package cli

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/simplefin/internal/simplefin"
	"github.com/mvanhorn/printing-press-library/library/payments/simplefin/internal/store"
)

func isLiabilityType(t string) bool {
	return t == "credit-card" || t == "loan"
}

type netWorthAccount struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Type        string  `json:"account_type"`
	Currency    string  `json:"currency"`
	Balance     float64 `json:"balance"`
	IsLiability bool    `json:"is_liability"`
}

type netWorthView struct {
	NetWorth    float64           `json:"net_worth"`
	Assets      float64           `json:"assets"`
	Liabilities float64           `json:"liabilities"`
	Currency    string            `json:"currency"`
	Accounts    []netWorthAccount `json:"accounts"`
	Note        string            `json:"note,omitempty"`
}

type trendPoint struct {
	Date     string  `json:"date"`
	At       int64   `json:"snapshot_at"`
	NetWorth float64 `json:"net_worth"`
}

func newNovelNetworthCmd(flags *rootFlags) *cobra.Command {
	var flagTrend bool
	var flagAt string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "networth",
		Short: "Total net worth across every institution, with a balance trajectory over time no bank app can show.",
		Long: "Sum balances across every connected account into one net-worth figure. Credit cards and\n" +
			"loans are treated as liabilities. With --trend, join the per-sync balance snapshot series\n" +
			"to show how net worth has moved over time — a trajectory the SimpleFIN protocol (which only\n" +
			"returns point-in-time balances) cannot provide on its own.\n\n" +
			"Use this command for total or historical net worth across institutions. Do NOT use it for\n" +
			"investment gain/loss — use 'portfolio'. Do NOT use it for spending breakdowns — use 'cashflow'.",
		Example:     "  simplefin-pp-cli networth --trend --agent",
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
			hintIfUnsynced(cmd, db, "accounts")

			if flagTrend {
				var minAt int64
				if flagAt != "" {
					t, err := simplefin.ParseDate(flagAt, time.Now().UTC())
					if err != nil {
						return usageErr(fmt.Errorf("--at: %w", err))
					}
					minAt = t.Unix()
				}
				return runNetWorthTrend(ctx, cmd, flags, db, minAt)
			}

			accounts, err := loadAccounts(ctx, db)
			if err != nil {
				return err
			}
			view := netWorthView{}
			currency := ""
			for _, a := range accounts {
				bal, _ := simplefin.ParseAmount(a.Balance)
				liab := isLiabilityType(a.AccountType)
				if liab {
					if bal < 0 {
						bal = -bal
					}
					view.Liabilities += bal
				} else {
					view.Assets += bal
				}
				if currency == "" {
					currency = a.Currency
				} else if currency != a.Currency && a.Currency != "" {
					currency = "mixed"
				}
				view.Accounts = append(view.Accounts, netWorthAccount{
					ID: a.ID, Name: a.Name, Type: a.AccountType, Currency: a.Currency,
					Balance: bal, IsLiability: liab,
				})
			}
			view.NetWorth = view.Assets - view.Liabilities
			view.Currency = currency
			if currency == "mixed" {
				view.Note = "accounts span multiple currencies; totals are a naive sum and not FX-converted"
			}
			sort.Slice(view.Accounts, func(i, j int) bool { return view.Accounts[i].Balance > view.Accounts[j].Balance })

			if flags.asJSON || flags.agent || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return flags.printJSON(cmd, view)
			}
			if len(accounts) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no accounts in the local store — run: simplefin-pp-cli sync")
				return nil
			}
			for _, a := range view.Accounts {
				tag := ""
				if a.IsLiability {
					tag = " (liability)"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-46s %18s  %s%s\n", truncate(a.Name, 46), humanMoney(a.Balance), a.Type, tag)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", "---------------------------------------------------------------------")
			fmt.Fprintf(cmd.OutOrStdout(), "%-46s %18s\n", "Assets", humanMoney(view.Assets))
			fmt.Fprintf(cmd.OutOrStdout(), "%-46s %18s\n", "Liabilities", humanMoney(view.Liabilities))
			fmt.Fprintf(cmd.OutOrStdout(), "%-46s %18s %s\n", "Net worth", humanMoney(view.NetWorth), view.Currency)
			if view.Note != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "note: %s\n", view.Note)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagTrend, "trend", false, "Show net worth at each historical sync snapshot instead of just now")
	cmd.Flags().StringVar(&flagAt, "at", "", "(with --trend) only show snapshots on or after this date (unix epoch, relative, or YYYY-MM-DD)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}

func runNetWorthTrend(ctx context.Context, cmd *cobra.Command, flags *rootFlags, db *store.Store, minAt int64) error {
	// Map account_id -> liability so the snapshot sum applies the same sign rule.
	accounts, err := loadAccounts(ctx, db)
	if err != nil {
		return err
	}
	liab := map[string]bool{}
	for _, a := range accounts {
		liab[a.ID] = isLiabilityType(a.AccountType)
	}
	rows, err := db.DB().QueryContext(ctx,
		`SELECT account_id, snapshot_at, balance FROM balance_snapshots WHERE snapshot_at >= ? ORDER BY snapshot_at ASC`, minAt)
	if err != nil {
		return err
	}
	type rec struct {
		acct string
		bal  float64
	}
	byTime := map[int64][]rec{}
	order := []int64{}
	seen := map[int64]bool{}
	for rows.Next() {
		var acct string
		var at int64
		var bal float64
		if err := rows.Scan(&acct, &at, &bal); err != nil {
			_ = rows.Close()
			return err
		}
		if !seen[at] {
			seen[at] = true
			order = append(order, at)
		}
		byTime[at] = append(byTime[at], rec{acct, bal})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	points := make([]trendPoint, 0, len(order))
	for _, at := range order {
		var net float64
		for _, r := range byTime[at] {
			b := r.bal
			if liab[r.acct] {
				if b < 0 {
					b = -b
				}
				net -= b
			} else {
				net += b
			}
		}
		points = append(points, trendPoint{
			Date: time.Unix(at, 0).UTC().Format("2006-01-02 15:04"), At: at, NetWorth: net,
		})
	}

	if flags.asJSON || flags.agent || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
		return flags.printJSON(cmd, map[string]any{"trend": points})
	}
	if len(points) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no balance snapshots yet — run 'sync' a few times to build a trend")
		return nil
	}
	var prev float64
	for i, p := range points {
		delta := ""
		if i > 0 {
			d := p.NetWorth - prev
			sign := "+"
			if d < 0 {
				sign = "-"
			}
			delta = fmt.Sprintf("  (%s%s)", sign, humanMoney(abs(d)))
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s   %18s%s\n", p.Date, humanMoney(p.NetWorth), delta)
		prev = p.NetWorth
	}
	return nil
}
