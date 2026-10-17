// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var sectorWindows = map[string]string{"1d": "chg1d", "1w": "ret1w", "1m": "ret1m", "3m": "ret3m", "6m": "ret6m", "1y": "ret1y"}

type sectorRow struct {
	Rank        int     `json:"rank"`
	Sector      string  `json:"sector"`
	Stocks      int     `json:"stocks"`
	ReturnPct   float64 `json:"return_pct"`
	MedianPct   float64 `json:"median_pct"`
	AvgRS       float64 `json:"avg_rs"`
	PctAbove50  float64 `json:"pct_above_50dma"`
	PctAbove200 float64 `json:"pct_above_200dma"`
	Leader      string  `json:"leader,omitempty"`
	PrevRank    *int    `json:"prev_rank,omitempty"`
	RankChange  *int    `json:"rank_change,omitempty"`
}

type sectorsResult struct {
	Window   string      `json:"window"`
	Source   string      `json:"source"`
	AsOf     string      `json:"as_of,omitempty"`
	PrevAsOf string      `json:"prev_as_of,omitempty"`
	Sectors  []sectorRow `json:"sectors"`
	Note     string      `json:"note,omitempty"`
}

// rankSectors aggregates screener rows by sector over a window field.
func rankSectors(rows []ipRow, field string) []sectorRow {
	type acc struct {
		rets        []float64
		rs          []float64
		n           int
		above50     int
		above200    int
		leaderSym   string
		leaderRet   float64
		leaderFound bool
	}
	by := map[string]*acc{}
	for _, r := range rows {
		sec := ipStrField(r, "sector")
		if sec == "" {
			continue
		}
		a := by[sec]
		if a == nil {
			a = &acc{}
			by[sec] = a
		}
		a.n++
		if v, ok := ipNum(r, field); ok {
			a.rets = append(a.rets, v)
			if !a.leaderFound || v > a.leaderRet {
				a.leaderFound, a.leaderRet, a.leaderSym = true, v, ipStrField(r, "symbol")
			}
		}
		if v, ok := ipNum(r, "rs"); ok {
			a.rs = append(a.rs, v)
		}
		c, cok := ipNum(r, "close")
		if s50, ok := ipNum(r, "sma50"); ok && cok && c > s50 {
			a.above50++
		}
		if s200, ok := ipNum(r, "sma200"); ok && cok && c > s200 {
			a.above200++
		}
	}
	out := make([]sectorRow, 0, len(by))
	for sec, a := range by {
		out = append(out, sectorRow{
			Sector: sec, Stocks: a.n,
			ReturnPct:   ipRound(ipMean(a.rets), 2),
			MedianPct:   ipRound(ipMedian(a.rets), 2),
			AvgRS:       ipRound(ipMean(a.rs), 1),
			PctAbove50:  ipRound(float64(a.above50)/float64(a.n)*100, 0),
			PctAbove200: ipRound(float64(a.above200)/float64(a.n)*100, 0),
			Leader:      a.leaderSym,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ReturnPct != out[j].ReturnPct {
			return out[i].ReturnPct > out[j].ReturnPct
		}
		return out[i].Sector < out[j].Sector
	})
	for i := range out {
		out[i].Rank = i + 1
	}
	return out
}

func newNovelSectorsCmd(flags *rootFlags) *cobra.Command {
	var window, dbPath string
	var trend bool

	cmd := &cobra.Command{
		Use:   "sectors",
		Short: "Equal-weight sector returns over 1w-1y, average RS and % above 50/200 DMA, with rank change versus an earlier snapshot.",
		Long: `Group the Nifty 200 screener table by sector and rank the sectors by equal-weight return over a window,
alongside average relative strength and the share of stocks above their 50 and 200 day averages.
With --trend, compare each sector's rank with the previous stored snapshot.

Use this to see which sectors are gaining or losing strength over 1w-1y and how ranks moved.
Do NOT use it for index-wide advance/decline and DMA counts; use 'breadth' instead.
Do NOT use it to find stocks inside a sector; use 'screen' with --sector instead.`,
		Example: strings.Trim(`
  investorpaisa-pp-cli sectors --window 1m --trend
  investorpaisa-pp-cli sectors --window 1w --agent --select sectors.sector,sectors.return_pct`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "--window=1m"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "sectors")
			}
			field, ok := sectorWindows[strings.ToLower(window)]
			if !ok {
				return usageErr(fmt.Errorf("unknown --window %q; use 1d, 1w, 1m, 3m, 6m or 1y", window))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			loader := newIPLoader(cmd, flags, dbPath)
			defer loader.close()
			rows, src, err := loader.screenerRows(ctx)
			if err != nil {
				return err
			}
			res := sectorsResult{Window: strings.ToLower(window), Source: src.Source, AsOf: src.AsOf, Sectors: rankSectors(rows, field)}
			if trend {
				db, derr := loader.store(ctx)
				if derr != nil {
					return derr
				}
				snaps, serr := ipLatestSnapshots(ctx, db, ipKindScreener, 8)
				if serr != nil {
					return serr
				}
				var prev *ipSnapshot
				for i := range snaps {
					if snaps[i].AsOf < src.AsOf {
						prev = &snaps[i]
						break
					}
				}
				if prev == nil {
					res.Note = "--trend needs an earlier stored snapshot; run 'investorpaisa-pp-cli sync' on another day"
				} else if prevRows, perr := ipParseRows(prev.Body); perr == nil {
					res.PrevAsOf = prev.AsOf
					prevRank := map[string]int{}
					for _, p := range rankSectors(prevRows, field) {
						prevRank[p.Sector] = p.Rank
					}
					for i := range res.Sectors {
						if pr, ok := prevRank[res.Sectors[i].Sector]; ok {
							change := pr - res.Sectors[i].Rank
							res.Sectors[i].PrevRank, res.Sectors[i].RankChange = &pr, &change
						}
					}
				}
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "Sectors by %s return  (%s, as of %s)\n", res.Window, res.Source, res.AsOf)
				items := make([]map[string]any, 0, len(res.Sectors))
				for _, s := range res.Sectors {
					it := map[string]any{"rank": s.Rank, "sector": s.Sector, "return_pct": s.ReturnPct, "avg_rs": s.AvgRS, "above_50dma": s.PctAbove50, "above_200dma": s.PctAbove200}
					if s.RankChange != nil {
						it["rank_change"] = *s.RankChange
					}
					items = append(items, it)
				}
				if res.Note != "" {
					fmt.Fprintln(cmd.ErrOrStderr(), res.Note)
				}
				return printAutoTable(cmd.OutOrStdout(), items)
			}
			return printJSONFiltered(cmd.OutOrStdout(), res, flags)
		},
	}
	cmd.Flags().StringVar(&window, "window", "1m", "Return window: 1d, 1w, 1m, 3m, 6m or 1y")
	cmd.Flags().BoolVar(&trend, "trend", false, "Compare ranks with the previous stored snapshot")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local SQLite store")
	return cmd
}
