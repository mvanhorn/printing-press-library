// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type briefIndex struct {
	Level  float64 `json:"level"`
	ChgPct float64 `json:"chg_pct"`
}

type briefWatchRow struct {
	Symbol  string   `json:"symbol"`
	Name    string   `json:"name,omitempty"`
	Ltp     float64  `json:"ltp"`
	ChgPct  float64  `json:"chg_pct"`
	RelVol  float64  `json:"rel_vol,omitempty"`
	RS      *float64 `json:"rs,omitempty"`
	Signals []string `json:"signals,omitempty"`
}

type briefWatchlist struct {
	Name string          `json:"name"`
	Rows []briefWatchRow `json:"rows"`
}

type briefSector struct {
	Sector string  `json:"sector"`
	AvgPct float64 `json:"avg_pct"`
	Up     int     `json:"up"`
	Down   int     `json:"down"`
}

type briefHeadline struct {
	Symbol      string `json:"symbol"`
	Title       string `json:"title"`
	Source      string `json:"source"`
	PublishedAt string `json:"published_at"`
}

type briefResult struct {
	Source     string                   `json:"source"`
	AsOf       string                   `json:"as_of,omitempty"`
	LastTrade  string                   `json:"last_trade,omitempty"`
	Index      map[string]briefIndex    `json:"index"`
	Breadth    map[string]any           `json:"breadth"`
	Signals    []map[string]any         `json:"top_signals,omitempty"`
	Sectors    map[string][]briefSector `json:"sectors,omitempty"`
	Watchlists []briefWatchlist         `json:"watchlists"`
	Headlines  []briefHeadline          `json:"headlines,omitempty"`
	Notes      []string                 `json:"notes,omitempty"`
}

// summarizeWatchlist builds rows for a watchlist from live and screener data,
// ordered by absolute move so the biggest stories lead.
func summarizeWatchlist(l ipWatchlist, live map[string]ipRow, rs map[string]float64) briefWatchlist {
	out := briefWatchlist{Name: l.Name, Rows: make([]briefWatchRow, 0, len(l.Symbols))}
	for _, sym := range l.Symbols {
		r, ok := live[sym]
		if !ok {
			continue
		}
		row := briefWatchRow{Symbol: sym, Name: ipStrField(r, "name")}
		row.Ltp, _ = ipNum(r, "ltp")
		row.ChgPct, _ = ipNum(r, "chg")
		row.RelVol, _ = ipNum(r, "relVol")
		row.Ltp, row.ChgPct, row.RelVol = ipRound(row.Ltp, 2), ipRound(row.ChgPct, 2), ipRound(row.RelVol, 2)
		if v, ok := rs[sym]; ok {
			vv := v
			row.RS = &vv
		}
		for _, s := range ipRowSignals(r) {
			row.Signals = append(row.Signals, s.Label)
		}
		out.Rows = append(out.Rows, row)
	}
	sort.SliceStable(out.Rows, func(i, j int) bool {
		a, b := out.Rows[i].ChgPct, out.Rows[j].ChgPct
		if a < 0 {
			a = -a
		}
		if b < 0 {
			b = -b
		}
		return a > b
	})
	return out
}

