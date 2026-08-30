// Copyright 2026 Todd Dailey and contributors. Licensed under Apache-2.0. See LICENSE.
// Absorbed feature: list accounts with current balances. Offline-first (reads
// the local store); --live fetches fresh balances from the SimpleFIN server.
// Replaces the generated live-only promoted accounts command, which passed
// relative --start-date values straight through and got HTTP 400 (SimpleFIN
// requires unix-epoch timestamps).
//
// pp:data-source auto

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/simplefin/internal/simplefin"
)

type accountView struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Type             string  `json:"account_type"`
	Currency         string  `json:"currency"`
	Balance          float64 `json:"balance"`
	AvailableBalance float64 `json:"available_balance"`
}

func newAccountsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var live bool

	cmd := &cobra.Command{
		Use:   "accounts",
		Short: "List accounts and balances across every institution (offline from the local store)",
		Long: "Show every connected account with its current balance. Reads the local store by default\n" +
			"(fast, offline, no rate-limit cost). Pass --live to fetch fresh balances directly from the\n" +
			"SimpleFIN server.\n\n" +
			"To pull full transaction history into the store, use 'sync'.",
		Example:     "  simplefin-pp-cli accounts --json",
		Annotations: map[string]string{"pp:endpoint": "accounts.list", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if dryRunOK(flags) {
				return nil
			}

			live = live || flags.dataSource == "live"

			var views []accountView
			if live {
				// balances-only keeps the live fetch light and avoids the
				// 90-day transaction window entirely (no date params -> no 400).
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				if c.Config.AuthHeader() == "" {
					return authErr(errNoSimplefinCredentials())
				}
				raw, err := c.GetNoCache(ctx, "/accounts", map[string]string{"balances-only": "1", "version": "2"})
				if err != nil {
					return classifyAPIError(err, flags)
				}
				set, err := simplefin.ParseAccountSet(raw)
				if err != nil {
					return err
				}
				for _, a := range set.Accounts {
					bal, _ := simplefin.ParseAmount(a.Balance)
					av, _ := simplefin.ParseAmount(a.AvailableBalance)
					views = append(views, accountView{ID: a.ID, Name: a.Name, Type: inferAccountType(a), Currency: a.Currency, Balance: bal, AvailableBalance: av})
				}
			} else {
				db, ok, err := resolveSimplefinDB(ctx, cmd, flags, dbPath)
				if err != nil || !ok {
					return err
				}
				defer db.Close()
				hintIfUnsynced(cmd, db, "accounts")
				accts, err := loadAccounts(ctx, db)
				if err != nil {
					return err
				}
				for _, a := range accts {
					bal, _ := simplefin.ParseAmount(a.Balance)
					av, _ := simplefin.ParseAmount(a.AvailableBalance)
					views = append(views, accountView{ID: a.ID, Name: a.Name, Type: a.AccountType, Currency: a.Currency, Balance: bal, AvailableBalance: av})
				}
			}
			sort.Slice(views, func(i, j int) bool { return views[i].Balance > views[j].Balance })

			if flags.asJSON || flags.agent || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return flags.printJSON(cmd, views)
			}
			if len(views) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no accounts — run: simplefin-pp-cli sync (or 'accounts --live')")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-46s %-12s %16s %16s\n", "account", "type", "balance", "available")
			for _, v := range views {
				fmt.Fprintf(cmd.OutOrStdout(), "%-46s %-12s %16s %16s\n", truncate(v.Name, 46), v.Type, humanMoney(v.Balance), humanMoney(v.AvailableBalance))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&live, "live", false, "Fetch fresh balances from the SimpleFIN server instead of the local store")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}
