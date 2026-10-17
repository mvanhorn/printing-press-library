// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto
// pp:client-call (reaches the API through ipLoader.liveSnapshot / screenerRows)

package cli

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/investorpaisa/internal/cliutil"
	"github.com/spf13/cobra"
)

type moverRow struct {
	Symbol  string   `json:"symbol"`
	Name    string   `json:"name"`
	Sector  string   `json:"sector"`
	Ltp     float64  `json:"ltp"`
	ChgPct  float64  `json:"chg_pct"`
	RelVol  float64  `json:"rel_vol"`
	GapPct  float64  `json:"gap_pct"`
	ValueCr float64  `json:"value_cr"`
	RS      *float64 `json:"rs,omitempty"`
	Signals []string `json:"signals,omitempty"`
}

type moversResult struct {
	Kind      string     `json:"kind"`
	Source    string     `json:"source"`
	AsOf      string     `json:"as_of,omitempty"`
	LastTrade string     `json:"last_trade,omitempty"`
	Scanned   int        `json:"scanned"`
	Count     int        `json:"count"`
	Rows      []moverRow `json:"rows"`
	Note      string     `json:"note,omitempty"`
}

var moverKinds = []string{"gainers", "losers", "volume", "gap", "high52", "low52"}

func ipRowAtHigh(r ipRow) bool {
	for _, s := range ipRowSignals(r) {
		if s.Type == "hi52" {
			return true
		}
	}
	ltp, ok1 := ipNum(r, "ltp")
	hi, ok2 := ipNum(r, "hi52")
	return ok1 && ok2 && hi > 0 && ltp >= hi
}

func ipRowAtLow(r ipRow) bool {
	for _, s := range ipRowSignals(r) {
		if s.Type == "lo52" {
			return true
		}
	}
	ltp, ok1 := ipNum(r, "ltp")
	lo, ok2 := ipNum(r, "lo52")
	return ok1 && ok2 && lo > 0 && ltp <= lo
}

// rankMovers filters and orders live rows for a mover kind. Pure function so
// it can be tested without any network or store access.
func rankMovers(rows []ipRow, kind string, rsBySymbol map[string]float64, minRS float64, sector string, top int) ([]moverRow, error) {
	valid := false
	for _, k := range moverKinds {
		valid = valid || k == kind
	}
	if !valid {
		return nil, fmt.Errorf("unknown --kind %q; use one of: %s", kind, strings.Join(moverKinds, ", "))
	}
	out := make([]moverRow, 0)
	for _, r := range rows {
		sym := ipStrField(r, "symbol")
		if sym == "" {
			continue
		}
		if sector != "" && !strings.EqualFold(ipStrField(r, "sector"), sector) {
			continue
		}
		switch kind {
		case "high52":
			if !ipRowAtHigh(r) {
				continue
			}
		case "low52":
			if !ipRowAtLow(r) {
				continue
			}
		}
		m := moverRow{Symbol: sym, Name: ipStrField(r, "name"), Sector: ipStrField(r, "sector")}
		m.Ltp, _ = ipNum(r, "ltp")
		m.ChgPct, _ = ipNum(r, "chg")
		m.RelVol, _ = ipNum(r, "relVol")
		m.GapPct, _ = ipNum(r, "gap")
		m.ValueCr, _ = ipNum(r, "valueCr")
		if rs, ok := rsBySymbol[sym]; ok {
			v := rs
			m.RS = &v
		}
		if minRS > 0 && (m.RS == nil || *m.RS < minRS) {
			continue
		}
		for _, s := range ipRowSignals(r) {
			m.Signals = append(m.Signals, s.Label)
		}
		m.Ltp, m.ChgPct, m.RelVol, m.GapPct, m.ValueCr = ipRound(m.Ltp, 2), ipRound(m.ChgPct, 2), ipRound(m.RelVol, 2), ipRound(m.GapPct, 2), ipRound(m.ValueCr, 1)
		out = append(out, m)
	}
	sort.SliceStable(out, func(i, j int) bool {
		switch kind {
		case "losers":
			return out[i].ChgPct < out[j].ChgPct
		case "volume":
			return out[i].RelVol > out[j].RelVol
		case "gap":
			return math.Abs(out[i].GapPct) > math.Abs(out[j].GapPct)
		case "high52", "low52":
			return out[i].ValueCr > out[j].ValueCr
		}
		return out[i].ChgPct > out[j].ChgPct
	})
	if top > 0 && len(out) > top {
		out = out[:top]
	}
	return out, nil
}

