// Copyright 2026 erash11 and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type pbRow struct {
	AthleteID    string  `json:"athlete_id"`
	AthleteName  string  `json:"athlete_name"`
	PB           float64 `json:"pb"`
	Latest       float64 `json:"latest"`
	ReadinessPct float64 `json:"readiness_pct"`
	Trend        string  `json:"trend"`
	Sessions     int     `json:"recent_sessions"`
}

func newNovelPbCmd(flags *rootFlags) *cobra.Command {
	var flagAthlete string
	var flagSquad bool
	var flagMetric string
	var flagVsPeak bool
	var flagRecent int
	var flagPBWindow int

	cmd := &cobra.Command{
		Use:   "pb",
		Short: "Recent session values vs each athlete's personal best over the PB window, with readiness % and trend",
		Long: strings.TrimSpace(`
Compares each athlete's most recent session value against their all-time
personal best, printing a readiness index (% of PB) and 3-session trend
arrows over the recent-session window.

Use this command to compare an athlete against their OWN historical best.
Do NOT use it to compare against other athletes; use 'benchmark'.`),
		Example:     "  catapult-openfield-connect-pp-cli pb --metric max_vel --vs-peak --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "pb")
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
			pbByAthlete, err := fetchPB(ctx, c, flagMetric, flagPBWindow)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			recent, err := fetchStatsRows(ctx, c, []string{flagMetric}, []string{"athlete", "activity"},
				[]map[string]any{statsFilter("lastActivities", "=", fmt.Sprint(flagRecent))})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			type sess struct {
				day string
				val float64
			}
			perAthlete := map[string]*struct {
				name string
				sess []sess
			}{}
			for _, r := range recent {
				aid := rowString(r, "athlete_id")
				val, ok := rowNum(r, flagMetric)
				if aid == "" || !ok {
					continue
				}
				a := perAthlete[aid]
				if a == nil {
					a = &struct {
						name string
						sess []sess
					}{name: rowString(r, "athlete_name")}
					perAthlete[aid] = a
				}
				a.sess = append(a.sess, sess{day: rowDate(r), val: val})
			}
			results := make([]pbRow, 0, len(perAthlete))
			for aid, a := range perAthlete {
				if !nameMatches(a.name, flagAthlete) {
					continue
				}
				sort.Slice(a.sess, func(i, j int) bool { return a.sess[i].day < a.sess[j].day })
				pb := pbByAthlete[aid]
				latest := a.sess[len(a.sess)-1].val
				if latest > pb {
					pb = latest
				}
				readiness := 0.0
				if pb > 0 {
					readiness = latest / pb * 100
				}
				trend := ""
				n := len(a.sess)
				for i := max(1, n-3); i < n; i++ {
					switch {
					case a.sess[i].val > a.sess[i-1].val:
						trend += "↑"
					case a.sess[i].val < a.sess[i-1].val:
						trend += "↓"
					default:
						trend += "→"
					}
				}
				results = append(results, pbRow{
					AthleteID:    aid,
					AthleteName:  a.name,
					PB:           round2(pb),
					Latest:       round2(latest),
					ReadinessPct: round2(readiness),
					Trend:        trend,
					Sessions:     n,
				})
			}
			sort.Slice(results, func(i, j int) bool { return results[i].ReadinessPct > results[j].ReadinessPct })
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No recent sessions found. Check the metric slug with 'search --type parameters'.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-24s %10s %10s %10s  %s\n", "ATHLETE", "PB", "LATEST", "% OF PB", "TREND")
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%-24s %10.2f %10.2f %9.1f%%  %s\n", r.AthleteName, r.PB, r.Latest, r.ReadinessPct, r.Trend)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagAthlete, "athlete", "", "Filter to athletes whose name contains this text")
	cmd.Flags().BoolVar(&flagSquad, "squad", false, "Show the full squad (default when --athlete is not set)")
	cmd.Flags().StringVar(&flagMetric, "metric", "max_vel", "Metric slug — max-style metrics like max_vel; additive metrics reflect the window aggregate, not a per-session best")
	cmd.Flags().BoolVar(&flagVsPeak, "vs-peak", true, "Compare against the peak in the PB window (always on; accepted for compatibility)")
	cmd.Flags().IntVar(&flagRecent, "recent", 10, "Recent activities to inspect for latest value and trend")
	cmd.Flags().IntVar(&flagPBWindow, "pb-window", defaultPBWindowActivities, "Activities to search for the personal best (60 \u2248 a season)")
	return cmd
}
