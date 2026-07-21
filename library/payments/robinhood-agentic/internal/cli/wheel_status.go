// Copyright 2026 Kevin Magnan and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: wheel status. Infers each underlying's wheel-strategy stage
// by joining equity positions, option positions, and option orders. The MCP
// exposes no assignment/exercise tool, so the stage is inferred client-side.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// wheelRow is the per-underlying wheel snapshot rendered by `wheel status`.
type wheelRow struct {
	Symbol           string  `json:"symbol"`
	Stage            string  `json:"stage"`
	Shares           float64 `json:"shares"`
	ShortPuts        int     `json:"short_puts"`
	ShortCalls       int     `json:"short_calls"`
	NextExpiration   string  `json:"next_expiration,omitempty"`
	OpenOptionOrders int     `json:"open_option_orders,omitempty"`
}

// wheelStatusOutput is the JSON envelope for `wheel status`.
type wheelStatusOutput struct {
	Account   string     `json:"account"`
	Symbol    string     `json:"symbol,omitempty"`
	Positions []wheelRow `json:"positions"`
}

// wheelEquityPosition is a defensively parsed equity position.
type wheelEquityPosition struct {
	Symbol   string
	Quantity float64
}

// wheelOptionPosition is a defensively parsed option position.
type wheelOptionPosition struct {
	Symbol     string // underlying (chain) symbol
	Direction  string // "long", "short", or "" when the feed omitted it
	OptionType string // "call", "put", or "" when the feed omitted it
	Contracts  int    // absolute contract count, minimum 1 for open positions
	Expiration string // e.g. "2026-08-21", "" when omitted
}

// inferWheelStage classifies one underlying's position in the wheel cycle.
// Pure: all live-data joining happens before this is called.
//
//	cash_secured_put    short put(s) open, no shares held
//	assigned_holding    shares held, no short call written against them
//	covered_call        shares held plus short call(s)
//	called_away_or_idle no shares and no wheel-relevant short exposure that a
//	                    put would explain (includes a lingering naked short
//	                    call right after shares were called away, and an
//	                    equity record whose quantity has gone to zero)
//	long_option         the symbol only shows up via long option positions
func inferWheelStage(hasEquity bool, equityQty float64, shortPuts int, shortCalls int) string {
	hasShares := hasEquity && equityQty > 0
	switch {
	case hasShares && shortCalls > 0:
		return "covered_call"
	case hasShares:
		return "assigned_holding"
	case shortPuts > 0:
		return "cash_secured_put"
	case shortCalls > 0:
		// A short call with no shares: most likely the shares were just
		// called away while the (now naked) call is still on the books.
		return "called_away_or_idle"
	case hasEquity:
		// Equity record exists but quantity is zero (or negative) and no
		// short options remain: sold, called away, or simply idle.
		return "called_away_or_idle"
	default:
		// No equity record and no short options: the symbol only entered
		// the join via long option positions.
		return "long_option"
	}
}

// buildWheelRows joins equity and option positions (plus open-order counts as
// context) into per-symbol wheel rows, sorted by symbol. symbolFilter, when
// non-empty, restricts the join to that underlying (case-insensitive). Pure.
func buildWheelRows(equity []wheelEquityPosition, options []wheelOptionPosition, openOrders map[string]int, symbolFilter string) []wheelRow {
	filter := strings.ToUpper(strings.TrimSpace(symbolFilter))
	type wheelAgg struct {
		hasEquity    bool
		shares       float64
		shortPuts    int
		shortCalls   int
		shortUnknown int
		nextExp      string
	}
	bySymbol := map[string]*wheelAgg{}
	get := func(symbol string) *wheelAgg {
		symbol = strings.ToUpper(symbol)
		a, ok := bySymbol[symbol]
		if !ok {
			a = &wheelAgg{}
			bySymbol[symbol] = a
		}
		return a
	}

	for _, p := range equity {
		sym := strings.ToUpper(strings.TrimSpace(p.Symbol))
		if sym == "" || (filter != "" && sym != filter) {
			continue
		}
		a := get(sym)
		a.hasEquity = true
		a.shares += p.Quantity
	}

	for _, o := range options {
		sym := strings.ToUpper(strings.TrimSpace(o.Symbol))
		if sym == "" || (filter != "" && sym != filter) {
			continue
		}
		a := get(sym)
		n := o.Contracts
		if n <= 0 {
			n = 1
		}
		switch {
		case o.Direction == "short" && o.OptionType == "put":
			a.shortPuts += n
		case o.Direction == "short" && o.OptionType == "call":
			a.shortCalls += n
		case o.Direction == "short":
			a.shortUnknown += n
		default:
			// Long (or unknown-direction, non-negative-quantity) options
			// only mark presence; the default stage branch handles them.
		}
		if o.Expiration != "" && (a.nextExp == "" || o.Expiration < a.nextExp) {
			a.nextExp = o.Expiration
		}
	}

	symbols := make([]string, 0, len(bySymbol))
	for s := range bySymbol {
		symbols = append(symbols, s)
	}
	sort.Strings(symbols)

	rows := make([]wheelRow, 0, len(symbols))
	for _, s := range symbols {
		a := bySymbol[s]
		shortPuts, shortCalls := a.shortPuts, a.shortCalls
		if a.shortUnknown > 0 {
			// The feed omitted call/put for some short contracts. Guess the
			// wheel-consistent leg: shorts written against held shares are
			// calls; shorts without shares are (cash-secured) puts.
			if a.hasEquity && a.shares > 0 {
				shortCalls += a.shortUnknown
			} else {
				shortPuts += a.shortUnknown
			}
		}
		rows = append(rows, wheelRow{
			Symbol:           s,
			Stage:            inferWheelStage(a.hasEquity, a.shares, shortPuts, shortCalls),
			Shares:           a.shares,
			ShortPuts:        shortPuts,
			ShortCalls:       shortCalls,
			NextExpiration:   a.nextExp,
			OpenOptionOrders: openOrders[s],
		})
	}
	return rows
}

