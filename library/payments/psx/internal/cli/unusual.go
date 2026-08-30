// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/psx/internal/cliutil"
)

// newNovelUnusualCmd ranks names by deviation from their OWN trailing history
// rather than by raw move. The portal's movers list has no baseline, so a thin
// scrip doubling on tiny size outranks a genuine institutional print there.
// The statistic is median + MAD: mechanical, explainable, no model involved.
func newNovelUnusualCmd(flags *rootFlags) *cobra.Command {
	var baseline, dbPath, dimension string
	var top int
	cmd := &cobra.Command{
		Use:   "unusual",
		Short: "Find names trading abnormally against their own trailing history, not just the day's biggest movers.",
		Long: "Use this command to find names trading abnormally versus their own history.\n" +
			"Do NOT use it for the plain top gainers/losers/most-active ranking; use 'market performers' instead.\n" +
			"Scores each symbol by robust z-score (median absolute deviation) over stored snapshots.",
		Example:     "  psx-pp-cli unusual --baseline 30d --top 20 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "unusual")
			}
			col := "volume"
			switch strings.ToLower(strings.TrimSpace(dimension)) {
			case "volume", "":
				col = "volume"
			case "price":
				col = "current"
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--dimension must be volume or price (got %q)", dimension))
			}
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
			if strings.TrimSpace(baseline) != "" {
				d, err := cliutil.ParseDurationLoose(baseline)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--baseline %q is not a duration (try 30d): %w", baseline, err))
				}
				cutoff = nowUTC().Add(-d).Format(snapshotTimeFormat)
			}
			times, err := listSnapshotTimes(ctx, s, snapshotKindMarket)
			if err != nil {
				return err
			}
			type scored struct {
				Symbol  string  `json:"symbol"`
				Latest  float64 `json:"latest"`
				Median  float64 `json:"median"`
				MAD     float64 `json:"mad"`
				ZScore  float64 `json:"z_score"`
				Samples int     `json:"samples"`
			}
			view := struct {
				Dimension string   `json:"dimension"`
				Baseline  string   `json:"baseline,omitempty"`
				Snapshots int      `json:"snapshots"`
				Count     int      `json:"count"`
				Results   []scored `json:"results"`
				Note      string   `json:"note,omitempty"`
			}{Dimension: col, Baseline: baseline, Snapshots: len(times), Results: make([]scored, 0)}

			if len(times) < 3 {
				view.Note = fmt.Sprintf("only %d market snapshot(s) stored; a baseline needs at least 3. Run 'psx-pp-cli snapshot take' on a schedule.", len(times))
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}

			// Collect per-symbol series across snapshots, newest last.
			hist := map[string][]float64{}
			for i := len(times) - 1; i >= 0; i-- {
				if cutoff != "" && times[i] < cutoff {
					continue
				}
				snap, err := loadSnapshot(ctx, s, snapshotKindMarket, times[i])
				if err != nil {
					return err
				}
				for sym, row := range snap {
					if v, ok := parseNum(row[col]); ok {
						hist[sym] = append(hist[sym], v)
					}
				}
			}
			for sym, series := range hist {
				if len(series) < 3 {
					continue
				}
				latest := series[len(series)-1]
				prior := series[:len(series)-1]
				med := median(prior)
				mad := medianAbsDev(prior, med)
				if mad == 0 {
					continue // flat history: no dispersion to measure against
				}
				z := 0.6745 * (latest - med) / mad
				view.Results = append(view.Results, scored{
					Symbol: sym, Latest: latest, Median: med, MAD: mad,
					ZScore: z, Samples: len(series),
				})
			}
			sort.Slice(view.Results, func(i, j int) bool {
				return math.Abs(view.Results[i].ZScore) > math.Abs(view.Results[j].ZScore)
			})
			if top > 0 && len(view.Results) > top {
				view.Results = view.Results[:top]
			}
			view.Count = len(view.Results)
			if view.Count == 0 {
				view.Note = "no symbol had enough dispersion in the stored window to score"
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if view.Count == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %14s %14s %9s %8s\n", "SYMBOL", "LATEST", "MEDIAN", "Z", "N")
			for _, r := range view.Results {
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %14.2f %14.2f %9.2f %8d\n", r.Symbol, r.Latest, r.Median, r.ZScore, r.Samples)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&baseline, "baseline", "30d", "trailing window for the baseline (30d, 4w)")
	cmd.Flags().StringVar(&dimension, "dimension", "volume", "what to score: volume or price")
	cmd.Flags().IntVar(&top, "top", 20, "maximum rows to return (0 = all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "database path")
	return cmd
}

// median returns the middle value of a copy of xs. Robust to the outliers that
// make a plain mean useless on thin PSX scrips.
func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	cp := append([]float64(nil), xs...)
	sort.Float64s(cp)
	n := len(cp)
	if n%2 == 1 {
		return cp[n/2]
	}
	return (cp[n/2-1] + cp[n/2]) / 2
}

// medianAbsDev is the median of absolute deviations from med.
func medianAbsDev(xs []float64, med float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	dev := make([]float64, 0, len(xs))
	for _, x := range xs {
		dev = append(dev, math.Abs(x-med))
	}
	return median(dev)
}
