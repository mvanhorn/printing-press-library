// Copyright 2026 erash11 and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/catapult-openfield-connect/internal/cliutil"

	"github.com/spf13/cobra"
)

type diffView struct {
	RefA      string    `json:"ref_a"`
	RefB      string    `json:"ref_b"`
	ActivityA string    `json:"activity_a"`
	ActivityB string    `json:"activity_b"`
	DateA     string    `json:"date_a"`
	DateB     string    `json:"date_b"`
	Rows      []diffRow `json:"rows"`
}

type diffRow struct {
	AthleteID   string   `json:"athlete_id"`
	AthleteName string   `json:"athlete_name"`
	Metric      string   `json:"metric"`
	ValueA      float64  `json:"value_a"`
	ValueB      float64  `json:"value_b"`
	Delta       float64  `json:"delta"`
	DeltaPct    *float64 `json:"delta_pct"`
}

func newNovelDiffCmd(flags *rootFlags) *cobra.Command {
	var flagSquad bool
	var flagMetrics string
	var flagAthlete string

	cmd := &cobra.Command{
		Use:   "diff <activity-a> <activity-b>",
		Short: "Side-by-side comparison of two sessions: absolute and % change per metric per athlete",
		Long: strings.TrimSpace(`
Compares two activities (by ID, date like 2026-03-15 or 20260315, or the
keywords 'last'/'prev') and prints per-athlete, per-metric deltas.

Use this command to compare TWO sessions.
Do NOT use it for period-by-period breakdown within one session; use 'heatmap'.`),
		Example:     "  catapult-openfield-connect-pp-cli diff last prev --squad --metrics total_player_load,max_vel --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<activity-a>=last;<activity-b>=prev"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("two activity references are required (ID, date, 'last', or 'prev')"))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "diff")
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			metrics, err := cliutil.ParseStringList(flagMetrics)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --metrics list: %w", err))
			}
			if len(metrics) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--metrics must name at least one metric slug"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			now := time.Now()
			actIndex, ordered, err := fetchActivityIndex(ctx, c, now.AddDate(-1, 0, 0), now)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			actA, err := resolveActivityArg(args[0], actIndex, ordered)
			if err != nil {
				return err
			}
			actB, err := resolveActivityArg(args[1], actIndex, ordered)
			if err != nil {
				return err
			}
			if actA.ID == actB.ID {
				return usageErr(fmt.Errorf("both references resolve to the same activity (%s)", actA.Name))
			}
			rows, err := fetchStatsRows(ctx, c, metrics, []string{"athlete", "activity"},
				[]map[string]any{statsFilter("activity_id", "=", actA.ID, actB.ID)})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			type pair struct {
				name string
				a, b map[string]float64
			}
			perAthlete := map[string]*pair{}
			for _, r := range rows {
				aid := rowString(r, "athlete_id")
				if aid == "" {
					continue
				}
				p := perAthlete[aid]
				if p == nil {
					p = &pair{name: rowString(r, "athlete_name"), a: map[string]float64{}, b: map[string]float64{}}
					perAthlete[aid] = p
				}
				target := p.a
				if rowString(r, "activity_id") == actB.ID {
					target = p.b
				}
				for _, m := range metrics {
					if v, ok := rowNum(r, m); ok {
						target[m] += v
					}
				}
			}
			results := make([]diffRow, 0)
			for aid, p := range perAthlete {
				if !nameMatches(p.name, flagAthlete) {
					continue
				}
				for _, m := range metrics {
					va, vb := p.a[m], p.b[m]
					if va == 0 && vb == 0 {
						continue
					}
					var pct *float64
					if va != 0 {
						v := round2((vb - va) / va * 100)
						pct = &v
					}
					results = append(results, diffRow{
						AthleteID: aid, AthleteName: p.name, Metric: m,
						ValueA: round2(va), ValueB: round2(vb),
						Delta: round2(vb - va), DeltaPct: pct,
					})
				}
			}
			sort.Slice(results, func(i, j int) bool {
				if results[i].AthleteName != results[j].AthleteName {
					return results[i].AthleteName < results[j].AthleteName
				}
				return results[i].Metric < results[j].Metric
			})
			view := diffView{
				RefA: args[0], RefB: args[1],
				ActivityA: actA.Name, ActivityB: actB.Name,
				DateA: dayKey(actA.Start), DateB: dayKey(actB.Start),
				Rows: results,
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No overlapping stats between %s and %s for metrics %s.\n", actA.Name, actB.Name, flagMetrics)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "A: %s (%s)   B: %s (%s)\n\n", actA.Name, dayKey(actA.Start), actB.Name, dayKey(actB.Start))
			fmt.Fprintf(cmd.OutOrStdout(), "%-24s %-24s %10s %10s %9s\n", "ATHLETE", "METRIC", "A", "B", "Δ%")
			for _, r := range results {
				pctText := "     new"
				if r.DeltaPct != nil {
					pctText = fmt.Sprintf("%7.1f%%", *r.DeltaPct)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-24s %-24s %10.1f %10.1f %s\n", r.AthleteName, r.Metric, r.ValueA, r.ValueB, pctText)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagSquad, "squad", false, "Show the full squad (default)")
	cmd.Flags().StringVar(&flagMetrics, "metrics", "total_player_load", "Comma-separated metric slugs to compare")
	cmd.Flags().StringVar(&flagAthlete, "athlete", "", "Filter to athletes whose name contains this text")
	return cmd
}
