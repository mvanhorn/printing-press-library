// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type breadthPoint struct {
	Date         string   `json:"date"`
	Stocks       int      `json:"stocks"`
	Advancers    int      `json:"advancers"`
	Decliners    int      `json:"decliners"`
	AdRatio      *float64 `json:"ad_ratio,omitempty"`
	PctAbove50   float64  `json:"pct_above_50dma"`
	PctAbove200  float64  `json:"pct_above_200dma"`
	New52wHighs  int      `json:"new_52w_highs"`
	At52wLows    int      `json:"at_52w_lows"`
	AvgChgPct    float64  `json:"avg_chg_pct"`
	MedianChgPct float64  `json:"median_chg_pct"`
}

type breadthResult struct {
	Source string             `json:"source"`
	Days   int                `json:"days"`
	Points []breadthPoint     `json:"points"`
	Trend  map[string]float64 `json:"trend,omitempty"`
	Note   string             `json:"note,omitempty"`
}

// breadthOf computes breadth statistics from one screener snapshot.
func breadthOf(date string, rows []ipRow) breadthPoint {
	p := breadthPoint{Date: date, Stocks: len(rows)}
	var chgs []float64
	var above50, above200, with50, with200 int
	for _, r := range rows {
		if v, ok := ipNum(r, "chg1d"); ok {
			chgs = append(chgs, v)
			if v > 0 {
				p.Advancers++
			} else if v < 0 {
				p.Decliners++
			}
		}
		c, cok := ipNum(r, "close")
		if s, ok := ipNum(r, "sma50"); ok && cok {
			with50++
			if c > s {
				above50++
			}
		}
		if s, ok := ipNum(r, "sma200"); ok && cok {
			with200++
			if c > s {
				above200++
			}
		}
		if v, ok := ipNum(r, "newhi"); ok && v >= 1 {
			p.New52wHighs++
		}
		if v, ok := ipNum(r, "fromlo52"); ok && v <= 0 {
			p.At52wLows++
		}
	}
	if p.Decliners > 0 {
		r := ipRound(float64(p.Advancers)/float64(p.Decliners), 2)
		p.AdRatio = &r
	}
	if with50 > 0 {
		p.PctAbove50 = ipRound(float64(above50)/float64(with50)*100, 1)
	}
	if with200 > 0 {
		p.PctAbove200 = ipRound(float64(above200)/float64(with200)*100, 1)
	}
	p.AvgChgPct = ipRound(ipMean(chgs), 2)
	p.MedianChgPct = ipRound(ipMedian(chgs), 2)
	return p
}

func ipRatioText(r *float64) any {
	if r == nil {
		return "n/a"
	}
	return *r
}

func newNovelBreadthCmd(flags *rootFlags) *cobra.Command {
	var days int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "breadth",
		Short: "Trend of advance/decline, % above 50/200 DMA, new 52-week highs and stocks at 52-week lows across your stored daily snapshots.",
		Long: `Show Nifty 200 market breadth for every stored daily snapshot: advancers and decliners, the share of stocks above
their 50 and 200 day averages, new 52-week highs, stocks at 52-week lows, and the average day change.
The site shows only today's numbers; history builds up locally each time the screener table is fetched.
The 52-week counts come from the end-of-day screener rows (new_52w_highs = stocks flagged as a new high, at_52w_lows =
stocks at or below their 52-week low), so they can differ from the intraday at_52w_high/at_52w_low figures in 'brief'.

Use this for how market breadth has trended across your stored snapshots, e.g. whether the share above the 200 DMA is rising.
Do NOT use it for per-sector breakdowns; use 'sectors' instead.
Do NOT use it for which individual stocks changed; use 'changes' instead.`,
		Example: strings.Trim(`
  investorpaisa-pp-cli breadth --days 30
  investorpaisa-pp-cli breadth --days 10 --agent --select points.date,points.pct_above_200dma`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "--days=10"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "breadth")
			}
			if days < 1 {
				return usageErr(fmt.Errorf("--days must be at least 1"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			loader := newIPLoader(cmd, flags, dbPath)
			defer loader.close()
			_, src, err := loader.screenerRows(ctx) // fetches and stores today's snapshot
			if err != nil {
				return err
			}
			db, err := loader.store(ctx)
			if err != nil {
				return err
			}
			snaps, err := ipLatestSnapshots(ctx, db, ipKindScreener, days)
			if err != nil {
				return err
			}
			res := breadthResult{Source: src.Source, Days: days, Points: make([]breadthPoint, 0, len(snaps))}
			for i := len(snaps) - 1; i >= 0; i-- { // oldest first
				rows, perr := ipParseRows(snaps[i].Body)
				if perr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: skipping unreadable snapshot %s\n", snaps[i].AsOf)
					continue
				}
				res.Points = append(res.Points, breadthOf(snaps[i].AsOf, rows))
			}
			if len(res.Points) < 2 {
				res.Note = "only one snapshot is stored so no trend is available yet; run 'investorpaisa-pp-cli sync' on more trading days"
			} else {
				first, last := res.Points[0], res.Points[len(res.Points)-1]
				res.Trend = map[string]float64{
					"pct_above_50dma_change":  ipRound(last.PctAbove50-first.PctAbove50, 1),
					"pct_above_200dma_change": ipRound(last.PctAbove200-first.PctAbove200, 1),
				}
				if first.AdRatio != nil && last.AdRatio != nil {
					res.Trend["ad_ratio_change"] = ipRound(*last.AdRatio-*first.AdRatio, 2)
				}
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				items := make([]map[string]any, 0, len(res.Points))
				for _, p := range res.Points {
					items = append(items, map[string]any{"date": p.Date, "adv": p.Advancers, "dec": p.Decliners, "ad_ratio": ipRatioText(p.AdRatio), "above_50dma": p.PctAbove50, "above_200dma": p.PctAbove200, "52w_highs": p.New52wHighs, "52w_lows": p.At52wLows})
				}
				if res.Note != "" {
					fmt.Fprintln(cmd.ErrOrStderr(), res.Note)
				}
				return printAutoTable(cmd.OutOrStdout(), items)
			}
			return printJSONFiltered(cmd.OutOrStdout(), res, flags)
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "Number of most recent stored snapshots to include")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local SQLite store")
	return cmd
}
