// Copyright 2026 Kevin Magnan and contributors. Licensed under Apache-2.0. See LICENSE.
//
// brief — the whole pre-open check in one command. The MCP requires four
// separate calls (portfolio, orders, positions, quotes) for what a trader does
// every morning; this joins them, adds the day-over-day delta from the local
// snapshot store (which the API cannot provide), and records a fresh snapshot
// so the portfolio time series keeps growing.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/robinhood-agentic/internal/client"
	"github.com/mvanhorn/printing-press-library/library/payments/robinhood-agentic/internal/store"

	"github.com/spf13/cobra"
)

func newNovelBriefCmd(flags *rootFlags) *cobra.Command {
	var flagAccount string

	cmd := &cobra.Command{
		Use:         "brief",
		Short:       "The whole pre-open check — portfolio value, day-over-day delta, open orders, positions, top movers among holdings, and upcoming earnings — in one command.",
		Example:     "  robinhood-agentic-pp-cli brief --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			account := flagAccount
			if account == "" {
				account, err = resolveDefaultAccount(ctx, c)
				if err != nil {
					return classifyAPIError(err, flags)
				}
			}

			// Authoritative portfolio value (get_accounts buying power is unreliable).
			portfolioRaw, err := c.Get(ctx, "/tools/get_portfolio", map[string]string{"account_number": account}) // pp:client-call
			if err != nil {
				return classifyAPIError(err, flags)
			}
			portfolio := extractData(portfolioRaw)

			// Day-over-day delta from the local snapshot series, then record a
			// new snapshot so history compounds.
			delta := briefDelta(ctx, account, portfolio)
			recordBriefSnapshot(account, portfolio)

			// The remaining sub-sections are best-effort: a fetch failure must
			// not sink the whole brief, but it must NOT be silently swallowed
			// either — an empty section from a real error is indistinguishable
			// from a genuinely empty section unless we say so. Each failure is
			// recorded in an always-present `warnings` field (and echoed to
			// stderr) so a caller can tell "no movers" from "movers failed".
			warnings := []string{}
			note := func(section string, err error) {
				if err != nil {
					warnings = append(warnings, section+": "+err.Error())
				}
			}

			openOrders, err := fetchOpenOrders(ctx, c, account)
			note("open_orders", err)
			positions, err := fetchArray(ctx, c, "/tools/get_equity_positions", map[string]string{"account_number": account}, "positions")
			note("positions", err)

			heldSymbols := positionSymbols(positions)
			movers, err := fetchMovers(ctx, c, heldSymbols)
			note("movers", err)
			earnings, err := fetchHeldEarnings(ctx, c, heldSymbols)
			note("earnings", err)

			result := map[string]any{
				"account":     account,
				"as_of":       time.Now().UTC().Format(time.RFC3339),
				"portfolio":   portfolio,
				"delta":       delta,
				"open_orders": openOrders,
				"positions":   positions,
				"movers":      movers,
				"earnings":    earnings,
				"warnings":    warnings,
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Brief for %s (as of %s)\n", account, time.Now().Format("2006-01-02 15:04"))
			if tv, ok := portfolio["total_value"].(string); ok {
				fmt.Fprintf(w, "  Portfolio value: %s\n", tv)
			}
			if delta != nil {
				fmt.Fprintf(w, "  Change vs last snapshot: %+.2f (%+.2f%%)\n", delta["change"], delta["change_pct"])
			} else {
				fmt.Fprintln(w, "  Change vs last snapshot: (first snapshot — no prior value)")
			}
			fmt.Fprintf(w, "  Open orders: %d\n", len(openOrders))
			fmt.Fprintf(w, "  Positions: %d\n", len(positions))
			for _, m := range movers {
				fmt.Fprintf(w, "  Mover: %s %+.2f%%\n", m.Symbol, m.ChangePct)
			}
			if len(earnings) > 0 {
				fmt.Fprintf(w, "  Upcoming earnings for held symbols: %d\n", len(earnings))
			}
			for _, warn := range warnings {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: brief section unavailable — %s\n", warn)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagAccount, "account", "", "Account number (defaults to the first/default account)")
	return cmd
}

// resolveDefaultAccount picks the default account (or the first) from get_accounts.
func resolveDefaultAccount(ctx context.Context, c *client.Client) (string, error) {
	raw, err := c.Get(ctx, "/tools/get_accounts", nil) // pp:client-call
	if err != nil {
		return "", err
	}
	var env struct {
		Data struct {
			Accounts []accountRef `json:"accounts"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return "", err
	}
	if len(env.Data.Accounts) == 0 {
		return "", fmt.Errorf("no accounts found; run 'auth login' first")
	}
	return pickDefaultAccount(env.Data.Accounts), nil
}

// accountRef is the minimal account shape pickDefaultAccount needs.
type accountRef struct {
	AccountNumber string `json:"account_number"`
	IsDefault     bool   `json:"is_default"`
}

// pickDefaultAccount returns the default account number, or the first one when
// none is flagged default. Pure function for testing.
func pickDefaultAccount(accounts []accountRef) string {
	first := ""
	for i, a := range accounts {
		if i == 0 {
			first = a.AccountNumber
		}
		if a.IsDefault {
			return a.AccountNumber
		}
	}
	return first
}

// briefDelta computes the change from the most recent prior snapshot to the
// current portfolio value. Returns nil when there is no prior snapshot. Opens
// the store read-only — reading the snapshot series must not take the write
// lock.
func briefDelta(ctx context.Context, account string, portfolio map[string]any) map[string]float64 {
	st, err := openStoreForRead(ctx, "robinhood-agentic-pp-cli")
	if err != nil || st == nil {
		return nil
	}
	defer st.Close()
	snaps, err := st.PortfolioSnapshots(account, time.Now().Add(-365*24*time.Hour))
	if err != nil || len(snaps) == 0 {
		return nil
	}
	prior := parseMoney(snaps[len(snaps)-1].TotalValue)
	current := parseMoney(asString(portfolio["total_value"]))
	return computeDelta(prior, current)
}

// computeDelta returns absolute and percentage change. Pure function.
func computeDelta(prior, current float64) map[string]float64 {
	change := current - prior
	pct := 0.0
	if prior != 0 {
		pct = change / prior * 100
	}
	return map[string]float64{"prior": prior, "current": current, "change": change, "change_pct": pct}
}

func recordBriefSnapshot(account string, portfolio map[string]any) {
	st, err := store.Open(defaultDBPath("robinhood-agentic-pp-cli"))
	if err != nil {
		return
	}
	defer st.Close()
	raw, err := json.Marshal(portfolio)
	if err != nil {
		return
	}
	_ = st.RecordPortfolioSnapshot(account, raw)
}

// fetchOpenOrders returns in-flight equity orders. A fetch error is returned to
// the caller (which records it as a warning) rather than swallowed.
func fetchOpenOrders(ctx context.Context, c *client.Client, account string) ([]map[string]any, error) {
	all, err := fetchArray(ctx, c, "/tools/get_equity_orders", map[string]string{"account_number": account}, "orders")
	if err != nil {
		return nil, err
	}
	var open []map[string]any
	for _, o := range all {
		state, _ := o["state"].(string)
		if isOpenOrderState(state) {
			open = append(open, o)
		}
	}
	return open, nil
}

// isOpenOrderState reports whether an order state is still in flight. Pure.
func isOpenOrderState(state string) bool {
	switch state {
	case "new", "queued", "confirmed", "unconfirmed", "partially_filled":
		return true
	default:
		return false
	}
}

// fetchArray fetches a tool and returns the array at data.<key>. A transport
// error is returned so the caller can surface it; a well-formed response that
// simply lacks the key returns an empty slice with no error (a genuinely empty
// section is not a failure).
func fetchArray(ctx context.Context, c *client.Client, path string, params map[string]string, key string) ([]map[string]any, error) {
	raw, err := c.Get(ctx, path, params) // pp:client-call
	if err != nil {
		return nil, err
	}
	var env map[string]json.RawMessage
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("parsing %s response: %w", key, err)
	}
	data, ok := env["data"]
	if !ok {
		return nil, nil
	}
	var inner map[string]json.RawMessage
	if err := json.Unmarshal(data, &inner); err != nil {
		return nil, fmt.Errorf("parsing %s data: %w", key, err)
	}
	arrRaw, ok := inner[key]
	if !ok {
		return nil, nil
	}
	var arr []map[string]any
	_ = json.Unmarshal(arrRaw, &arr)
	return arr, nil
}

// mover is a held symbol ranked by its intraday quote percentage change.
type mover struct {
	Symbol    string  `json:"symbol"`
	ChangePct float64 `json:"change_pct"`
	Last      string  `json:"last_trade_price"`
}

// positionSymbols pulls the distinct symbols out of a positions array.
func positionSymbols(positions []map[string]any) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range positions {
		if s, _ := p["symbol"].(string); s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// fetchMovers quotes the held symbols and returns the top movers by absolute
// intraday percentage change. Returns (nil, nil) when there are no symbols; a
// fetch error is returned so the caller can record it as a warning.
func fetchMovers(ctx context.Context, c *client.Client, symbols []string) ([]mover, error) {
	if len(symbols) == 0 {
		return nil, nil
	}
	// The quotes tool caps at 20 symbols; brief only ever quotes held symbols,
	// which is well under that for any realistic portfolio, but cap to be safe.
	if len(symbols) > 20 {
		symbols = symbols[:20]
	}
	raw, err := c.Get(ctx, "/tools/get_equity_quotes", map[string]string{"symbols": strings.Join(symbols, ",")}) // pp:client-call
	if err != nil {
		return nil, err
	}
	quotes := fetchArrayFromRaw(raw, "results")
	return topMovers(quotes, 5), nil
}

// topMovers computes percentage change from each quote and returns the top n by
// absolute change, descending. get_equity_quotes nests the price fields under a
// `quote` object (with a separate `close` block); read from there, falling back
// to a flat row shape for robustness. Pure function for testing.
func topMovers(quotes []map[string]any, n int) []mover {
	var movers []mover
	for _, row := range quotes {
		q := row
		if nested, ok := row["quote"].(map[string]any); ok {
			q = nested
		}
		last := parseMoney(asString(q["last_trade_price"]))
		prev := parseMoney(asString(q["previous_close"]))
		if prev == 0 {
			continue
		}
		movers = append(movers, mover{
			Symbol:    asString(q["symbol"]),
			ChangePct: (last - prev) / prev * 100,
			Last:      asString(q["last_trade_price"]),
		})
	}
	// Sort by absolute change desc, with symbol as a stable tiebreaker so the
	// output is deterministic across identical runs (equal-magnitude movers
	// must not reorder run-to-run).
	sort.Slice(movers, func(i, j int) bool {
		ai, aj := abs(movers[i].ChangePct), abs(movers[j].ChangePct)
		if ai != aj {
			return ai > aj
		}
		return movers[i].Symbol < movers[j].Symbol
	})
	if len(movers) > n {
		movers = movers[:n]
	}
	return movers
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

// fetchHeldEarnings returns upcoming earnings events (next 14 days) for held
// symbols. Returns (nil, nil) with no symbols; a fetch error is returned so the
// caller can record it as a warning.
func fetchHeldEarnings(ctx context.Context, c *client.Client, symbols []string) ([]map[string]any, error) {
	if len(symbols) == 0 {
		return nil, nil
	}
	raw, err := c.Get(ctx, "/tools/get_earnings_calendar", map[string]string{"days": "14"}) // pp:client-call
	if err != nil {
		return nil, err
	}
	events := fetchArrayFromRaw(raw, "events")
	held := map[string]bool{}
	for _, s := range symbols {
		held[strings.ToUpper(s)] = true
	}
	var out []map[string]any
	for _, e := range events {
		if held[strings.ToUpper(asString(e["symbol"]))] {
			out = append(out, e)
		}
	}
	return out, nil
}

// extractData unwraps the {data: {...}} envelope to the inner object.
func extractData(raw json.RawMessage) map[string]any {
	var env struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil || env.Data == nil {
		return map[string]any{}
	}
	return env.Data
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}
