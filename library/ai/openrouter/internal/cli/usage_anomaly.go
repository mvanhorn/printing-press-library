// Copyright 2026 rick-van-de-laar. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH transcendence-commands: hand-built — z-score cost regression alarm (deterministic, no LLM).

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/openrouter/internal/store"

	"github.com/spf13/cobra"
)

func newUsageAnomalyCmd(flags *rootFlags) *cobra.Command {
	var since, baseline string
	var sigma float64
	var llm bool

	cmd := &cobra.Command{
		Use:         "anomaly",
		Short:       "Flag (model, day) pairs whose cost exceeds Nσ of baseline window",
		Example:     "  openrouter-pp-cli usage anomaly --since 24h --baseline 7d --llm",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			sinceT, err := parseSinceDuration(since)
			if err != nil {
				return usageErr(err)
			}
			baseT, err := parseSinceDuration(baseline)
			if err != nil {
				return usageErr(err)
			}
			dbPath := defaultDBPath("openrouter-pp-cli")
			db, err := store.OpenWithContext(context.Background(), dbPath)
			if err != nil {
				return apiErr(fmt.Errorf("open store: %w", err))
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT model, date, COALESCE(usage,0) FROM activity WHERE date >= ?`,
				baseT.Format("2006-01-02"))
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()

			type key struct{ model, date string }
			daily := map[key]float64{}
			for rows.Next() {
				var m, d string
				var u float64
				if err := rows.Scan(&m, &d, &u); err != nil {
					continue
				}
				daily[key{m, d}] += u
			}
			if len(daily) == 0 {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), []any{}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "insufficient data: activity table empty (run 'openrouter-pp-cli sync activity')")
				return nil
			}

			// Group baseline values per model (excluding the since-window).
			perModel := map[string][]float64{}
			recent := map[string][]struct {
				date string
				cost float64
			}{}
			sinceDate := sinceT.Format("2006-01-02")
			for k, v := range daily {
				if k.date >= sinceDate {
					recent[k.model] = append(recent[k.model], struct {
						date string
						cost float64
					}{k.date, v})
				} else {
					perModel[k.model] = append(perModel[k.model], v)
				}
			}

			type anomaly struct {
				Model  string  `json:"model"`
				Date   string  `json:"date"`
				Cost   float64 `json:"cost_usd"`
				Mean   float64 `json:"baseline_mean"`
				Stddev float64 `json:"baseline_stddev"`
				Z      float64 `json:"z_score"`
			}
			out := []anomaly{}
			for model, recentDays := range recent {
				base := perModel[model]
				if len(base) < 3 {
					continue
				}
				var mean float64
				for _, x := range base {
					mean += x
				}
				mean /= float64(len(base))
				var ss float64
				for _, x := range base {
					ss += (x - mean) * (x - mean)
				}
				stddev := math.Sqrt(ss / float64(len(base)))
				if stddev == 0 {
					continue
				}
				for _, r := range recentDays {
					z := (r.cost - mean) / stddev
					if z > sigma {
						out = append(out, anomaly{Model: model, Date: r.date, Cost: r.cost, Mean: mean, Stddev: stddev, Z: z})
					}
				}
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Z > out[j].Z })

			if flags.asJSON {
				if out == nil {
					out = []anomaly{}
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if llm {
				if len(out) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "no anomalies")
					return nil
				}
				for _, a := range out {
					fmt.Fprintf(cmd.OutOrStdout(), "model=%s date=%s cost=$%.4f mean=$%.4f stddev=$%.4f z=%.2f\n",
						a.Model, a.Date, a.Cost, a.Mean, a.Stddev, a.Z)
				}
				return nil
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no anomalies")
				return nil
			}
			rows2 := make([][]string, 0, len(out))
			for _, a := range out {
				rows2 = append(rows2, []string{a.Model, a.Date, fmt.Sprintf("$%.4f", a.Cost), fmt.Sprintf("%.2f", a.Z)})
			}
			return flags.printTable(cmd, []string{"MODEL", "DATE", "COST", "Z"}, rows2)
		},
	}
	cmd.Flags().StringVar(&since, "since", "24h", "Recent window to evaluate")
	cmd.Flags().StringVar(&baseline, "baseline", "7d", "Baseline window for mean/stddev")
	cmd.Flags().Float64Var(&sigma, "sigma", 2.0, "Z-score threshold for anomaly")
	cmd.Flags().BoolVar(&llm, "llm", false, "Terse k:v output")
	// suppress unused warning on time
	_ = time.Now
	return cmd
}
