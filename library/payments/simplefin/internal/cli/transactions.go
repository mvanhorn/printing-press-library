// Copyright 2026 Todd Dailey and contributors. Licensed under Apache-2.0. See LICENSE.
// Absorbed feature: the transaction ledger — list transactions from the local
// store with account, date, and category filters. This is the prominent
// "list my transactions" command.
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

func newTransactionsCmd(flags *rootFlags) *cobra.Command {
	var dbPath, account, since, until, category string
	var limit int
	var pending bool

	cmd := &cobra.Command{
		Use:     "transactions",
		Aliases: []string{"txns", "ledger"},
		Short:   "List transactions from the local store, filtered by account, date, or category",
		Long: "Show transactions across all institutions (or a single account) from the local store,\n" +
			"newest first. Filter by --account (id or name substring), --since/--until date window,\n" +
			"and --category (requires 'categorize' to have run). Fast and offline.",
		Example:     "  simplefin-pp-cli transactions --account Checking --since 30d --json",
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
			txns, err := loadTransactions(ctx, db, account, minP, maxP, pending)
			if err != nil {
				return err
			}
			cats, err := loadCategoryMap(ctx, db)
			if err != nil {
				return err
			}

			type txnView struct {
				ID          string  `json:"id"`
				Account     string  `json:"account"`
				Date        string  `json:"date"`
				Amount      float64 `json:"amount"`
				Description string  `json:"description"`
				Payee       string  `json:"payee,omitempty"`
				Category    string  `json:"category,omitempty"`
				Pending     bool    `json:"pending,omitempty"`
			}
			out := make([]txnView, 0, len(txns))
			for _, t := range txns {
				if category != "" && cats[t.ResourceID] != category {
					continue
				}
				amt, _ := simplefin.ParseAmount(t.Amount)
				out = append(out, txnView{
					ID: t.ID, Account: t.AccountName,
					Date:        time.Unix(t.Posted, 0).UTC().Format("2006-01-02"),
					Amount:      amt,
					Description: t.Description, Payee: t.Payee,
					Category: cats[t.ResourceID], Pending: t.Pending,
				})
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Date > out[j].Date })
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}

			if flags.asJSON || flags.agent || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return flags.printJSON(cmd, out)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no matching transactions — run: simplefin-pp-cli sync")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-10s %-32s %15s  %s\n", "date", "account", "amount", "description")
			for _, t := range out {
				p := ""
				if t.Pending {
					p = " (pending)"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-10s %-32s %15s  %s%s\n", t.Date, truncate(t.Account, 32), humanMoney(t.Amount), truncate(t.Description, 40), p)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "Filter by account id or name substring")
	cmd.Flags().StringVar(&since, "since", "", "Only transactions on or after this date (unix epoch, relative like 30d, or YYYY-MM-DD)")
	cmd.Flags().StringVar(&until, "until", "", "Only transactions before this date")
	cmd.Flags().StringVar(&category, "category", "", "Filter by assigned category (see 'categorize')")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum transactions to return")
	cmd.Flags().BoolVar(&pending, "pending", false, "Include pending transactions")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}
