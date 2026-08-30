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

type benchmarkRow struct {
	AthleteID      string  `json:"athlete_id"`
	AthleteName    string  `json:"athlete_name"`
	Position       string  `json:"position"`
	AvgPerSession  float64 `json:"avg_per_session"`
	PercentileRank float64 `json:"percentile_rank"`
	SquadMedian    float64 `json:"squad_median"`
	PositionMedian float64 `json:"position_median"`
	Outlier        bool    `json:"outlier"`
	Sessions       int     `json:"sessions"`
}

func newNovelBenchmarkCmd(flags *rootFlags) *cobra.Command {
	var flagMetric string
	var flagPosition string
	var flagPercentile bool
	var flagAthlete string
	var flagDays int

	cmd := &cobra.Command{
		Use:   "benchmark",
		Short: "Rank athletes against positional peers: percentile, squad median, position median",
		Long: strings.TrimSpace(`
Ranks athletes by their average per-session value for a metric over the
window, against positional peers (or the whole squad), with percentile rank,
medians, and IQR outlier flags.

Use this command to rank an athlete against positional peers.
Do NOT use it for self-comparison against personal history; use 'pb'.`),
		Example:     "  catapult-openfield-connect-pp-cli benchmark --metric total_distance --percentile --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "benchmark")
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
			start := now.AddDate(0, 0, -flagDays+1)
			rows, err := fetchStatsWindow(ctx, c, []string{flagMetric}, start, now)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			athletes, err := fetchAthleteIndex(ctx, c)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			type acc struct {
				total    float64
				sessions int
				name     string
			}
			perAthlete := map[string]*acc{}
			for _, r := range rows {
				aid := rowString(r, "athlete_id")
				val, ok := rowNum(r, flagMetric)
				if aid == "" || !ok {
					continue
				}
				a := perAthlete[aid]
				if a == nil {
					a = &acc{name: rowString(r, "athlete_name")}
					perAthlete[aid] = a
				}
				a.total += val
				a.sessions++
			}
			// build rows with position, compute cohort stats
			all := make([]benchmarkRow, 0, len(perAthlete))
			var squadVals []float64
			posVals := map[string][]float64{}
			for aid, a := range perAthlete {
				if a.sessions == 0 {
					continue
				}
				avg := a.total / float64(a.sessions)
				pos := athletes[aid].Position
				name := a.name
				if name == "" {
					name = athletes[aid].Name
				}
				all = append(all, benchmarkRow{AthleteID: aid, AthleteName: name, Position: pos, AvgPerSession: round2(avg), Sessions: a.sessions})
				squadVals = append(squadVals, avg)
				posVals[pos] = append(posVals[pos], avg)
			}
			squadMed := median(squadVals)
			// restrict cohort if --position given
			cohort := all
			if flagPosition != "" {
				cohort = nil
				for _, r := range all {
					if strings.EqualFold(r.Position, flagPosition) {
						cohort = append(cohort, r)
					}
				}
				if len(cohort) == 0 {
					return notFoundErr(fmt.Errorf("no athletes with position %q had data in the last %d days", flagPosition, flagDays))
				}
			}
			sort.Slice(cohort, func(i, j int) bool { return cohort[i].AvgPerSession < cohort[j].AvgPerSession })
			cohortVals := make([]float64, len(cohort))
			for i, r := range cohort {
				cohortVals[i] = r.AvgPerSession
			}
			q1 := median(cohortVals[:len(cohortVals)/2])
			q3 := median(cohortVals[(len(cohortVals)+1)/2:])
			iqr := q3 - q1
			results := make([]benchmarkRow, 0, len(cohort))
			for i := range cohort {
				r := cohort[i]
				if len(cohort) > 1 {
					r.PercentileRank = round2(float64(i) / float64(len(cohort)-1) * 100)
				} else {
					r.PercentileRank = 100
				}
				r.SquadMedian = round2(squadMed)
				r.PositionMedian = round2(median(posVals[r.Position]))
				if iqr > 0 && (r.AvgPerSession < q1-1.5*iqr || r.AvgPerSession > q3+1.5*iqr) {
					r.Outlier = true
				}
				if !nameMatches(r.AthleteName, flagAthlete) {
					continue
				}
				results = append(results, r)
			}
			sort.Slice(results, func(i, j int) bool { return results[i].PercentileRank > results[j].PercentileRank })
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No athletes with data in the window.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-24s %-16s %12s %6s %12s %12s %s\n", "ATHLETE", "POSITION", "AVG/SESSION", "PCT", "SQUAD MED", "POS MED", "OUTLIER")
			for _, r := range results {
				flag := ""
				if r.Outlier {
					flag = red("*")
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-24s %-16s %12.1f %5.0f%% %12.1f %12.1f %s\n", r.AthleteName, r.Position, r.AvgPerSession, r.PercentileRank, r.SquadMedian, r.PositionMedian, flag)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagMetric, "metric", "total_distance", "Metric slug to benchmark")
	cmd.Flags().StringVar(&flagPosition, "position", "", "Restrict the cohort to one position (exact name)")
	cmd.Flags().BoolVar(&flagPercentile, "percentile", true, "Show percentile ranks (always on; accepted for compatibility)")
	cmd.Flags().StringVar(&flagAthlete, "athlete", "", "Highlight-filter to athletes whose name contains this text")
	cmd.Flags().IntVar(&flagDays, "days", 28, "Window in days")
	return cmd
}
