// Copyright 2026 Milos Mladenovic and contributors. Licensed under Apache-2.0. See LICENSE.

// Novel feature: fitness & form trend. Reads the CTL/ATL values intervals.icu
// computes per activity from the live activities endpoint and derives TSB
// (form = CTL - ATL). No training-load math is reimplemented locally; the
// numbers are the platform's own.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/other/intervals-icu/internal/cliutil"

	"github.com/spf13/cobra"
)

type formPoint struct {
	Date string  `json:"date"`
	CTL  float64 `json:"ctl"`
	ATL  float64 `json:"atl"`
	Form float64 `json:"form"`
	Load float64 `json:"load"`
	Name string  `json:"name,omitempty"`
	Type string  `json:"type,omitempty"`
}

type formView struct {
	Days       int         `json:"days"`
	AthleteID  string      `json:"athlete_id"`
	Current    *formPoint  `json:"current"`
	Series     []formPoint `json:"series"`
	ScannedAct int         `json:"scanned_activities"`
	Note       string      `json:"note,omitempty"`
}

func newNovelFormCmd(flags *rootFlags) *cobra.Command {
	var flagDays string

	cmd := &cobra.Command{
		Use:   "form",
		Short: "Show fitness (CTL), fatigue (ATL) and form (TSB) trend from your activities.",
		Long: "Fetch your activities over a window and report the CTL/ATL/TSB trend\n" +
			"intervals.icu computes. Form (TSB) = CTL - ATL: positive means fresh,\n" +
			"strongly negative means accumulated fatigue. Use to judge taper or\n" +
			"overreaching. Do NOT use it for one activity's load; use 'activity get'.",
		Example:     "  intervals-icu-pp-cli form --days 90 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch activities and compute fitness/form trend")
				return nil
			}
			days := parseWindowDays(flagDays, 90)
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			id := athleteID(flags)
			path := replacePathParam("/api/v1/athlete/{id}/activities", "id", id)
			data, err := c.Get(cmd.Context(), path, map[string]string{
				"oldest": localDate(-days),
				"newest": localDate(0),
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var acts []map[string]json.RawMessage
			if err := json.Unmarshal(data, &acts); err != nil {
				return fmt.Errorf("parsing activities: %w", err)
			}
			series := make([]formPoint, 0, len(acts))
			for _, a := range acts {
				ctl, okC := cliutil.ExtractNumber(a, "icu_ctl")
				atl, okA := cliutil.ExtractNumber(a, "icu_atl")
				if !okC || !okA {
					// Need both CTL and ATL for a valid form (TSB) point;
					// otherwise TSB is computed against a phantom zero. Skips
					// Strava stubs and not-yet-analyzed activities.
					continue
				}
				load, _ := cliutil.ExtractNumber(a, "icu_training_load")
				p := formPoint{
					Date: jsonStr(a, "start_date_local"),
					CTL:  round1(ctl),
					ATL:  round1(atl),
					Form: round1(ctl - atl),
					Load: load,
					Name: jsonStr(a, "name"),
					Type: jsonStr(a, "type"),
				}
				if p.Date == "" {
					p.Date = jsonStr(a, "start_date")
				}
				series = append(series, p)
			}
			sort.SliceStable(series, func(i, j int) bool { return series[i].Date < series[j].Date })
			view := formView{
				Days:       days,
				AthleteID:  id,
				Series:     series,
				ScannedAct: len(acts),
			}
			if n := len(series); n > 0 {
				cur := series[n-1]
				view.Current = &cur
			} else {
				view.Note = fmt.Sprintf("no activities with computed load in the last %d days; widen --days or sync more history", days)
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return flags.printJSON(cmd, view)
			}
			if view.Current != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Fitness (CTL): %.1f   Fatigue (ATL): %.1f   Form (TSB): %.1f   [%s]\n",
					view.Current.CTL, view.Current.ATL, view.Current.Form, view.Current.Date)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%-12s %7s %7s %7s %7s\n", "DATE", "CTL", "ATL", "FORM", "LOAD")
			for _, p := range series {
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %7.1f %7.1f %7.1f %7.0f\n", p.Date, p.CTL, p.ATL, p.Form, p.Load)
			}
			if view.Note != "" {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDays, "days", "", "Window to analyze, e.g. 90d, 6w, or a bare number of days (default 90)")
	return cmd
}
