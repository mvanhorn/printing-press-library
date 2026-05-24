// Hand-authored absorbed feature: agenda (gcalcli-style time-range listing),
// local-store-backed and offline. Survives regen.
package cli

import (
	"sort"
	"time"

	"github.com/spf13/cobra"
)

type agendaRow struct {
	Start    string `json:"start"`
	End      string `json:"end"`
	Summary  string `json:"summary"`
	Calendar string `json:"calendar"`
	Status   string `json:"status"`
	AllDay   bool   `json:"all_day"`
}

func newAgendaCmd(flags *rootFlags) *cobra.Command {
	var calendarsCSV, window string
	cmd := &cobra.Command{
		Use:   "agenda",
		Short: "List events in a time range across calendars (offline-fast)",
		Long: "Show your agenda for a window across one or more calendars, sorted by start time.\n\n" +
			"Backed by the local event store (with a live fetch + cache fallback), so a normal `agenda` is\n" +
			"instant and works offline — the gcalcli `agenda` you know, minus the round-trip.",
		Example: "  google-calendar-pp-cli agenda --window today\n  google-calendar-pp-cli agenda --calendars primary,work --window 'this week' --agent",
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
			var out []agendaRow
			for _, ev := range events {
				if ev.Status == "cancelled" {
					continue
				}
				row := agendaRow{Summary: ev.Summary, Calendar: ev.CalendarID, Status: ev.Status, AllDay: ev.AllDay}
				if !ev.Start.IsZero() {
					row.Start = ev.Start.Format(time.RFC3339)
				}
				if !ev.End.IsZero() {
					row.End = ev.End.Format(time.RFC3339)
				}
				out = append(out, row)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Start < out[j].Start })
			if out == nil {
				out = []agendaRow{}
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&calendarsCSV, "calendars", "primary", "Comma-separated calendar IDs to include")
	cmd.Flags().StringVar(&window, "window", "today", "Time window (today, this week, next 7 days, 2026-05-24, a..b)")
	return cmd
}
