// Copyright 2026 erash11 and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type heatmapView struct {
	Activity string      `json:"activity"`
	Date     string      `json:"date"`
	Metric   string      `json:"metric"`
	Periods  []string    `json:"periods"`
	Athletes []string    `json:"athletes"`
	Values   [][]float64 `json:"values"`
}

func newNovelHeatmapCmd(flags *rootFlags) *cobra.Command {
	var flagActivity string
	var flagMetric string
	var flagAllAthletes bool

	cmd := &cobra.Command{
		Use:   "heatmap",
		Short: "Athlete-by-period intensity matrix for one session (ANSI heatmap or JSON matrix)",
		Long: strings.TrimSpace(`
Pivots period-grouped stats into an athlete x period intensity matrix,
surfacing where fatigue compounded during a session. Renders an ANSI heatmap
on a terminal and a JSON matrix under --json/--agent.

Use this command for within-one-session period breakdown.
Do NOT use it to compare two different sessions; use 'diff'.`),
		Example:     "  catapult-openfield-connect-pp-cli heatmap --activity last --metric meterage_per_minute --all-athletes --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--activity=last"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "heatmap")
			}
			if flagActivity == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--activity is required (ID, date, or 'last')"))
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			now := time.Now()
			actIndex, ordered, err := fetchActivityIndex(ctx, c, now.AddDate(-1, 0, 0), now)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			act, err := resolveActivityArg(flagActivity, actIndex, ordered)
			if err != nil {
				return err
			}
			rows, err := fetchStatsRows(ctx, c, []string{flagMetric}, []string{"athlete", "period"},
				[]map[string]any{statsFilter("activity_id", "=", act.ID)})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			type cell struct {
				athlete, period string
				val             float64
			}
			var cells []cell
			periodSet := map[string]bool{}
			athleteSet := map[string]bool{}
			for _, r := range rows {
				athlete := rowString(r, "athlete_name")
				period := rowString(r, "period_name")
				if period == "" {
					period = rowString(r, "period_id")
				}
				v, ok := rowNum(r, flagMetric)
				if athlete == "" || period == "" || !ok {
					continue
				}
				cells = append(cells, cell{athlete: athlete, period: period, val: v})
				periodSet[period] = true
				athleteSet[athlete] = true
			}
			if len(cells) == 0 {
				return notFoundErr(fmt.Errorf("no period-level stats for %s (metric %s)", act.Name, flagMetric))
			}
			periods := make([]string, 0, len(periodSet))
			for p := range periodSet {
				periods = append(periods, p)
			}
			sort.Strings(periods)
			athletes := make([]string, 0, len(athleteSet))
			for a := range athleteSet {
				athletes = append(athletes, a)
			}
			sort.Strings(athletes)
			pIdx := map[string]int{}
			for i, p := range periods {
				pIdx[p] = i
			}
			aIdx := map[string]int{}
			for i, a := range athletes {
				aIdx[a] = i
			}
			values := make([][]float64, len(athletes))
			for i := range values {
				values[i] = make([]float64, len(periods))
			}
			var vmin, vmax float64
			first := true
			for _, cl := range cells {
				values[aIdx[cl.athlete]][pIdx[cl.period]] = round2(cl.val)
				if first || cl.val < vmin {
					vmin = cl.val
				}
				if first || cl.val > vmax {
					vmax = cl.val
				}
				first = false
			}
			view := heatmapView{Activity: act.Name, Date: dayKey(act.Start), Metric: flagMetric, Periods: periods, Athletes: athletes, Values: values}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s (%s) — %s\n\n", act.Name, view.Date, flagMetric)
			fmt.Fprintf(cmd.OutOrStdout(), "%-24s", "")
			for _, p := range periods {
				fmt.Fprintf(cmd.OutOrStdout(), " %-10.10s", p)
			}
			fmt.Fprintln(cmd.OutOrStdout())
			span := vmax - vmin
			for i, a := range athletes {
				fmt.Fprintf(cmd.OutOrStdout(), "%-24s", a)
				for j := range periods {
					v := values[i][j]
					blockText := fmt.Sprintf(" %-10.1f", v)
					if span > 0 && colorEnabled() {
						q := (v - vmin) / span
						switch {
						case q >= 0.75:
							blockText = red(blockText)
						case q >= 0.5:
							blockText = yellow(blockText)
						case q > 0:
							blockText = green(blockText)
						}
					}
					fmt.Fprint(cmd.OutOrStdout(), blockText)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagActivity, "activity", "", "Activity reference: ID, date (2026-03-15), or 'last'")
	cmd.Flags().StringVar(&flagMetric, "metric", "meterage_per_minute", "Metric slug for intensity")
	cmd.Flags().BoolVar(&flagAllAthletes, "all-athletes", true, "Include every athlete in the session (always on; accepted for compatibility)")
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newNovelHeatmapCmd(flags))
	})
}
