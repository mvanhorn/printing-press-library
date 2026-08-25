// Copyright 2026 Todd Dailey and contributors. Licensed under Apache-2.0. See LICENSE.
// Absorbed feature: stale-account detection. Flags accounts whose balance_date
// is older than a threshold — SimpleFIN connections can silently go stale (a
// "green dot" while the data is weeks old).
//
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

func newStaleCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var days int

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "List accounts whose balance is older than a threshold (silently stale connections)",
		Long: "SimpleFIN connections can go stale without an obvious error — the balance simply stops\n" +
			"updating. This flags accounts whose balance_date is older than --days so you know which\n" +
			"connection to refresh or reconnect.\n\n" +
			"For duplicate transactions use 'reconcile'.",
		Example:     "  simplefin-pp-cli stale --days 3 --json",
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

			accounts, err := loadAccounts(ctx, db)
			if err != nil {
				return err
			}
			cutoff := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour).Unix()
			type staleAcct struct {
				Account     string `json:"account"`
				BalanceDate string `json:"balance_date"`
				AgeDays     int    `json:"age_days"`
			}
			out := make([]staleAcct, 0)
			now := time.Now().UTC().Unix()
			for _, a := range accounts {
				if a.BalanceDate == 0 || a.BalanceDate >= cutoff {
					continue
				}
				out = append(out, staleAcct{
					Account:     a.Name,
					BalanceDate: time.Unix(a.BalanceDate, 0).UTC().Format("2006-01-02"),
					AgeDays:     int((now - a.BalanceDate) / 86400),
				})
			}
			sort.Slice(out, func(i, j int) bool { return out[i].AgeDays > out[j].AgeDays })

			if flags.asJSON || flags.agent || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return flags.printJSON(cmd, map[string]any{"stale_accounts": out, "threshold_days": days})
			}
			if len(accounts) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no accounts in the local store — run: simplefin-pp-cli sync")
				return nil
			}
			if len(out) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no accounts older than %d days — all connections fresh\n", days)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-46s %-12s %s\n", "account", "as of", "age")
			for _, s := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%-46s %-12s %d days\n", truncate(s.Account, 46), s.BalanceDate, s.AgeDays)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&days, "days", 3, "Flag accounts whose balance is older than this many days")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}
