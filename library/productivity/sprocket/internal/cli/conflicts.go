// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// defaultEventDuration is assumed when an event has no parseable end time, so
// overlap math still works for events that only carry a start.
const defaultEventDuration = 90 * time.Minute

func newNovelConflictsCmd(flags *rootFlags) *cobra.Command {
	var days int
	var gapMinutes int
	cmd := &cobra.Command{
		Use:   "conflicts",
		Short: "Flags time overlaps and impossible tight time-plus-location gaps between events across all your players.",
		Long: "Find scheduling conflicts across all your players' events: time overlaps (double-bookings) and " +
			"tight gaps between events at different locations you could not realistically travel between.\n\n" +
			"Use this command to find double-bookings and impossible back-to-backs across players. Do NOT use " +
			"it to list the schedule; use 'agenda' or 'week'.",
		Example:     "  sprocket-pp-cli conflicts --days 14\n  sprocket-pp-cli conflicts --days 30 --gap 45",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := rejectLocalDataSource(flags); err != nil {
				return err
			}
			if days < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--days must be at least 1"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			now := time.Now()
			events, err := fetchCalendar(ctx, c, now, now.AddDate(0, 0, days))
			if err != nil {
				return err
			}
			conflicts := detectConflicts(events, time.Duration(gapMinutes)*time.Minute)
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				if len(conflicts) == 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "No conflicts in the next %d days.\n", days)
					return nil
				}
				for _, p := range conflicts {
					label := "OVERLAP"
					if p.Kind == "tight-gap" {
						label = fmt.Sprintf("TIGHT GAP (%d min)", p.GapMinutes)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "%s\n  - %s\n  - %s\n", label, conflictLine(p.A), conflictLine(p.B))
				}
				return nil
			}
			return flags.printJSON(cmd, conflictViews(conflicts))
		},
	}
	cmd.Flags().IntVar(&days, "days", 14, "number of days ahead to check")
	cmd.Flags().IntVar(&gapMinutes, "gap", 60, "minutes below which a gap between events at different locations counts as a conflict")
	return cmd
}

type conflictPair struct {
	A, B       calEvent
	Kind       string // "overlap" or "tight-gap"
	GapMinutes int
}

// effectiveInterval returns an event's [start, end), defaulting the end when
// the event has no parseable end time.
func effectiveInterval(e calEvent) (time.Time, time.Time) {
	start := e.Start
	end := e.End
	if !end.After(start) {
		end = start.Add(defaultEventDuration)
	}
	return start, end
}

// detectConflicts finds overlapping events and tight gaps between events at
// different locations. Pure and unit-tested. Cancelled and undated events are
// ignored. gap <= 0 disables tight-gap detection.
func detectConflicts(events []calEvent, gap time.Duration) []conflictPair {
	var dated []calEvent
	for _, e := range events {
		if e.HasStart && !e.Cancelled {
			dated = append(dated, e)
		}
	}
	sortEventsByStart(dated)
	var out []conflictPair
	for i := 0; i < len(dated); i++ {
		_, aEnd := effectiveInterval(dated[i])
		for j := i + 1; j < len(dated); j++ {
			bStart, _ := effectiveInterval(dated[j])
			if bStart.Before(aEnd) {
				// aStart <= bStart < aEnd, so the intervals overlap.
				out = append(out, conflictPair{A: dated[i], B: dated[j], Kind: "overlap"})
				continue
			}
			// bStart >= aEnd: no overlap. Test for a tight travel gap.
			delta := bStart.Sub(aEnd)
			if gap <= 0 || delta >= gap {
				// dated is sorted by start, so every later j is even further
				// from aEnd: none overlap and none are within the gap window.
				break
			}
			if delta >= 0 && differentLocations(dated[i], dated[j]) {
				out = append(out, conflictPair{A: dated[i], B: dated[j], Kind: "tight-gap", GapMinutes: int(delta.Minutes())})
			}
			// Keep scanning: a later event at a different location may also fall
			// inside the gap window (e.g. a same-location neighbor masks a
			// cross-location one just beyond it).
		}
	}
	return out
}

func differentLocations(a, b calEvent) bool {
	if a.LocationID != 0 && b.LocationID != 0 {
		return a.LocationID != b.LocationID
	}
	// Fall back to name comparison when IDs are missing.
	if a.Location != "" && b.Location != "" {
		return a.Location != b.Location
	}
	// Unknown locations: assume different so a tight gap is surfaced as a caution.
	return true
}

func conflictLine(e calEvent) string {
	when := "TBD"
	if e.HasStart {
		when = e.Start.Format("Mon Jan 2, 3:04 PM")
	}
	title := e.Title
	if e.Opponent != "" {
		title = fmt.Sprintf("%s vs %s", title, e.Opponent)
	}
	return fmt.Sprintf("%s — %s", when, title)
}

type conflictEventView struct {
	Title    string `json:"title"`
	Start    string `json:"start"`
	Opponent string `json:"opponent,omitempty"`
	Location string `json:"location,omitempty"`
}

type conflictView struct {
	Kind       string            `json:"kind"`
	GapMinutes int               `json:"gapMinutes,omitempty"`
	A          conflictEventView `json:"a"`
	B          conflictEventView `json:"b"`
}

func toConflictEventView(e calEvent) conflictEventView {
	v := conflictEventView{Title: e.Title, Opponent: e.Opponent, Location: e.Location}
	if e.HasStart {
		v.Start = e.Start.Format(time.RFC3339)
	}
	return v
}

func conflictViews(pairs []conflictPair) []conflictView {
	out := make([]conflictView, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, conflictView{
			Kind:       p.Kind,
			GapMinutes: p.GapMinutes,
			A:          toConflictEventView(p.A),
			B:          toConflictEventView(p.B),
		})
	}
	return out
}
