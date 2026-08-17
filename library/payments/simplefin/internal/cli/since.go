// Copyright 2026 Todd Dailey and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: recent activity since a date or the last sync — new
// transactions and balance changes across every account, neutrally framed.
// Diffs the local store against a point in time; the API only returns
// point-in-time state, not "what changed".
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

type sinceAccount struct {
	Account       string  `json:"account"`
	NewTxns       int     `json:"new_transactions"`
	NewTxnTotal   float64 `json:"new_transaction_total"`
	Balance       float64 `json:"balance"`
	BalanceChange float64 `json:"balance_change"`
}

func newNovelSinceCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var sinceFlag string

	cmd := &cobra.Command{
		Use:   "since [date]",
		Short: "What's new across all accounts since a date or your last sync — new transactions and balance changes, neutrally framed.",
		Long: "Summarize recent activity across every account since a date: how many new transactions\n" +
			"posted (and their net total) and how each balance moved versus the nearest earlier snapshot.\n" +
			"A neutral 'what happened lately' digest — not an alert.\n\n" +
			"Accepts a positional date (unix epoch, relative like 7d/2w, or YYYY-MM-DD); defaults to 7d.\n" +
			"Use this for recent activity across accounts. For full net-worth history use 'networth --trend'.",
		Example:     "  simplefin-pp-cli since 7d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if dryRunOK(flags) {
				return nil
			}
			now := time.Now().UTC()
			spec := sinceFlag
			if len(args) > 0 {
				spec = args[0]
			}
			if spec == "" {
				spec = "7d"
			}
			sinceTime, err := simplefin.ParseDate(spec, now)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}

			db, ok, err := resolveSimplefinDB(ctx, cmd, flags, dbPath)
			if err != nil || !ok {
				return err
			}
			defer db.Close()
			hintIfUnsynced(cmd, db, "transactions")

			accounts, err := loadAccounts(ctx, db)
			if err != nil {
				return err
			}
			txns, err := loadTransactions(ctx, db, "", sinceTime.Unix(), 0, false)
			if err != nil {
				return err
			}

			perAcct := map[string]*sinceAccount{}
			for _, a := range accounts {
				bal, _ := simplefin.ParseAmount(a.Balance)
				perAcct[a.ID] = &sinceAccount{Account: a.Name, Balance: bal}
			}
			for _, t := range txns {
				sa := perAcct[t.AccountID]
				if sa == nil {
					sa = &sinceAccount{Account: t.AccountName}
					perAcct[t.AccountID] = sa
				}
				sa.NewTxns++
				if amt, ok := simplefin.ParseAmount(t.Amount); ok {
					sa.NewTxnTotal += amt
				}
			}
			// Balance change vs nearest snapshot at/before sinceTime.
			for id, sa := range perAcct {
				var baseline float64
				err := db.DB().QueryRowContext(ctx,
					`SELECT balance FROM balance_snapshots WHERE account_id = ? AND snapshot_at <= ? ORDER BY snapshot_at DESC LIMIT 1`,
					id, sinceTime.Unix()).Scan(&baseline)
				if err == nil {
					sa.BalanceChange = sa.Balance - baseline
				}
			}

			out := make([]sinceAccount, 0, len(perAcct))
			var totalNew int
			var totalNet float64
			for _, sa := range perAcct {
				out = append(out, *sa)
				totalNew += sa.NewTxns
				totalNet += sa.NewTxnTotal
			}
			sort.Slice(out, func(i, j int) bool { return out[i].NewTxns > out[j].NewTxns })

			view := map[string]any{
				"since":            sinceTime.Format("2006-01-02"),
				"accounts":         out,
				"total_new_txns":   totalNew,
				"total_net_change": totalNet,
			}
			if flags.asJSON || flags.agent || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return flags.printJSON(cmd, view)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Activity since %s:\n\n", sinceTime.Format("2006-01-02"))
			if len(accounts) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no accounts in the local store — run: simplefin-pp-cli sync")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-46s %6s %16s %16s %16s\n", "account", "new", "net", "balance", "Δ balance")
			for _, sa := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%-46s %6d %16s %16s %16s\n",
					truncate(sa.Account, 46), sa.NewTxns, humanMoney(sa.NewTxnTotal), humanMoney(sa.Balance), humanMoney(sa.BalanceChange))
			}
			netSign := "+"
			if totalNet < 0 {
				netSign = "-"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d new transactions, net %s%s across all accounts\n", totalNew, netSign, humanMoney(abs(totalNet)))
			return nil
		},
	}
	cmd.Flags().StringVar(&sinceFlag, "since", "", "Date to summarize activity since (unix epoch, relative like 7d, or YYYY-MM-DD); defaults to 7d")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}