func newNovelMoversCmd(flags *rootFlags) *cobra.Command {
	var kind, sector, dbPath string
	var minRS float64
	var top int

	cmd := &cobra.Command{
		Use:   "movers",
		Short: "Ranked gainers, losers, unusual volume, gappers and 52-week highs/lows, filtered by RS and sector.",
		Long: `Ranked movers from the live Nifty 200 snapshot, joined to each stock's relative-strength rank from the screener table.

Use this for today's ranked gainers, losers, unusual-volume names, gappers and 52-week highs/lows, optionally filtered by --min-rs and --sector.
Do NOT use it for custom multi-metric filters; use 'screen' instead.
Do NOT use it for the entire index and breadth picture; use 'breadth' or 'brief' instead.`,
		Example: strings.Trim(`
  investorpaisa-pp-cli movers --kind gainers --top 5
  investorpaisa-pp-cli movers --kind volume --min-rs 70 --top 10 --agent
  investorpaisa-pp-cli movers --kind high52 --sector IT`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "--kind=gainers;--top=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "movers")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if _, err := rankMovers(nil, kind, nil, 0, "", 0); err != nil {
				return usageErr(err)
			}
			loader := newIPLoader(cmd, flags, dbPath)
			defer loader.close()
			live, src, err := loader.liveSnapshot(ctx)
			if err != nil {
				return err
			}
			rs := map[string]float64{}
			if rows, _, serr := loader.screenerRows(ctx); serr == nil {
				for _, r := range rows {
					if v, ok := ipNum(r, "rs"); ok {
						rs[ipStrField(r, "symbol")] = v
					}
				}
			} else if minRS > 0 {
				return fmt.Errorf("--min-rs needs the screener table: %w", serr)
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: screener table unavailable (%v); RS omitted\n", serr)
			}
			rows, rerr := rankMovers(live.Payload.Rows, kind, rs, minRS, sector, top)
			if rerr != nil {
				return usageErr(rerr)
			}
			res := moversResult{Kind: kind, Source: src.Source, AsOf: live.Payload.SessionDate, LastTrade: live.Payload.LastTrade, Scanned: len(live.Payload.Rows), Count: len(rows), Rows: rows}
			if len(rows) == 0 {
				res.Note = "no stock matched these filters in the current snapshot"
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "%s  (%s, session %s, last trade %s)\n", kind, res.Source, cliutil.ScrubTerminal(res.AsOf), cliutil.ScrubTerminal(res.LastTrade))
				items := make([]map[string]any, 0, len(rows))
				for _, m := range rows {
					items = append(items, map[string]any{"symbol": m.Symbol, "sector": m.Sector, "ltp": m.Ltp, "chg_pct": m.ChgPct, "rel_vol": m.RelVol, "rs": m.RS})
				}
				if len(items) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), res.Note)
					return nil
				}
				return printAutoTable(cmd.OutOrStdout(), items)
			}
			return printJSONFiltered(cmd.OutOrStdout(), res, flags)
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "gainers", "gainers, losers, volume, gap, high52 or low52")
	cmd.Flags().StringVar(&sector, "sector", "", "Only stocks in this sector")
	cmd.Flags().Float64Var(&minRS, "min-rs", 0, "Only stocks with relative strength at or above this value")
	cmd.Flags().IntVar(&top, "top", 10, "Maximum rows to return (0 for all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local SQLite store")
	return cmd
}
