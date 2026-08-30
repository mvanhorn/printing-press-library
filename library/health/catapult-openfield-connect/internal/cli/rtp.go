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

type rtpRow struct {
	Date           string  `json:"date"`
	Activity       string  `json:"activity"`
	AthleteValue   float64 `json:"athlete_value"`
	ReferenceValue float64 `json:"reference_value"`
	ProgressionPct float64 `json:"progression_pct"`
	MeetsThreshold bool    `json:"meets_threshold"`
}

type rtpView struct {
	Athlete       string   `json:"athlete"`
	Metric        string   `json:"metric"`
	Reference     string   `json:"reference"`
	Threshold     float64  `json:"threshold_pct"`
	Sessions      []rtpRow `json:"sessions"`
	LatestPct     float64  `json:"latest_pct"`
	ClearedStreak int      `json:"consecutive_sessions_at_threshold"`
}

func newNovelRtpCmd(flags *rootFlags) *cobra.Command {
	var flagAthlete string
	var flagTargetMetric string
	var flagVsSquad bool
	var flagBaseline string
	var flagThreshold float64
	var flagDays int

	cmd := &cobra.Command{
		Use:   "rtp",
		Short: "Return-to-play load progression vs squad average or a pre-injury baseline",
		Long: strings.TrimSpace(`
Tracks an injured athlete's per-session load as a percentage of the squad
average for the same session, or of the athlete's own pre-injury baseline
window (--baseline 2026-01-01..2026-02-15).

Use this command for one athlete's rehab progression vs squad or baseline.
Do NOT use it for squad-wide risk screening; use 'acwr'.`),
		Example:     "  catapult-openfield-connect-pp-cli rtp --athlete top --target-metric total_player_load --vs-squad --threshold 85 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "rtp")
			}
			if flagAthlete == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--athlete is required"))
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
			rows, err := fetchStatsWindow(ctx, c, []string{flagTargetMetric}, start, now)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			actIndex := indexFromRows(rows)
			var target athleteMeta
			if strings.EqualFold(flagAthlete, "top") {
				// "top" = the athlete with the highest total load in the window
				totals := map[string]float64{}
				names := map[string]string{}
				for _, r := range rows {
					aid := rowString(r, "athlete_id")
					if aid == "" {
						continue
					}
					if v, ok := rowNum(r, flagTargetMetric); ok {
						totals[aid] += v
						names[aid] = rowString(r, "athlete_name")
					}
				}
				for aid, v := range totals {
					if v > totals[target.ID] || target.ID == "" {
						target = athleteMeta{ID: aid, Name: names[aid]}
					}
				}
				if target.ID == "" {
					return notFoundErr(fmt.Errorf("no athletes with data in the last %d days", flagDays))
				}
			} else {
				athletes, err := fetchAthleteIndex(ctx, c)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				target, err = matchAthlete(athletes, flagAthlete)
				if err != nil {
					return err
				}
			}
			// reference: baseline avg, or per-activity squad average
			reference := "squad-average"
			baselineAvg := 0.0
			if flagBaseline != "" {
				parts := strings.SplitN(flagBaseline, "..", 2)
				if len(parts) != 2 {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--baseline must be <from>..<to>, e.g. 2026-01-01..2026-02-15"))
				}
				bFrom, errF := time.Parse("2006-01-02", parts[0])
				bTo, errT := time.Parse("2006-01-02", parts[1])
				if errF != nil || errT != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--baseline dates must be YYYY-MM-DD"))
				}
				if bFrom.After(bTo) {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--baseline start %s is after end %s", parts[0], parts[1]))
				}
				bRows, err := fetchStatsWindow(ctx, c, []string{flagTargetMetric}, bFrom, bTo)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var total float64
				var n int
				for _, r := range bRows {
					if rowString(r, "athlete_id") != target.ID {
						continue
					}
					if v, ok := rowNum(r, flagTargetMetric); ok {
						total += v
						n++
					}
				}
				if n == 0 {
					return notFoundErr(fmt.Errorf("no baseline sessions for %s in %s", target.Name, flagBaseline))
				}
				baselineAvg = total / float64(n)
				reference = "baseline " + flagBaseline
			}
			type sess struct {
				actID string
				val   float64
			}
			var athleteSessions []sess
			squadTotals := map[string]struct {
				total float64
				n     int
			}{}
			for _, r := range rows {
				aid := rowString(r, "athlete_id")
				actID := rowString(r, "activity_id")
				v, ok := rowNum(r, flagTargetMetric)
				if !ok || actID == "" {
					continue
				}
				if aid == target.ID {
					athleteSessions = append(athleteSessions, sess{actID: actID, val: v})
				}
				agg := squadTotals[actID]
				agg.total += v
				agg.n++
				squadTotals[actID] = agg
			}
			if len(athleteSessions) == 0 {
				return notFoundErr(fmt.Errorf("no sessions for %s in the last %d days", target.Name, flagDays))
			}
			sort.Slice(athleteSessions, func(i, j int) bool {
				return actIndex[athleteSessions[i].actID].Start.Before(actIndex[athleteSessions[j].actID].Start)
			})
			view := rtpView{Athlete: target.Name, Metric: flagTargetMetric, Reference: reference, Threshold: flagThreshold}
			streak := 0
			for _, s := range athleteSessions {
				ref := baselineAvg
				if flagBaseline == "" {
					agg := squadTotals[s.actID]
					if agg.n > 0 {
						ref = agg.total / float64(agg.n)
					}
				}
				pct := 0.0
				if ref > 0 {
					pct = s.val / ref * 100
				}
				meets := pct >= flagThreshold
				if meets {
					streak++
				} else {
					streak = 0
				}
				meta := actIndex[s.actID]
				view.Sessions = append(view.Sessions, rtpRow{
					Date: dayKey(meta.Start), Activity: meta.Name,
					AthleteValue: round2(s.val), ReferenceValue: round2(ref),
					ProgressionPct: round2(pct), MeetsThreshold: meets,
				})
				view.LatestPct = round2(pct)
			}
			view.ClearedStreak = streak
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s — %s vs %s (threshold %.0f%%)\n\n", view.Athlete, view.Metric, view.Reference, view.Threshold)
			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-28s %10s %10s %8s\n", "DATE", "ACTIVITY", "ATHLETE", "REF", "%")
			for _, s := range view.Sessions {
				pct := fmt.Sprintf("%7.1f%%", s.ProgressionPct)
				if s.MeetsThreshold {
					pct = green(pct)
				} else {
					pct = yellow(pct)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-28s %10.1f %10.1f %s\n", s.Date, s.Activity, s.AthleteValue, s.ReferenceValue, pct)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nLatest: %.1f%% of %s — %d consecutive session(s) at threshold\n", view.LatestPct, view.Reference, view.ClearedStreak)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagAthlete, "athlete", "", "Athlete name (full or partial), or 'top' for the highest-load athlete in the window")
	cmd.Flags().StringVar(&flagTargetMetric, "target-metric", "total_player_load", "Metric slug to track")
	cmd.Flags().BoolVar(&flagVsSquad, "vs-squad", true, "Compare against the squad average per session (always on unless --baseline is set)")
	cmd.Flags().StringVar(&flagBaseline, "baseline", "", "Pre-injury baseline window <from>..<to> (overrides --vs-squad)")
	cmd.Flags().Float64Var(&flagThreshold, "threshold", 85, "Progression threshold percent")
	cmd.Flags().IntVar(&flagDays, "days", 28, "Window in days")
	return cmd
}
