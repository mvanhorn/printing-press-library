// Hand-authored novel feature: attendee/RSVP rollup. Survives regen.
package cli

import (
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type rsvpRow struct {
	ID          string `json:"id"`
	Summary     string `json:"summary"`
	Calendar    string `json:"calendar"`
	Start       string `json:"start,omitempty"`
	Accepted    int    `json:"accepted"`
	Declined    int    `json:"declined"`
	Tentative   int    `json:"tentative"`
	NeedsAction int    `json:"needs_action"`
	Total       int    `json:"total"`
	MyResponse  string `json:"my_response,omitempty"`
}

func newRSVPStatusCmd(flags *rootFlags) *cobra.Command {
	var calendarsCSV, window string
	var pendingOnly bool
	cmd := &cobra.Command{
		Use:   "rsvp-status",
		Short: "Summarize accepted/declined/tentative counts per event",
		Long: "Roll up attendee responses (accepted / declined / tentative / needs-action) per event over a window.\n\n" +
			"Counts stored attendee arrays from the local event store — a mechanical aggregation no single Google\n" +
			"API call provides.",
		Example: "  google-calendar-pp-cli rsvp-status --window 'next 14 days' --agent",
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

			var out []rsvpRow
			for _, ev := range events {
				if ev.Status == "cancelled" || len(ev.Attendees) == 0 {
					continue
				}
				row := rsvpRow{ID: ev.ID, Summary: ev.Summary, Calendar: ev.CalendarID, Total: len(ev.Attendees)}
				if !ev.Start.IsZero() {
					row.Start = ev.Start.Format(time.RFC3339)
				}
				for _, a := range ev.Attendees {
					switch strings.ToLower(a.ResponseStatus) {
					case "accepted":
						row.Accepted++
					case "declined":
						row.Declined++
					case "tentative":
						row.Tentative++
					default:
						row.NeedsAction++
					}
					if a.Self {
						row.MyResponse = a.ResponseStatus
					}
				}
				if pendingOnly && row.NeedsAction == 0 && row.Tentative == 0 {
					continue
				}
				out = append(out, row)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Start < out[j].Start })
			if out == nil {
				out = []rsvpRow{}
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&calendarsCSV, "calendars", "primary", "Comma-separated calendar IDs to include")
	cmd.Flags().StringVar(&window, "window", "next 14 days", "Time window (this week, next 14 days, a..b)")
	cmd.Flags().BoolVar(&pendingOnly, "pending-only", false, "Only show events with tentative or unanswered attendees")
	return cmd
}
