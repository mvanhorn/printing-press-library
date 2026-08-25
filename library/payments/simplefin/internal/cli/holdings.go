// Copyright 2026 Todd Dailey and contributors. Licensed under Apache-2.0. See LICENSE.
// Absorbed feature: raw investment holdings dump from the local store. For
// gain/loss analysis use 'portfolio'.
//
// pp:data-source local

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/simplefin/internal/simplefin"
)

func newHoldingsCmd(flags *rootFlags) *cobra.Command {
	var dbPath, account string

	cmd := &cobra.Command{
		Use:   "holdings",
		Short: "List investment holdings (symbol, shares, market value) from the local store",
		Long: "Dump investment positions across every brokerage account. For gain/loss versus cost basis\n" +
			"use 'portfolio'.",
		Example:     "  simplefin-pp-cli holdings --json",
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
			hintIfUnsynced(cmd, db, "holdings")

			holdings, err := loadHoldings(ctx, db)
			if err != nil {
				return err
			}
			type hv struct {
				Symbol      string  `json:"symbol"`
				Description string  `json:"description"`
				Account     string  `json:"account"`
				Shares      float64 `json:"shares"`
				MarketValue float64 `json:"market_value"`
				Currency    string  `json:"currency"`
			}
			out := make([]hv, 0, len(holdings))
			for _, h := range holdings {
				if account != "" && !containsAny(lowerName(h.AccountName), lowerName(account)) && h.AccountID != account {
					continue
				}
				mv, _ := simplefin.ParseAmount(h.MarketValue)
				sh, _ := simplefin.ParseAmount(h.Shares)
				out = append(out, hv{Symbol: h.Symbol, Description: h.Description, Account: h.AccountName, Shares: sh, MarketValue: mv, Currency: h.Currency})
			}
			sort.Slice(out, func(i, j int) bool { return out[i].MarketValue > out[j].MarketValue })

			if flags.asJSON || flags.agent || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return flags.printJSON(cmd, out)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no holdings in the local store — run: simplefin-pp-cli sync")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-8s %-26s %12s %14s\n", "symbol", "description", "shares", "value")
			for _, h := range out {
				sym := h.Symbol
				if sym == "" {
					sym = "—"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-8s %-26s %12.4f %14.2f\n", sym, truncate(h.Description, 26), h.Shares, h.MarketValue)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "Filter by account id or name substring")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}
