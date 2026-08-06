// Copyright 2026 erash11 and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type acwrRow struct {
	AthleteID   string  `json:"athlete_id"`
	AthleteName string  `json:"athlete_name"`
	Acute7d     float64 `json:"acute_7d"`
	Chronic28d  float64 `json:"chronic_28d_weekly_avg"`
	ACWR        float64 `json:"acwr"`
	RiskZone    string  `json:"risk_zone"`
	Sessions    int     `json:"sessions_28d"`
}

func newNovelAcwrCmd(flags *rootFlags) *cobra.Command {
	var flagSquad bool
	var flagAthlete string
	var flagMetric string
	var flagFlagRisk bool

	cmd := &cobra.Command{
		Use:   "acwr",
		Short: "Acute:chronic workload ratios with safe/caution/danger risk zones",
		Long: strings.TrimSpace(`
Computes each athlete's 7-day acute load against their 28-day chronic load
(weekly average) for a metric, bucketing the ratio into risk zones:
caution-low (<0.8), safe (0.8-1.3), caution (1.3-1.5), danger (>1.5).

Use this command for acute:chronic workload ratios and risk zones.
Do NOT use it for weekly summaries; use 'report'.
Do NOT use it for a single rehabbing athlete's progression; use 'rtp'.`),
		Example:     "  catapult-openfield-connect-pp-cli acwr --squad --metric total_player_load --flag-risk --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "acwr")
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
			start := now.AddDate(0, 0, -(acwrChronicDays - 1))
			rows, err := fetchStatsWindow(ctx, c, []string{flagMetric}, start, now)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			// per athlete: daily load buckets keyed by date
			type acc struct {
				name     string
				byDay    map[string]float64
				sessions int
			}
			perAthlete := map[string]*acc{}
			for _, r := range rows {
				aid := rowString(r, "athlete_id")
				val, ok := rowNum(r, flagMetric)
				if aid == "" || !ok {
					continue
				}
				day := rowDate(r)
				if day == "" {
					continue
				}
				a := perAthlete[aid]
				if a == nil {
					a = &acc{name: rowString(r, "athlete_name"), byDay: map[string]float64{}}
					perAthlete[aid] = a
				}
				a.byDay[day] += val
				a.sessions++
			}
			results := make([]acwrRow, 0, len(perAthlete))
			for aid, a := range perAthlete {
				if !nameMatches(a.name, flagAthlete) {
					continue
				}
				acute, chronicWeekly, ratio := computeACWR(a.byDay, now)
				row := acwrRow{
					AthleteID:   aid,
					AthleteName: a.name,
					Acute7d:     round2(acute),
					Chronic28d:  round2(chronicWeekly),
					ACWR:        round2(ratio),
					RiskZone:    riskZone(ratio),
					Sessions:    a.sessions,
				}
				if flagFlagRisk && row.RiskZone == "safe" {
					continue
				}
				results = append(results, row)
			}
			sortAcwr(results)
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No athletes with load data in the last 28 days. Run 'sync' or widen your data window.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-24s %10s %10s %6s  %s\n", "ATHLETE", "ACUTE 7D", "CHRONIC", "ACWR", "ZONE")
			for _, r := range results {
				zone := r.RiskZone
				switch zone {
				case "safe":
					zone = green(zone)
				case "caution", "caution-low":
					zone = yellow(zone)
				case "danger":
					zone = red(zone)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-24s %10.1f %10.1f %6.2f  %s\n", r.AthleteName, r.Acute7d, r.Chronic28d, r.ACWR, zone)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagSquad, "squad", false, "Show the full squad (default when --athlete is not set)")
	cmd.Flags().StringVar(&flagAthlete, "athlete", "", "Filter to athletes whose name contains this text")
	cmd.Flags().StringVar(&flagMetric, "metric", "total_player_load", "Metric slug to compute ACWR over (see 'search --type parameters')")
	cmd.Flags().BoolVar(&flagFlagRisk, "flag-risk", false, "Only show athletes outside the safe zone")
	return cmd
}