// wheelFieldString returns the first non-empty string value found under keys.
func wheelFieldString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

// wheelFieldFloat returns the first parseable numeric value found under keys.
// Robinhood feeds return quantities both as JSON numbers and as strings.
func wheelFieldFloat(m map[string]any, keys ...string) (float64, bool) {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		switch t := v.(type) {
		case float64:
			return t, true
		case json.Number:
			if f, err := t.Float64(); err == nil {
				return f, true
			}
		case string:
			if f, err := strconv.ParseFloat(strings.TrimSpace(t), 64); err == nil {
				return f, true
			}
		}
	}
	return 0, false
}

// wheelParseEquityPositions converts raw get_equity_positions items.
func wheelParseEquityPositions(raw []map[string]any) []wheelEquityPosition {
	out := make([]wheelEquityPosition, 0, len(raw))
	for _, m := range raw {
		sym := wheelFieldString(m, "symbol", "chain_symbol")
		if sym == "" {
			continue
		}
		qty, _ := wheelFieldFloat(m, "quantity", "shares")
		out = append(out, wheelEquityPosition{Symbol: sym, Quantity: qty})
	}
	return out
}

// wheelParseOptionPositions converts raw get_option_positions items, reading
// field names defensively: direction may live in "type"/"position_type",
// call-vs-put in "option_type"/"contract_type" (and tolerated missing).
func wheelParseOptionPositions(raw []map[string]any) []wheelOptionPosition {
	out := make([]wheelOptionPosition, 0, len(raw))
	for _, m := range raw {
		sym := wheelFieldString(m, "chain_symbol", "symbol", "underlying_symbol")
		if sym == "" {
			continue
		}
		direction := ""
		switch strings.ToLower(wheelFieldString(m, "type", "position_type", "direction")) {
		case "long":
			direction = "long"
		case "short":
			direction = "short"
		}
		optionType := ""
		switch strings.ToLower(wheelFieldString(m, "option_type", "contract_type", "kind")) {
		case "call":
			optionType = "call"
		case "put":
			optionType = "put"
		}
		// Some schemas overload "type" with call/put instead of long/short.
		if optionType == "" {
			switch strings.ToLower(wheelFieldString(m, "type")) {
			case "call":
				optionType = "call"
			case "put":
				optionType = "put"
			}
		}
		qty, hasQty := wheelFieldFloat(m, "quantity", "contracts")
		if !hasQty {
			// No quantity or contracts field on this row: we cannot size the
			// position. Skip it rather than default to one contract, which
			// would fabricate a short put or covered call and mislabel the
			// symbol's wheel stage.
			continue
		}
		if qty == 0 {
			continue // closed position; nonzero:true should exclude these anyway
		}
		if direction == "" && qty < 0 {
			direction = "short"
		}
		magnitude := qty
		if magnitude < 0 {
			magnitude = -magnitude
		}
		contracts := int(magnitude + 0.5)
		if contracts == 0 {
			continue // sub-one magnitude rounds to zero; not a real position
		}
		out = append(out, wheelOptionPosition{
			Symbol:     sym,
			Direction:  direction,
			OptionType: optionType,
			Contracts:  contracts,
			Expiration: wheelFieldString(m, "expiration_date", "expiration", "exp_date"),
		})
	}
	return out
}

