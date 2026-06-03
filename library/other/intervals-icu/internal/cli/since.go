// Copyright 2026 Milos Mladenovic and contributors. Licensed under Apache-2.0. See LICENSE.

// Novel feature: calendar catch-up window. Fetches calendar events and
// activities over a window from the live API and classifies them into
// upcoming planned, completed, and missed (planned-but-no-activity). The
// aggregation across two endpoints is the value-add; neither endpoint provides
// it alone.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type sinceItem struct {
	Date     string `json:"date"`
	Name     string `json:"name,omitempty"`
	Type     string `json:"type,omitempty"`
	Category string `json:"category,omitempty"`
	ID       string `json:"id,omitempty"`
}

type sinceView struct {
	Window    string      `json:"window"`
	AthleteID string      `json:"athlete_id"`
	Today     string      `json:"today"`
	Upcoming  []sinceItem `json:"upcoming"`
	Completed []sinceItem `json:"completed"`
	Missed    []sinceItem `json:"missed"`
}

func newNovelSinceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "since [window]",
		Short: "Show planned, completed and missed sessions in a recent window.",
		Long: "Aggregate calendar events and activities across a window (default 7d)\n" +
			"into upcoming planned workouts, completed activities, and missed\n" +
			"planned workouts (no matching activity). Use for a quick 'what\n" +
			"happened / what's coming' digest. Do NOT use for a single date; use\n" +
			"'athlete events list'.",
		Example:     "  intervals-icu-pp-cli since 14d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch events and activities and classify the window")
				return nil
			}
			window := "7d"
			if len(args) > 0 {
				window = args[0]
				if !validWindow(window) {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid window %q: use a duration like 7d, 2w, or a number of days", window))
				}
			}
			days := parseWindowDays(window, 7)
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			id := athleteID(flags)
			today := localDate(0)

			// Events: past window .. future window (planned workouts/notes/races).
			evPath := replacePathParam("/api/v1/athlete/{id}/events", "id", id)
			evData, err := c.Get(cmd.Context(), evPath, map[string]string{
				"oldest": localDate(-days),
				"newest": localDate(days),
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var events []map[string]json.RawMessage
			_ = json.Unmarshal(evData, &events)

			// Activities: past window .. now (completed sessions).
			actPath := replacePathParam("/api/v1/athlete/{id}/activities", "id", id)
			actData, err := c.Get(cmd.Context(), actPath, map[string]string{
				"oldest": localDate(-days),
				"newest": today,
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var acts []map[string]json.RawMessage
			_ = json.Unmarshal(actData, &acts)

			// Index activity local-dates so missed-planned detection is O(1).
			activityDates := map[string]bool{}
			completed := make([]sinceItem, 0, len(acts))
			for _, a := range acts {
				d := dayOf(jsonStr(a, "start_date_local"))
				if d == "" {
					d = dayOf(jsonStr(a, "start_date"))
				}
				if d != "" {
					activityDates[d] = true
				}
				completed = append(completed, sinceItem{
					Date: d, Name: jsonStr(a, "name"), Type: jsonStr(a, "type"), ID: jsonStr(a, "id"),
				})
			}

			upcoming := make([]sinceItem, 0)
			missed := make([]sinceItem, 0)
			for _, e := range events {
				d := dayOf(jsonStr(e, "start_date_local"))
				if d == "" {
					continue
				}
				cat := jsonStr(e, "category")
				it := sinceItem{Date: d, Name: jsonStr(e, "name"), Category: cat, ID: jsonStr(e, "id")}
				if d >= today {
					upcoming = append(upcoming, it)
					continue
				}
				// Past planned workout with no activity that day = missed.
				if strings.EqualFold(cat, "WORKOUT") && !activityDates[d] {
					missed = append(missed, it)
				}
			}
			sort.SliceStable(upcoming, func(i, j int) bool { return upcoming[i].Date < upcoming[j].Date })
			sort.SliceStable(completed, func(i, j int) bool { return completed[i].Date > completed[j].Date })
			sort.SliceStable(missed, func(i, j int) bool { return missed[i].Date < missed[j].Date })

			view := sinceView{
				Window:    fmt.Sprintf("%dd", days),
				AthleteID: id,
				Today:     today,
				Upcoming:  upcoming,
				Completed: completed,
				Missed:    missed,
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return flags.printJSON(cmd, view)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Window: last/next %dd (today %s)\n", days, today)
			fmt.Fprintf(out, "\nUpcoming planned (%d):\n", len(upcoming))
			for _, it := range upcoming {
				fmt.Fprintf(out, "  %s  %-10s %s\n", it.Date, it.Category, it.Name)
			}
			fmt.Fprintf(out, "\nCompleted (%d):\n", len(completed))
			for _, it := range completed {
				fmt.Fprintf(out, "  %s  %-10s %s\n", it.Date, it.Type, it.Name)
			}
			fmt.Fprintf(out, "\nMissed planned workouts (%d):\n", len(missed))
			for _, it := range missed {
				fmt.Fprintf(out, "  %s  %s\n", it.Date, it.Name)
			}
			return nil
		},
	}
	return cmd
}

// dayOf returns the YYYY-MM-DD prefix of an ISO datetime string.
func dayOf(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}
