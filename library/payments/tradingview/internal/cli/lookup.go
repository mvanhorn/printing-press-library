// Copyright 2026 jbriaux and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: resolve a query to TradingView symbol candidates.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelSearchCmd(flags *rootFlags) *cobra.Command {
	var typeFilter string
	var exchange string
	var limit int
	cmd := &cobra.Command{
		Use:     "search <query>",
		Aliases: []string{"lookup"},
		Short:   "Search TradingView symbols: resolve a name or ticker to fully-qualified EXCHANGE:TICKER candidates.",
		Long: "Search TradingView's symbol universe and list candidates with their\n" +
			"fully-qualified EXCHANGE:TICKER, instrument type, exchange, and quote currency.\n" +
			"Use the resulting symbol with 'quote'. Filter with --type (stock, crypto, forex,\n" +
			"index, fund) and --exchange. Aliased as 'lookup'.",
		Example:     "  tradingview-pp-cli search bitcoin --type crypto",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "query=AAPL"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search TradingView symbols")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search query is required, e.g. 'search AAPL'"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			query := strings.Join(args, " ")
			matches, err := searchSymbols(ctx, c, query, typeFilter, exchange, limit)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), matches, flags)
			}

			out := cmd.OutOrStdout()
			if len(matches) == 0 {
				fmt.Fprintf(out, "no symbols found for %q\n", query)
				return nil
			}
			for _, m := range matches {
				marker := " "
				if m.Primary {
					marker = "*"
				}
				fmt.Fprintf(out, "%s %-24s %-6s %-8s %s\n", marker, m.Symbol, m.Type, m.Currency, m.Description)
			}
			fmt.Fprintln(out, "\n(* = primary listing; use the symbol with 'quote')")
			return nil
		},
	}
	cmd.Flags().StringVar(&typeFilter, "type", "", "Restrict by instrument type: stock, crypto, forex, index, fund")
	cmd.Flags().StringVar(&exchange, "exchange", "", "Restrict to an exchange, e.g. NASDAQ, BINANCE")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum candidates to return")
	return cmd
}
