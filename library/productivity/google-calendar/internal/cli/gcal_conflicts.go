// Hand-authored novel feature: cross-calendar conflict detection. Survives regen.
package cli

import (
	"sort"
	"time"

	"github.com/spf13/cobra"
)

type conflictEndpoint struct {
	ID       string `json:"id"`
	Summary  string `json:"summary"`
	Calendar string `json:"calendar"`
	Start    string `json:"start"`
	End      string `json:"end"`
}

type conflictPair struct {
	A              conflictEndpoint `json:"a"`
	B              conflictEndpoint `json:"b"`
	OverlapMinutes int              `json:"overlap_minutes"`
	CrossCalendar  bool             `json:"cross_calendar"`
}

func newConflictsCmd(flags *rootFlags) *cobra.Command {
	var calendarsCSV, window string
	cmd := &cobra.Command{
		Use:   "conflicts",
		Short: "List events that overlap in time across calendars",
		Long: "Detect double-bookings: events whose times overlap, within or across calendars, in a window.\n\n" +
			"Computed as a self-join over the local event store — no Google API endpoint returns cross-calendar\n" +
			"overlaps.",
		Example: "  google-calendar-pp-cli conflicts --calendars primary,team --window 'this week' --agent",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			start, end, err := parseWindow(window)
			if err != nil {
				return usageErr(err)
			}
			cals := resolveCalendars(calendarsCSV)
			events, _, err := gcalLoadEvents(cmd, flags, eventQuery{calendars: cals, timeMin: start, timeMax: end})
			if err != nil {
				return err
			}

			var busy []gcalEvent
			for _, ev := range events {
				if ev.Status == "cancelled" || ev.AllDay || ev.Transparency == "transparent" {
					continue
				}
				if ev.Start.IsZero() || ev.End.IsZero() || !ev.End.After(ev.Start) {
					continue
				}
				busy = append(busy, ev)
			}
			sort.Slice(busy, func(i, j int) bool { return busy[i].Start.Before(busy[j].Start) })

			var pairs []conflictPair
			for i := 0; i < len(busy); i++ {
				for j := i + 1; j < len(busy); j++ {
					a, b := busy[i], busy[j]
					if !b.Start.Before(a.End) {
						break // sorted by start; no later event can overlap a
					}
					overlap := minTime(a.End, b.End).Sub(maxTime(a.Start, b.Start))
					if overlap <= 0 {
						continue
					}
					pairs = append(pairs, conflictPair{
						A:              endpointOf(a),
						B:              endpointOf(b),
						OverlapMinutes: int(overlap.Minutes()),
						CrossCalendar:  a.CalendarID != b.CalendarID,
					})
				}
			}
			if pairs == nil {
				pairs = []conflictPair{}
			}
			return flags.printJSON(cmd, pairs)
		},
	}
	cmd.Flags().StringVar(&calendarsCSV, "calendars", "primary", "Comma-separated calendar IDs to check")
	cmd.Flags().StringVar(&window, "window", "this week", "Time window (today, this week, next 7 days, 2026-05-24, a..b)")
	return cmd
}

func endpointOf(ev gcalEvent) conflictEndpoint {
	return conflictEndpoint{
		ID:       ev.ID,
		Summary:  ev.Summary,
		Calendar: ev.CalendarID,
		Start:    ev.Start.Format(time.RFC3339),
		End:      ev.End.Format(time.RFC3339),
	}
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
