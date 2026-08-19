// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/psx/internal/cliutil"
)

// driftMetrics maps the public --metric vocabulary onto the screener's
// header-normalized column names.
// driftMetrics maps the public --metric vocabulary onto candidate screener
// columns. Candidates are needed because PSX bakes units into header text
// ("MARKET CAP. (B)" -> market_cap_b, "FREE FLOAT (M)" -> free_float_m), so a
// single fixed key silently matches nothing and the command wrongly blames
// missing history.
var driftMetrics = map[string][]string{
	"pe":         {"pe_ratio_ttm", "pe_ratio", "pe"},
	"yield":      {"dividend_yield_pct", "dividend_yield"},
	"market-cap": {"market_cap", "market_cap_b", "market_cap_m"},
	"free-float": {"free_float", "free_float_m"},
	"price":      {"price", "current"},
}

// resolveDriftColumn picks the first candidate present in the stored rows.
// Returns the column plus the keys actually available, so a miss can report
// what exists instead of guessing.
func resolveDriftColumn(candidates []string, rows []snapshotRow) (string, []string) {
	present := map[string]bool{}
	for _, r := range rows {
		for k := range r.Data {
			present[k] = true
		}
	}
	for _, c := range candidates {
		if present[c] {
			return c, sortedKeys(present)
		}
	}
	return "", sortedKeys(present)
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// newNovelDriftCmd traces one symbol's valuation metric over time. The screener
// exposes the metrics but retains no history, so this reads accumulated
// local snapshots.
func newNovelDriftCmd(flags *rootFlags) *cobra.Command {
	var metric, since, dbPath string
	cmd := &cobra.Command{
		Use:   "drift <symbol>",
		Short: "Trace one symbol's PE, dividend yield, market cap or free float across time.",
		Long: "Use this command to trace one symbol's valuation metric over time.\n" +
			"Do NOT use it to filter the universe by current metric thresholds; use 'screener' instead.\n" +
			"Reads accumulated 'snapshot take' history; the portal retains none of this itself.",
		Example:     "  psx-pp-cli drift OGDC --metric pe --since 90d --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "symbol=OGDC", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "drift")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a symbol is required, e.g. OGDC"))
			}
			candidates, ok := driftMetrics[strings.ToLower(strings.TrimSpace(metric))]
			if !ok {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--metric must be one of pe, yield, market-cap, free-float, price (got %q)", metric))
			}
			sym := strings.ToUpper(strings.TrimSpace(args[0]))
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if mirrorMissing(dbPath) {
				return writeMirrorHint(cmd, flags, orDefaultDB(dbPath), "snapshot")
			}
			s, _, err := openLocalStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer s.Close()

			cutoff := ""
			if strings.TrimSpace(since) != "" {
				d, err := cliutil.ParseDurationLoose(since)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--since %q is not a duration (try 90d, 4w): %w", since, err))
				}
				cutoff = nowUTC().Add(-d).Format(snapshotTimeFormat)
			}
			series, err := loadSeries(ctx, s, snapshotKindScreener, sym, cutoff)
			if err != nil {
				return err
			}
			col, available := resolveDriftColumn(candidates, series)
			if col == "" && len(series) > 0 {
				// History exists but this metric is not among the stored
				// columns — say so rather than blaming missing history.
				return usageErr(fmt.Errorf("metric %q is not present in the stored screener columns for %s; available: %s",
					metric, sym, strings.Join(available, ", ")))
			}
			if col == "" {
				col = candidates[0]
			}
			// Distinguish "symbol does not exist" from "no history yet". Only the
			// former is an error; the latter is a legitimate empty result.
			if len(series) == 0 {
				known, kerr := symbolIsListed(ctx, psxClient(flags), sym)
				if kerr == nil && !known {
					return notFoundErr(fmt.Errorf("no listed instrument %q; check the code with 'psx-pp-cli symbols list'", sym))
				}
			}
			type point struct {
				TakenAt string  `json:"taken_at"`
				Value   float64 `json:"value"`
			}
			view := struct {
				Symbol    string  `json:"symbol"`
				Metric    string  `json:"metric"`
				Column    string  `json:"column"`
				Count     int     `json:"count"`
				First     float64 `json:"first,omitempty"`
				Last      float64 `json:"last,omitempty"`
				ChangePct float64 `json:"change_pct,omitempty"`
				Points    []point `json:"points"`
				Note      string  `json:"note,omitempty"`
			}{Symbol: sym, Metric: metric, Column: col, Points: make([]point, 0, len(series))}

			for _, row := range series {
				v, ok := parseNum(row.Data[col])
				if !ok {
					continue
				}
				view.Points = append(view.Points, point{TakenAt: row.TakenAt, Value: v})
			}
			view.Count = len(view.Points)
			switch {
			case view.Count == 0:
				view.Note = fmt.Sprintf("no screener history for %s; run 'psx-pp-cli snapshot take' on a schedule to accumulate it", sym)
			case view.Count == 1:
				view.First, view.Last = view.Points[0].Value, view.Points[0].Value
				view.Note = "only one snapshot stored; drift needs at least two points to show a trend"
			default:
				view.First = view.Points[0].Value
				view.Last = view.Points[len(view.Points)-1].Value
				if view.First != 0 {
					view.ChangePct = (view.Last - view.First) / view.First * 100
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if view.Count == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n\n", sym, metric)
			for _, p := range view.Points {
				fmt.Fprintf(cmd.OutOrStdout(), "%-22s %12.4f\n", p.TakenAt, p.Value)
			}
			if view.Count > 1 {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%.4f -> %.4f (%.2f%%)\n", view.First, view.Last, view.ChangePct)
			} else if view.Note != "" {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&metric, "metric", "pe", "metric to trace: pe, yield, market-cap, free-float or price")
	cmd.Flags().StringVar(&since, "since", "", "only points newer than this (90d, 4w)")
	cmd.Flags().StringVar(&dbPath, "db", "", "database path")
	return cmd
}
