// Copyright 2026 Kevin Magnan and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: `portfolio winrate`.
//
// The MCP has no win-rate endpoint, so this command fetches the raw
// trade-by-trade realized P&L history (get_pnl_trade_history) live and
// aggregates it LOCALLY: round trips, wins, losses, win rate, average
// win/loss, and total realized P&L, with an optional per-symbol breakdown.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// winTrade is the minimal local view of one CLOSING trade: only trades
// that carry a realized_pnl value ever become a winTrade (opening legs
// are filtered out before aggregation).
type winTrade struct {
	Symbol      string
	RealizedPnl float64
}

// winrateBucket is one aggregation bucket (overall or per symbol).
// Zero-P&L round trips count in TotalTrades but are neither wins nor
// losses, so WinRate = Wins / (Wins + Losses) skips breakevens.
type winrateBucket struct {
	TotalTrades   int     `json:"total_trades"`
	Wins          int     `json:"wins"`
	Losses        int     `json:"losses"`
	WinRate       float64 `json:"win_rate"`
	AvgWin        float64 `json:"avg_win"`
	AvgLoss       float64 `json:"avg_loss"`
	TotalRealized float64 `json:"total_realized"`
}

// winrateStats is the full local aggregation: the overall bucket plus a
// per-symbol breakdown (always computed; output only with --by-symbol).
type winrateStats struct {
	Overall  winrateBucket
	BySymbol map[string]winrateBucket
}

// computeWinrate aggregates closing trades into overall and per-symbol
// win-rate stats. Pure: no IO, deterministic for a given input.
func computeWinrate(trades []winTrade) winrateStats {
	bySym := map[string][]winTrade{}
	for _, t := range trades {
		bySym[t.Symbol] = append(bySym[t.Symbol], t)
	}
	stats := winrateStats{
		Overall:  winrateBucketStats(trades),
		BySymbol: make(map[string]winrateBucket, len(bySym)),
	}
	for sym, ts := range bySym {
		stats.BySymbol[sym] = winrateBucketStats(ts)
	}
	return stats
}

// winrateBucketStats folds one slice of closing trades into a bucket.
func winrateBucketStats(trades []winTrade) winrateBucket {
	var b winrateBucket
	var winSum, lossSum float64
	for _, t := range trades {
		b.TotalTrades++
		b.TotalRealized += t.RealizedPnl
		switch {
		case t.RealizedPnl > 0:
			b.Wins++
			winSum += t.RealizedPnl
		case t.RealizedPnl < 0:
			b.Losses++
			lossSum += t.RealizedPnl
		}
		// Exactly zero: a breakeven round trip. Counted in TotalTrades
		// and TotalRealized only.
	}
	if decided := b.Wins + b.Losses; decided > 0 {
		b.WinRate = float64(b.Wins) / float64(decided)
	}
	if b.Wins > 0 {
		b.AvgWin = winSum / float64(b.Wins)
	}
	if b.Losses > 0 {
		b.AvgLoss = lossSum / float64(b.Losses)
	}
	return b
}

// winratePct renders a bucket's win rate for humans; "n/a" when no
// decided (non-breakeven) round trips exist.
func winratePct(b winrateBucket) string {
	if b.Wins+b.Losses == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.1f%%", b.WinRate*100)
}

// winrateOutput is the JSON contract for `portfolio winrate`.
type winrateOutput struct {
	Account       string                   `json:"account"`
	Span          string                   `json:"span"`
	TotalTrades   int                      `json:"total_trades"`
	Wins          int                      `json:"wins"`
	Losses        int                      `json:"losses"`
	WinRate       float64                  `json:"win_rate"`
	AvgWin        float64                  `json:"avg_win"`
	AvgLoss       float64                  `json:"avg_loss"`
	TotalRealized float64                  `json:"total_realized"`
	BySymbol      map[string]winrateBucket `json:"by_symbol,omitempty"`
}