// wheelCountOpenOrders counts non-terminal option orders per underlying.
func wheelCountOpenOrders(raw []map[string]any) map[string]int {
	terminal := map[string]bool{
		"filled": true, "cancelled": true, "canceled": true,
		"rejected": true, "failed": true, "expired": true, "voided": true,
	}
	counts := map[string]int{}
	for _, m := range raw {
		sym := strings.ToUpper(wheelFieldString(m, "chain_symbol", "symbol", "underlying_symbol"))
		if sym == "" {
			continue
		}
		state := strings.ToLower(wheelFieldString(m, "state", "status"))
		if state == "" || terminal[state] {
			continue
		}
		counts[sym]++
	}
	return counts
}

// wheelFormatShares renders a share quantity without trailing zeros.
func wheelFormatShares(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func newNovelWheelStatusCmd(flags *rootFlags) *cobra.Command {
	var flagAccount string

	cmd := &cobra.Command{
		Use:         "status [SYMBOL]",
		Short:       "Per-symbol wheel-strategy stage (cash-secured put → assigned → covered call → called away) inferred automatically.",
		Example:     "  robinhood-agentic-pp-cli wheel status --account RH123456\n  robinhood-agentic-pp-cli wheel status --account RH123456 AAPL",
		Args:        cobra.MaximumNArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			symbol := ""
			if len(args) == 1 {
				symbol = strings.ToUpper(strings.TrimSpace(args[0]))
			}
			if strings.TrimSpace(flagAccount) == "" && !flags.dryRun {
				return usageErr(fmt.Errorf("required flag --account not set; usage: %s --account <ACCOUNT_NUMBER> [SYMBOL]", cmd.CommandPath()))
			}
			if dryRunOK(flags) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Live read 1: equity positions (shares per symbol).
			eqRaw, err := c.Get(cmd.Context(), "/tools/get_equity_positions", map[string]string{
				"account_number": flagAccount,
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var eqEnv struct {
				Data struct {
					Positions []map[string]any `json:"positions"`
				} `json:"data"`
			}
			if err := json.Unmarshal(eqRaw, &eqEnv); err != nil {
				return fmt.Errorf("decode get_equity_positions response: %w", err)
			}

			// Live read 2: open option positions (short puts/calls, expirations).
			optRaw, err := c.Get(cmd.Context(), "/tools/get_option_positions", map[string]string{
				"account_number": flagAccount,
				"nonzero":        "true",
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var optEnv struct {
				Data struct {
					Positions []map[string]any `json:"positions"`
				} `json:"data"`
			}
			if err := json.Unmarshal(optRaw, &optEnv); err != nil {
				return fmt.Errorf("decode get_option_positions response: %w", err)
			}

			// Live read 3: recent option orders (optional context). Degrade
			// gracefully — the join still works without order activity.
			openOrders := map[string]int{}
			if ordRaw, ordErr := c.Get(cmd.Context(), "/tools/get_option_orders", map[string]string{
				"account_number": flagAccount,
			}); ordErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: get_option_orders unavailable (%v); open-order context omitted\n", ordErr)
			} else {
				var ordEnv struct {
					Data struct {
						Orders []map[string]any `json:"orders"`
					} `json:"data"`
				}
				if err := json.Unmarshal(ordRaw, &ordEnv); err == nil {
					openOrders = wheelCountOpenOrders(ordEnv.Data.Orders)
				}
			}

			rows := buildWheelRows(
				wheelParseEquityPositions(eqEnv.Data.Positions),
				wheelParseOptionPositions(optEnv.Data.Positions),
				openOrders,
				symbol,
			)

			out := wheelStatusOutput{Account: flagAccount, Symbol: symbol, Positions: rows}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			w := cmd.OutOrStdout()
			if len(rows) == 0 {
				if symbol != "" {
					fmt.Fprintf(w, "No equity or option positions for %s in account %s.\n", symbol, flagAccount)
				} else {
					fmt.Fprintf(w, "No equity or option positions in account %s.\n", flagAccount)
				}
				return nil
			}
			tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "SYMBOL\tSTAGE\tSHARES\tSHORT_PUTS\tSHORT_CALLS")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%d\n", r.Symbol, r.Stage, wheelFormatShares(r.Shares), r.ShortPuts, r.ShortCalls)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&flagAccount, "account", "", "Account number (required)")
	return cmd
}
