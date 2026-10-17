// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type quoteHeadline struct {
	Title       string `json:"title"`
	Source      string `json:"source"`
	PublishedAt string `json:"published_at"`
	Link        string `json:"link"`
}

type quoteView struct {
	Symbol           string             `json:"symbol"`
	Name             string             `json:"name"`
	Sector           string             `json:"sector"`
	Price            float64            `json:"price"`
	ChgPct           float64            `json:"chg_pct"`
	Day              map[string]float64 `json:"day,omitempty"`
	Week52           map[string]float64 `json:"week52,omitempty"`
	Volume           map[string]float64 `json:"volume,omitempty"`
	Trend            map[string]float64 `json:"trend,omitempty"`
	Returns          map[string]float64 `json:"returns_pct,omitempty"`
	Risk             map[string]float64 `json:"risk,omitempty"`
	SectorPercentile map[string]float64 `json:"sector_percentile,omitempty"`
	Signals          []ipSignal         `json:"signals,omitempty"`
	Watchlists       []string           `json:"watchlists,omitempty"`
	News             []quoteHeadline    `json:"news,omitempty"`
}

type quoteResult struct {
	Source  string      `json:"source"`
	AsOf    string      `json:"as_of,omitempty"`
	Quotes  []quoteView `json:"quotes"`
	Missing []string    `json:"missing,omitempty"`
}

func pick(r ipRow, dst map[string]float64, pairs ...string) {
	for i := 0; i+1 < len(pairs); i += 2 {
		if v, ok := ipNum(r, pairs[i]); ok {
			dst[pairs[i+1]] = ipRound(v, 2)
		}
	}
}

// buildQuote assembles one stock's view from the screener row and, when
// present, its live row. Sector percentiles rank the stock against its peers.
func buildQuote(sym string, screener []ipRow, live ipRow) (quoteView, bool) {
	var row ipRow
	var peers []ipRow
	for _, r := range screener {
		if ipStrField(r, "symbol") == sym {
			row = r
		}
	}
	if row == nil && live == nil {
		return quoteView{}, false
	}
	q := quoteView{Symbol: sym, Day: map[string]float64{}, Week52: map[string]float64{}, Volume: map[string]float64{}, Trend: map[string]float64{}, Returns: map[string]float64{}, Risk: map[string]float64{}, SectorPercentile: map[string]float64{}}
	if row != nil {
		q.Name, q.Sector = ipStrField(row, "name"), ipStrField(row, "sector")
		q.Price, _ = ipNum(row, "close")
		q.ChgPct, _ = ipNum(row, "chg1d")
		pick(row, q.Day, "open", "open", "high", "high", "low", "low")
		pick(row, q.Week52, "hi52", "high", "lo52", "low", "pos52", "position_pct", "fromhi52", "from_high_pct", "fromlo52", "from_low_pct")
		pick(row, q.Volume, "volume", "today", "avgvol20", "avg_20d", "volratio", "pace_vs_20d")
		pick(row, q.Trend, "rs", "rs", "rsi14", "rsi14", "sma20", "sma20", "sma50", "sma50", "sma200", "sma200", "ema20", "ema20", "ema50", "ema50", "d50", "vs_50dma_pct", "d200", "vs_200dma_pct")
		pick(row, q.Returns, "ret1w", "1w", "ret1m", "1m", "ret3m", "3m", "ret6m", "6m", "ret1y", "1y", "cagr3y", "cagr_3y")
		pick(row, q.Risk, "beta1y", "beta_1y", "vol1y", "volatility_1y", "maxdd1y", "max_drawdown_1y", "atrpct", "atr_pct")
		for _, r := range screener {
			if ipStrField(r, "sector") == q.Sector {
				peers = append(peers, r)
			}
		}
		for _, f := range []string{"rs", "ret3m", "pos52"} {
			var vals []float64
			for _, p := range peers {
				if v, ok := ipNum(p, f); ok {
					vals = append(vals, v)
				}
			}
			if v, ok := ipNum(row, f); ok && len(vals) > 1 {
				q.SectorPercentile[f] = ipPercentile(vals, v)
			}
		}
	}
	if live != nil {
		if q.Name == "" {
			q.Name, q.Sector = ipStrField(live, "name"), ipStrField(live, "sector")
		}
		if v, ok := ipNum(live, "ltp"); ok {
			q.Price = v
		}
		if v, ok := ipNum(live, "chg"); ok {
			q.ChgPct = v
		}
		pick(live, q.Day, "open", "open", "high", "high", "low", "low", "prevClose", "prev_close")
		pick(live, q.Volume, "volume", "today", "relVol", "rel_vol", "valueCr", "value_cr")
		q.Signals = ipRowSignals(live)
	}
	q.Price, q.ChgPct = ipRound(q.Price, 2), ipRound(q.ChgPct, 2)
	return q, true
}