func newNovelPortfolioWinrateCmd(flags *rootFlags) *cobra.Command {
	var flagBySymbol bool
	var flagAccount string
	var flagSpan string

	cmd := &cobra.Command{
		Use:         "winrate",
		Short:       "Round-trip win rate, average win/loss, and per-symbol stats computed from your synced trade history.",
		Example:     "  robinhood-agentic-pp-cli portfolio winrate --account 5XX12345 --span month --by-symbol",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			allowedSpan := []string{"week", "month", "3month", "ytd", "all"}
			validSpan := false
			for _, v := range allowedSpan {
				if flagSpan == v {
					validSpan = true
					break
				}
			}
			if !validSpan {
				return usageErr(fmt.Errorf("invalid value %q for --span: must be one of %v", flagSpan, allowedSpan))
			}
			if flagAccount == "" && !flags.dryRun {
				return usageErr(fmt.Errorf("--account is required; usage: %s --account <number> [--span all] [--by-symbol]", cmd.CommandPath()))
			}
			if dryRunOK(flags) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// get_pnl_trade_history is cursor-paginated. Follow the cursor and
			// aggregate every page — computing a win rate from only the first
			// page would silently report partial-history stats as full-history.
			// A seen-cursor guard plus a hard page cap prevent an infinite loop
			// if the service echoes a cursor or never clears it.
			trades := make([]winTrade, 0, 64)
			seenCursor := map[string]bool{}
			cursor := ""
			const maxPages = 100
			for page := 0; page < maxPages; page++ {
				params := map[string]string{"account_number": flagAccount, "span": flagSpan}
				if cursor != "" {
					params["cursor"] = cursor
				}
				raw, err := c.Get(cmd.Context(), "/tools/get_pnl_trade_history", params) // pp:client-call
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var envelope struct {
					Data struct {
						Trades []struct {
							Symbol      string `json:"symbol"`
							RealizedPnl string `json:"realized_pnl"`
						} `json:"trades"`
						NextCursor string `json:"next_cursor"`
						Cursor     string `json:"cursor"`
					} `json:"data"`
				}
				if err := json.Unmarshal(raw, &envelope); err != nil {
					return fmt.Errorf("portfolio winrate: parse get_pnl_trade_history response: %w", err)
				}
				// Only closing trades carry a realized_pnl; opening legs
				// (empty/absent field) are not round trips and are dropped.
				for _, t := range envelope.Data.Trades {
					if strings.TrimSpace(t.RealizedPnl) == "" {
						continue
					}
					trades = append(trades, winTrade{Symbol: t.Symbol, RealizedPnl: parseMoney(t.RealizedPnl)})
				}
				next := envelope.Data.NextCursor
				if next == "" {
					next = envelope.Data.Cursor
				}
				if next == "" || len(envelope.Data.Trades) == 0 || seenCursor[next] {
					break
				}
				seenCursor[next] = true
				cursor = next
			}

			stats := computeWinrate(trades)
			out := winrateOutput{
				Account:       flagAccount,
				Span:          flagSpan,
				TotalTrades:   stats.Overall.TotalTrades,
				Wins:          stats.Overall.Wins,
				Losses:        stats.Overall.Losses,
				WinRate:       stats.Overall.WinRate,
				AvgWin:        stats.Overall.AvgWin,
				AvgLoss:       stats.Overall.AvgLoss,
				TotalRealized: stats.Overall.TotalRealized,
			}
			if flagBySymbol {
				out.BySymbol = stats.BySymbol
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Account %s  span %s\n", out.Account, out.Span)
			if out.TotalTrades == 0 {
				fmt.Fprintf(w, "no closed round trips in span %q\n", out.Span)
				return nil
			}
			fmt.Fprintf(w, "round trips: %d  wins: %d  losses: %d  win rate: %s\n",
				out.TotalTrades, out.Wins, out.Losses, winratePct(stats.Overall))
			fmt.Fprintf(w, "avg win: %+.2f  avg loss: %+.2f  total realized: %+.2f\n",
				out.AvgWin, out.AvgLoss, out.TotalRealized)
			if flagBySymbol && len(stats.BySymbol) > 0 {
				symbols := make([]string, 0, len(stats.BySymbol))
				for sym := range stats.BySymbol {
					symbols = append(symbols, sym)
				}
				sort.Strings(symbols)
				fmt.Fprintf(w, "\n%-8s %7s %5s %7s %9s %10s %10s %15s\n",
					"SYMBOL", "TRADES", "WINS", "LOSSES", "WINRATE", "AVG WIN", "AVG LOSS", "TOTAL REALIZED")
				for _, sym := range symbols {
					b := stats.BySymbol[sym]
					fmt.Fprintf(w, "%-8s %7d %5d %7d %9s %+10.2f %+10.2f %+15.2f\n",
						sym, b.TotalTrades, b.Wins, b.Losses, winratePct(b), b.AvgWin, b.AvgLoss, b.TotalRealized)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagBySymbol, "by-symbol", false, "Include a per-symbol win-rate breakdown")
	cmd.Flags().StringVar(&flagAccount, "account", "", "RHS account number (required)")
	cmd.Flags().StringVar(&flagSpan, "span", "all", "History window (one of: week, month, 3month, ytd, all)")
	return cmd
}
