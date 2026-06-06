// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelBiomarkerTrendCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var window string

	cmd := &cobra.Command{
		Use:         "trend [biomarker]",
		Short:       "Every value of a biomarker across every round you've ever drawn, with delta from Function-optimal, sparkline, and JSON",
		Long:        "Pulls every measurement of one biomarker (by case-insensitive name or UUID) across every synced round, computes the delta vs Function-optimal midpoint and Quest range, renders an ASCII sparkline in a terminal, and returns structured JSON for agents.",
		Example:     "  function-health-pp-cli biomarker trend ApoB\n  function-health-pp-cli biomarker trend ApoB --json --window 2y",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.Join(args, " ")
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			s, err := openLocalStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer safeCloseStore(s)
			rows, err := loadAllResults(ctx, s)
			if err != nil {
				return err
			}
			matching := filterByBiomarker(rows, query)
			if window != "" {
				matching = filterByWindow(matching, window)
			}
			if len(matching) == 0 {
				return notFoundErr(fmt.Errorf("no synced results for biomarker %q; run `function-health-pp-cli sync` (or check the name with `biomarkers list`)", query))
			}

			latest := matching[len(matching)-1]
			mid := optimalMidpoint(latest)

			type point struct {
				DrawDate       string  `json:"draw_date"`
				RequisitionID  string  `json:"requisition_id"`
				Value          float64 `json:"value"`
				Unit           string  `json:"unit"`
				Status         string  `json:"status"`
				Direction      string  `json:"direction"`
				DeltaFromMid   float64 `json:"delta_from_optimal_midpoint"`
				OptimalLow     float64 `json:"optimal_low"`
				OptimalHigh    float64 `json:"optimal_high"`
				QuestRangeLow  float64 `json:"quest_range_low"`
				QuestRangeHigh float64 `json:"quest_range_high"`
			}
			var points []point
			var values []float64
			for _, r := range matching {
				p := point{
					DrawDate: formatDrawDate(r.DrawDate), RequisitionID: r.RequisitionID,
					Value: r.Value, Unit: r.Unit, Status: r.Status, Direction: r.Direction,
					OptimalLow: r.OptimalLow, OptimalHigh: r.OptimalHigh,
					QuestRangeLow: r.QuestRangeLow, QuestRangeHigh: r.QuestRangeHigh,
				}
				if mid > 0 {
					p.DeltaFromMid = r.Value - mid
				}
				points = append(points, p)
				values = append(values, r.Value)
			}
			result := map[string]any{
				"biomarker":        latest.BiomarkerName,
				"biomarker_id":     latest.BiomarkerID,
				"category":         latest.Category,
				"unit":             latest.Unit,
				"optimal_low":      latest.OptimalLow,
				"optimal_high":     latest.OptimalHigh,
				"quest_range_low":  latest.QuestRangeLow,
				"quest_range_high": latest.QuestRangeHigh,
				"draws":            len(points),
				"slope_per_round":  slopePerRound(matching, 0),
				"history":          points,
			}
			if flags != nil && flags.asJSON {
				return flags.printJSON(cmd, result)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s (%s)  Function-optimal %.1f-%.1f  Quest %.1f-%.1f  %d draws\n",
				latest.BiomarkerName, latest.Unit,
				latest.OptimalLow, latest.OptimalHigh,
				latest.QuestRangeLow, latest.QuestRangeHigh, len(points))
			fmt.Fprintf(w, "  sparkline: %s\n", sparkline(values))
			fmt.Fprintln(w, "  draws (oldest → newest):")
			for _, p := range points {
				fmt.Fprintf(w, "    %-10s  %-12.2f  %-12s  Δmid %+.2f\n",
					p.DrawDate, p.Value, p.Status, p.DeltaFromMid)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Override local database path")
	cmd.Flags().StringVar(&window, "window", "", "Restrict to recent rounds (e.g. 1y, 6mo, 3rounds)")
	return cmd
}

// filterByWindow keeps rows within the requested window. Supports suffixes:
//   - "Nrounds" → keep last N rows
//   - "Ny" / "Nmo" / "Nd" → keep rows within the past N years / months / days
func filterByWindow(rows []resultRow, window string) []resultRow {
	if len(rows) == 0 || window == "" {
		return rows
	}
	window = strings.ToLower(strings.TrimSpace(window))
	if strings.HasSuffix(window, "rounds") {
		var n int
		fmt.Sscanf(window, "%drounds", &n)
		if n > 0 && n < len(rows) {
			return rows[len(rows)-n:]
		}
		return rows
	}
	var nUnit time.Duration
	var n int
	switch {
	case strings.HasSuffix(window, "y"):
		fmt.Sscanf(window, "%dy", &n)
		nUnit = 365 * 24 * time.Hour
	case strings.HasSuffix(window, "mo"):
		fmt.Sscanf(window, "%dmo", &n)
		nUnit = 30 * 24 * time.Hour
	case strings.HasSuffix(window, "d"):
		fmt.Sscanf(window, "%dd", &n)
		nUnit = 24 * time.Hour
	default:
		return rows
	}
	if n == 0 {
		return rows
	}
	cutoff := time.Now().Add(-time.Duration(n) * nUnit)
	var out []resultRow
	for _, r := range rows {
		t, err := time.Parse("2006-01-02", formatDrawDate(r.DrawDate))
		if err != nil {
			out = append(out, r) // unparseable date — keep
			continue
		}
		if t.After(cutoff) {
			out = append(out, r)
		}
	}
	return out
}
