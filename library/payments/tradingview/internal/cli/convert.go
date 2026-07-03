// Copyright 2026 jbriaux and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: fiat/crypto conversion using TradingView forex rates.
// pp:data-source live — forex rates are fetched from the TradingView API at
// call time; this command does not read the local store.

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

type convertView struct {
	Amount float64 `json:"amount"`
	From   string  `json:"from"`
	To     string  `json:"to"`
	Rate   float64 `json:"rate"`
	Result float64 `json:"result"`
	Via    string  `json:"via,omitempty"`
}

func newNovelConvertCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "convert <amount> <from> <to>",
		Short: "Convert an amount between currencies using TradingView's own live forex rates.",
		Long: "Convert an amount from one currency to another using TradingView's FX_IDC forex\n" +
			"symbols as the rate source (the same origin that prices instruments in 'quote').\n" +
			"Example: convert 100 USD EUR. USD-pegged stablecoins (USDT/USDC) are treated as USD.",
		Example:     "  tradingview-pp-cli convert 100 USD EUR --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "amount=100;from=USD;to=EUR"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) < 3 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("convert requires <amount> <from> <to>, e.g. 'convert 100 USD EUR'"))
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would convert using TradingView forex rates")
				return nil
			}

			amount, perr := strconv.ParseFloat(args[0], 64)
			if perr != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("amount %q is not a number", args[0]))
			}
			from := strings.ToUpper(args[1])
			to := strings.ToUpper(args[2])

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			rate, via, ok, err := fetchForexRate(ctx, c, from, to)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if !ok {
				return fmt.Errorf("could not resolve a %s->%s forex rate on TradingView", from, to)
			}

			view := convertView{
				Amount: amount,
				From:   from,
				To:     to,
				Rate:   rate,
				Result: amount * rate,
				Via:    via,
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s %s = %s %s\n", fmtNum(view.Amount), view.From, fmtNum(view.Result), view.To)
			fmt.Fprintf(out, "  rate: 1 %s = %s %s  (source: %s)\n", view.From, fmtNum(view.Rate), view.To, view.Via)
			return nil
		},
	}
	return cmd
}
