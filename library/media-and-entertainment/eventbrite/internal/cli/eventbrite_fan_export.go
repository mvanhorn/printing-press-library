package cli

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type ebFanExportRow struct {
	Email          string `json:"email"`
	Name           string `json:"name"`
	TicketClass    string `json:"ticket_class"`
	EventsAttended int    `json:"events_attended"`
	CheckedIn      bool   `json:"checked_in"`
}

func newFanExportCmd(flags *rootFlags) *cobra.Command {
	var eventID string
	var checkedInOnly bool

	cmd := &cobra.Command{
		Use:   "fan-export",
		Short: "Export deduped attendee contacts across events, opt-in flagged where present",
		Example: strings.Trim(`
  eventbrite-pp-cli fan-export
  eventbrite-pp-cli fan-export --event 1234567890
  eventbrite-pp-cli fan-export --checked-in
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			s, err := openEBStore(cmd.Context())
			if err != nil {
				return err
			}
			results := make([]ebFanExportRow, 0)
			if s == nil {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			defer s.Close()
			if !hintIfUnsynced(cmd, s, "") {
				hintIfStale(cmd, s, "", flags.maxAge)
			}

			db := s.DB()
			attendees, err := readEBAttendees(cmd.Context(), db)
			if err != nil {
				return err
			}

			type agg struct {
				name        string
				ticketClass string
				checkedIn   bool
				events      map[string]struct{}
			}
			byEmail := make(map[string]*agg)
			for _, a := range attendees {
				if eventID != "" && a.EventID != eventID {
					continue
				}
				// Skip cancelled/refunded registrations so events_attended
				// reflects kept tickets and contacts who opted out of every
				// event don't pollute the export.
				if a.Cancelled || a.Refunded || ebOrderRefundedOrCancelled(a.Status) {
					continue
				}
				if checkedInOnly && !a.CheckedIn {
					continue
				}
				if a.Email == "" {
					continue
				}
				g := byEmail[a.Email]
				if g == nil {
					g = &agg{events: map[string]struct{}{}}
					byEmail[a.Email] = g
				}
				if g.name == "" && a.Name != "" {
					g.name = a.Name
				}
				if a.TicketClass != "" {
					g.ticketClass = a.TicketClass
				}
				if a.CheckedIn {
					g.checkedIn = true
				}
				if a.EventID != "" {
					g.events[a.EventID] = struct{}{}
				}
			}

			for email, g := range byEmail {
				results = append(results, ebFanExportRow{
					Email:          email,
					Name:           g.name,
					TicketClass:    g.ticketClass,
					EventsAttended: len(g.events),
					CheckedIn:      g.checkedIn,
				})
			}

			sort.Slice(results, func(i, j int) bool { return results[i].Email < results[j].Email })
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}

	cmd.Flags().StringVar(&eventID, "event", "", "Filter by event ID")
	cmd.Flags().BoolVar(&checkedInOnly, "checked-in", false, "Only include checked-in attendees")
	return cmd
}
