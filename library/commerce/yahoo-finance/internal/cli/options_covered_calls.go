// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: options covered-calls. Scans portfolio lots with >=100
// shares and ranks call contracts by annualized yield.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/yahoo-finance/internal/optionsmath"
	"github.com/spf13/cobra"
)

type coveredCallRow struct {
	Symbol          string  `json:"symbol"`
	Shares          float64 `json:"shares"`
	Spot            float64 `json:"spot"`
	Strike          float64 `json:"strike"`
	Expiration      string  `json:"expiration"`
	DTE             int     `json:"dte"`
	CallBid         float64 `json:"call_bid"`
	AnnualizedYield float64 `json:"annualized_yield"`
}

func newOptionsCoveredCallsCmd(flags *rootFlags) *cobra.Command {
	var minYield float64
	var maxDTE int
	var symbolFilter string
	var dbPath string
	cmd := &cobra.Command{
		Use:         "options-covered-calls",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Scan portfolio lots (>=100 shares) for covered-call candidates ranked by annualized yield",
		Long: strings.Trim(`
For each portfolio lot with at least 100 shares, fetch the live options
chain and surface call contracts whose annualized yield ((bid/spot)*(365/dte))
meets --min-yield-annualized within --max-dte days.
`, "\n"),
		Example: strings.Trim(`
  yahoo-finance-pp-cli options-covered-calls --min-yield-annualized 0.08 --max-dte 60
  yahoo-finance-pp-cli options-covered-calls --symbol AAPL --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openDividendsDB(flags, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			// Aggregate shares per symbol.
			lotQuery := `SELECT symbol, SUM(shares) FROM portfolio_lots`
			var lotArgs []any
			if symbolFilter != "" {
				lotQuery += ` WHERE symbol = ?`
				lotArgs = append(lotArgs, strings.ToUpper(symbolFilter))
			}
			lotQuery += ` GROUP BY symbol HAVING SUM(shares) >= 100 ORDER BY symbol`
			rows, err := db.Query(lotQuery, lotArgs...)
			if err != nil {
				return fmt.Errorf("querying portfolio_lots: %w", err)
			}
			defer rows.Close()
			type pos struct {
				symbol string
				shares float64
			}
			var positions []pos
			for rows.Next() {
				var p pos
				if err := rows.Scan(&p.symbol, &p.shares); err != nil {
					return err
				}
				positions = append(positions, p)
			}
			if len(positions) == 0 {
				if flags.asJSON {
					return flags.printJSON(cmd, []coveredCallRow{})
				}
				fmt.Fprintln(cmd.OutOrStdout(), "(no lots with >=100 shares — covered calls require 100-share blocks)")
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var out []coveredCallRow
			for _, p := range positions {
				chain, err := c.Get(cmd.Context(), "/v7/finance/options/"+p.symbol, nil)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: options fetch failed for %s: %v\n", p.symbol, err)
					continue
				}
				rows := parseCoveredCalls(chain, p.symbol, p.shares, maxDTE, minYield)
				out = append(out, rows...)
			}

			// Rank descending by annualized_yield.
			sort.SliceStable(out, func(i, j int) bool { return out[i].AnnualizedYield > out[j].AnnualizedYield })

			if flags.asJSON {
				return flags.printJSON(cmd, out)
			}
			headers := []string{"SYMBOL", "STRIKE", "EXP", "DTE", "BID", "ANN_YIELD"}
			table := make([][]string, 0, len(out))
			for _, r := range out {
				table = append(table, []string{
					r.Symbol,
					fmt.Sprintf("%.2f", r.Strike),
					r.Expiration,
					fmt.Sprintf("%d", r.DTE),
					fmt.Sprintf("%.2f", r.CallBid),
					fmt.Sprintf("%.4f", r.AnnualizedYield),
				})
			}
			return flags.printTable(cmd, headers, table)
		},
	}
	cmd.Flags().Float64Var(&minYield, "min-yield-annualized", 0.08, "Minimum annualized yield to surface (decimal)")
	cmd.Flags().IntVar(&maxDTE, "max-dte", 60, "Maximum days-to-expiration")
	cmd.Flags().StringVar(&symbolFilter, "symbol", "", "Limit to one symbol")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override local SQLite path")
	return cmd
}

// parseCoveredCalls reads Yahoo's options-chain envelope and emits one
// coveredCallRow per qualifying call contract. Filters by maxDTE and
// minYield. Spot price is taken from quote.regularMarketPrice.
func parseCoveredCalls(raw json.RawMessage, symbol string, shares float64, maxDTE int, minYield float64) []coveredCallRow {
	var env struct {
		OptionChain struct {
			Result []struct {
				UnderlyingSymbol string `json:"underlyingSymbol"`
				Quote            struct {
					RegularMarketPrice float64 `json:"regularMarketPrice"`
				} `json:"quote"`
				Options []struct {
					ExpirationDate int64 `json:"expirationDate"`
					Calls          []struct {
						Strike     float64 `json:"strike"`
						Bid        float64 `json:"bid"`
						LastPrice  float64 `json:"lastPrice"`
						Expiration int64   `json:"expiration"`
					} `json:"calls"`
				} `json:"options"`
			} `json:"result"`
		} `json:"optionChain"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil
	}
	now := time.Now()
	var out []coveredCallRow
	for _, r := range env.OptionChain.Result {
		spot := r.Quote.RegularMarketPrice
		for _, o := range r.Options {
			expTS := o.ExpirationDate
			exp := time.Unix(expTS, 0)
			dte := int(exp.Sub(now).Hours() / 24)
			if dte <= 0 || dte > maxDTE {
				continue
			}
			for _, call := range o.Calls {
				bid := call.Bid
				if bid <= 0 {
					bid = call.LastPrice
				}
				if bid <= 0 {
					continue
				}
				y := optionsmath.AnnualizedYield(bid, spot, dte)
				if y < minYield {
					continue
				}
				out = append(out, coveredCallRow{
					Symbol:          symbol,
					Shares:          shares,
					Spot:            spot,
					Strike:          call.Strike,
					Expiration:      exp.UTC().Format("2006-01-02"),
					DTE:             dte,
					CallBid:         bid,
					AnnualizedYield: y,
				})
			}
		}
	}
	return out
}