func ipFetchNews(ctx context.Context, flags *rootFlags, sym string, limit int) ([]quoteHeadline, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	data, err := c.Get(ctx, "/api/news", map[string]string{"symbol": sym})
	if err != nil {
		return nil, classifyAPIErrorOnly(err)
	}
	var page struct {
		Items []struct {
			Title       string `json:"title"`
			Source      string `json:"source"`
			PublishedAt string `json:"publishedAt"`
			Link        string `json:"link"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, err
	}
	out := make([]quoteHeadline, 0, limit)
	for _, it := range page.Items {
		if len(out) >= limit {
			break
		}
		out = append(out, quoteHeadline{Title: it.Title, Source: it.Source, PublishedAt: it.PublishedAt, Link: it.Link})
	}
	return out, nil
}

// ipWatchlists returns watchlist name -> symbols (best effort).
func ipWatchlists(ctx context.Context, flags *rootFlags) ([]ipWatchlist, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	data, err := c.GetNoCache(ctx, "/api/watchlists", nil)
	if err != nil {
		return nil, classifyAPIErrorOnly(err)
	}
	var lists []ipWatchlist
	if err := json.Unmarshal(data, &lists); err != nil {
		return nil, err
	}
	return lists, nil
}

type ipWatchlist struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Symbols []string `json:"symbols"`
}

func newNovelQuoteCmd(flags *rootFlags) *cobra.Command {
	var newsN int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "quote [symbol...]",
		Short: "Price, range, RSI/SMA/EMA, returns, sector percentile, signals, headlines and watchlist membership for one or more stocks in a single call.",
		Long: `One call for the current picture of a Nifty 200 stock: price and day range, 52-week range, trend and momentum
(RS, RSI, SMAs/EMAs), returns, risk, percentile rank within its sector, live signals, recent headlines and which of
your watchlists it is on.

Use this for the current picture of one or a few stocks.
Do NOT use it for 5-year price series or corporate events; use 'history' instead.
Do NOT use it to scan the whole universe; use 'screen' instead.`,
		Example: strings.Trim(`
  investorpaisa-pp-cli quote RELIANCE
  investorpaisa-pp-cli quote TCS INFY --news 5 --agent
  investorpaisa-pp-cli quote 'M&M' --agent --select quotes.symbol,quotes.price,quotes.sector_percentile`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "symbol=RELIANCE"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "quote")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("at least one symbol is required, e.g. quote RELIANCE"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			loader := newIPLoader(cmd, flags, dbPath)
			defer loader.close()
			screener, src, err := loader.screenerRows(ctx)
			if err != nil {
				return err
			}
			liveBySym := map[string]ipRow{}
			if lv, _, lerr := loader.liveSnapshot(ctx); lerr == nil {
				for _, r := range lv.Payload.Rows {
					liveBySym[ipStrField(r, "symbol")] = r
				}
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: live snapshot unavailable (%v); showing last close\n", lerr)
			}
			var lists []ipWatchlist
			if flags.dataSource != "local" {
				var lerr error
				if lists, lerr = ipWatchlists(ctx, flags); lerr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: watchlist membership unavailable: %v\n", lerr)
				}
			}
			res := quoteResult{Source: src.Source, AsOf: src.AsOf, Quotes: make([]quoteView, 0, len(args))}
			for _, a := range args {
				sym := ipNormalizeSymbol(a)
				if !ipValidSymbol(sym) {
					return usageErr(fmt.Errorf("%q is not a valid NSE symbol", a))
				}
				q, ok := buildQuote(sym, screener, liveBySym[sym])
				if !ok {
					res.Missing = append(res.Missing, sym)
					continue
				}
				for _, l := range lists {
					for _, s := range l.Symbols {
						if s == sym {
							q.Watchlists = append(q.Watchlists, l.Name)
						}
					}
				}
				if newsN > 0 && flags.dataSource != "local" {
					if items, nerr := ipFetchNews(ctx, flags, sym, newsN); nerr == nil {
						q.News = items
					} else {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: news for %s unavailable: %v\n", sym, nerr)
					}
				}
				res.Quotes = append(res.Quotes, q)
			}
			if len(res.Quotes) == 0 {
				return notFoundErr(fmt.Errorf("no Nifty 200 stock found for: %s", strings.Join(res.Missing, ", ")))
			}
			return printJSONFiltered(cmd.OutOrStdout(), res, flags)
		},
	}
	cmd.Flags().IntVar(&newsN, "news", 3, "Headlines to include per stock (0 for none)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local SQLite store")
	return cmd
}