func newNovelBriefCmd(flags *rootFlags) *cobra.Command {
	var watchlistName, dbPath string
	var noNews bool
	var sinceHours int

	cmd := &cobra.Command{
		Use:   "brief",
		Short: "Index levels, breadth, your watchlist rows with change, RS and signals, and fresh headlines in one JSON document.",
		Long: `A single-call market brief for a scheduled agent or a morning check: Nifty 50/100/200 levels, breadth, the strongest
and weakest sectors today, the ranked 52-week/gap/volume signals, your watchlist rows (change, relative volume, RS, signals)
and fresh headlines for the watchlist names that moved most.
The breadth block uses the site's intraday aggregate (at_52w_high/at_52w_low), which can differ from the end-of-day
counts that 'breadth' derives from the screener table.

Use this for a single-call summary of the market backdrop plus your watchlist names and their fresh headlines.
Do NOT use it for a single stock deep dive; use 'quote' instead.
Do NOT use it for state changes since an earlier snapshot; use 'changes' instead.`,
		Example: strings.Trim(`
  investorpaisa-pp-cli brief
  investorpaisa-pp-cli brief --watchlist "My watchlist" --since-hours 24 --agent
  investorpaisa-pp-cli brief --agent --select index,breadth,watchlists.name`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "--no-news"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "brief")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			loader := newIPLoader(cmd, flags, dbPath)
			defer loader.close()
			live, src, err := loader.liveSnapshot(ctx)
			if err != nil {
				return err
			}
			res := briefResult{Source: src.Source, AsOf: live.Payload.SessionDate, LastTrade: live.Payload.LastTrade, Index: map[string]briefIndex{}, Watchlists: []briefWatchlist{}}
			for name, ix := range live.Payload.Indices {
				res.Index[strings.ToLower(name)] = briefIndex{Level: ipRound(ix.Level, 2), ChgPct: ipRound(ix.ChgPct, 2)}
			}
			agg := live.Payload.Agg
			res.Breadth = map[string]any{
				"advancers": agg.Up, "decliners": agg.Down, "avg_chg_pct": ipRound(agg.Avg, 2), "median_chg_pct": ipRound(agg.Median, 2),
				"at_52w_high": agg.Hi52, "at_52w_low": agg.Lo52, "pct_above_200dma": ipRound(agg.Above200, 1),
				"unusual_volume": agg.UnusualVol, "traded_value_cr": ipRound(agg.ValueCr, 0),
			}
			var sectorsRaw struct {
				Payload struct {
					Agg struct {
						Sectors []struct {
							Sector string  `json:"sector"`
							Avg    float64 `json:"avg"`
							Up     int     `json:"up"`
							Down   int     `json:"down"`
						} `json:"sectors"`
					} `json:"agg"`
				} `json:"payload"`
			}
			if body, _, serr := loader.snapshot(ctx, ipKindLive); serr == nil && json.Unmarshal(body, &sectorsRaw) == nil {
				secs := make([]briefSector, 0)
				for _, s := range sectorsRaw.Payload.Agg.Sectors {
					secs = append(secs, briefSector{Sector: s.Sector, AvgPct: ipRound(s.Avg, 2), Up: s.Up, Down: s.Down})
				}
				sort.SliceStable(secs, func(i, j int) bool { return secs[i].AvgPct > secs[j].AvgPct })
				if len(secs) > 0 {
					n := 3
					if len(secs) < 6 {
						n = len(secs) / 2
					}
					res.Sectors = map[string][]briefSector{"leaders": secs[:n], "laggards": secs[len(secs)-n:]}
				}
			}
			if body, _, oerr := loader.snapshot(ctx, ipKindOverview); oerr == nil {
				var ov struct {
					Signals []map[string]any `json:"signals"`
				}
				if json.Unmarshal(body, &ov) == nil {
					if len(ov.Signals) > 5 {
						ov.Signals = ov.Signals[:5]
					}
					res.Signals = ov.Signals
				}
			} else {
				res.Notes = append(res.Notes, "market overview unavailable: "+oerr.Error())
			}
			rs := map[string]float64{}
			if rows, _, serr := loader.screenerRows(ctx); serr == nil {
				for _, r := range rows {
					if v, ok := ipNum(r, "rs"); ok {
						rs[ipStrField(r, "symbol")] = v
					}
				}
			}
			liveBy := map[string]ipRow{}
			for _, r := range live.Payload.Rows {
				liveBy[ipStrField(r, "symbol")] = r
			}
			if flags.dataSource == "local" {
				res.Notes = append(res.Notes, "watchlists and headlines are not stored locally; omitted with --data-source local")
			} else if lists, lerr := ipWatchlists(ctx, flags); lerr != nil {
				res.Notes = append(res.Notes, "watchlists unavailable: "+lerr.Error())
			} else {
				var moved []briefWatchRow
				for _, l := range lists {
					if watchlistName != "" && !strings.EqualFold(l.Name, watchlistName) {
						continue
					}
					w := summarizeWatchlist(l, liveBy, rs)
					res.Watchlists = append(res.Watchlists, w)
					moved = append(moved, w.Rows...)
				}
				if watchlistName != "" && len(res.Watchlists) == 0 {
					return usageErr(fmt.Errorf("no watchlist named %q", watchlistName))
				}
				sort.SliceStable(moved, func(i, j int) bool { return math.Abs(moved[i].ChgPct) > math.Abs(moved[j].ChgPct) })
				if !noNews {
					cutoff := time.Now().Add(-time.Duration(sinceHours) * time.Hour)
					seen := map[string]bool{}
					for i, m := range moved {
						if i >= 8 || seen[m.Symbol] {
							continue
						}
						seen[m.Symbol] = true
						items, nerr := ipFetchNews(ctx, flags, m.Symbol, 5)
						if nerr != nil {
							res.Notes = append(res.Notes, fmt.Sprintf("news for %s unavailable", m.Symbol))
							continue
						}
						kept := 0
						for _, it := range items {
							if t, perr := time.Parse(time.RFC3339, it.PublishedAt); perr == nil && t.Before(cutoff) {
								continue
							}
							if kept >= 2 {
								break
							}
							kept++
							res.Headlines = append(res.Headlines, briefHeadline{Symbol: m.Symbol, Title: it.Title, Source: it.Source, PublishedAt: it.PublishedAt})
						}
					}
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), res, flags)
		},
	}
	cmd.Flags().StringVar(&watchlistName, "watchlist", "", "Only this watchlist (default: all of them)")
	cmd.Flags().BoolVar(&noNews, "no-news", false, "Skip fetching headlines")
	cmd.Flags().IntVar(&sinceHours, "since-hours", 36, "Only headlines published within this many hours")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local SQLite store")
	return cmd
}
