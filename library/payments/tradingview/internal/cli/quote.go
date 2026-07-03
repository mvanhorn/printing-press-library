// Copyright 2026 jbriaux and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: multi-currency quote (native + USD + EUR).
// pp:data-source live — prices and forex rates are fetched from the
// TradingView API at call time; this command does not read the local store.

package cli

import (
	"context"
	"fmt"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/payments/tradingview/internal/client"
	"github.com/spf13/cobra"
)

type quoteView struct {
	Symbol      string  `json:"symbol"`
	Description string  `json:"description"`
	Type        string  `json:"type"`
	Price       float64 `json:"price"`
	Currency    string  `json:"currency"`
	PriceUSD    float64 `json:"price_usd"`
	PriceEUR    float64 `json:"price_eur"`
	EURUSD      float64 `json:"eurusd_rate"`
	ChangePct   float64 `json:"change_pct"`
	Note        string  `json:"note,omitempty"`
}

func newNovelQuoteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "quote <symbol>",
		Short: "See any ticker's last price in its native currency, USD, and EUR in a single command.",
		Long: "Fetch the last price for a symbol and show it in its native currency, USD, and EUR.\n" +
			"Accepts a fully-qualified EXCHANGE:TICKER (e.g. NASDAQ:AAPL) or a bare ticker\n" +
			"(e.g. AAPL), which is auto-resolved via symbol search. USD-pegged crypto quote\n" +
			"currencies (USDT/USDC) are treated as USD. EUR is derived from TradingView's\n" +
			"FX_IDC:EURUSD forex rate.",
		Example:     "  tradingview-pp-cli quote NASDAQ:AAPL --agent --select price_usd,price_eur",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "symbol=NASDAQ:AAPL"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch quote from TradingView")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a symbol is required, e.g. 'quote NASDAQ:AAPL' or 'quote AAPL'"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			fq, _, err := resolveSymbol(ctx, c, args[0], "")
			if err != nil {
				return err
			}

			view, found, err := buildQuoteView(ctx, c, fq)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if !found {
				return fmt.Errorf("no price available for %q (try 'search %s' for the exact ticker)", fq, args[0])
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}

			// Human-readable output.
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s  %s\n", view.Symbol, view.Description)
			fmt.Fprintf(out, "  Price:  %s %s", fmtNum(view.Price), view.Currency)
			if view.ChangePct != 0 {
				fmt.Fprintf(out, "  (%+.2f%% today)", view.ChangePct)
			}
			fmt.Fprintln(out)
			if view.PriceUSD != 0 {
				fmt.Fprintf(out, "  USD:    $%s\n", fmtNum(view.PriceUSD))
			}
			if view.PriceEUR != 0 {
				fmt.Fprintf(out, "  EUR:    EUR %s  (EURUSD %s)\n", fmtNum(view.PriceEUR), fmtNum(view.EURUSD))
			}
			if view.Note != "" {
				fmt.Fprintf(out, "  note:   %s\n", view.Note)
			}
			return nil
		},
	}
	return cmd
}

// buildQuoteView fetches a fully-qualified symbol's price and normalizes it
// into native currency, USD, and EUR. Shared by the quote command and the
// watchlist sync/quotes commands so the multi-currency logic lives in one
// place. Returns (view, found, error); found is false when the symbol has no
// price.
func buildQuoteView(ctx context.Context, c *client.Client, fq string) (quoteView, bool, error) {
	q, found, err := fetchRawQuote(ctx, c, fq)
	if err != nil {
		return quoteView{}, false, err
	}
	if !found {
		return quoteView{}, false, nil
	}

	view := quoteView{
		Symbol:      fq,
		Description: q.Description,
		Type:        q.Type,
		Price:       q.Close,
		Currency:    q.Currency,
		ChangePct:   q.Change,
	}

	// Native price -> USD.
	if isUSDLike(q.Currency) {
		view.PriceUSD = q.Close
		if q.Currency != "USD" {
			view.Note = q.Currency + " treated as USD-pegged for USD/EUR conversion"
		}
	} else if q.Currency != "" {
		rate, _, ok, rerr := fetchForexRate(ctx, c, q.Currency, "USD")
		if rerr != nil {
			return quoteView{}, false, rerr
		}
		if ok {
			view.PriceUSD = q.Close * rate
		} else {
			view.Note = "could not resolve a " + q.Currency + "->USD forex rate; USD/EUR omitted"
		}
	}

	// USD -> EUR via EURUSD.
	if view.PriceUSD != 0 {
		eur, ok, rerr := fetchRawQuote(ctx, c, "FX_IDC:EURUSD")
		if rerr != nil {
			return quoteView{}, false, rerr
		}
		if ok && eur.Close > 0 {
			view.EURUSD = eur.Close
			view.PriceEUR = view.PriceUSD / eur.Close
		}
	}
	return view, true, nil
}

// fmtNum renders a float with a minimal, human-friendly representation.
func fmtNum(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
