package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type ebRosterRow struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	TicketClass string `json:"ticket_class"`
	Status      string `json:"status"`
	CheckedIn   bool   `json:"checked_in"`
}

func newRosterCmd(flags *rootFlags) *cobra.Command {
	var eventID string
	var checkedInOnly bool
	var notCheckedInOnly bool

	cmd := &cobra.Command{
		Use:   "roster [event_id]",
		Short: "Offline attendee roster for one event with check-in status",
		Example: strings.Trim(`
  eventbrite-pp-cli roster 1234567890
  eventbrite-pp-cli roster --event 1234567890 --checked-in
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			target := ""
			if len(args) > 0 {
				target = strings.TrimSpace(args[0])
			}
			if target == "" {
				target = strings.TrimSpace(eventID)
			}
			if target == "" {
				return fmt.Errorf("provide an event id: roster <event_id> or --event <event_id>")
			}

			s, err := openEBStore(cmd.Context())
			if err != nil {
				return err
			}
			results := make([]ebRosterRow, 0)
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

			for _, a := range attendees {
				if a.EventID != target {
					continue
				}
				// Door roster lists expected arrivals only — a cancelled or
				// refunded registrant must not appear as someone still due to
				// check in, or staff could admit them by mistake.
				if a.Cancelled || a.Refunded || ebOrderRefundedOrCancelled(a.Status) {
					continue
				}
				if checkedInOnly && !a.CheckedIn {
					continue
				}
				if notCheckedInOnly && a.CheckedIn {
					continue
				}
				results = append(results, ebRosterRow{
					Name:        a.Name,
					Email:       a.Email,
					TicketClass: a.TicketClass,
					Status:      a.Status,
					CheckedIn:   a.CheckedIn,
				})
			}

			sort.Slice(results, func(i, j int) bool {
				if results[i].CheckedIn != results[j].CheckedIn {
					return !results[i].CheckedIn
				}
				return results[i].Name < results[j].Name
			})
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}

	cmd.Flags().StringVar(&eventID, "event", "", "Event ID (alternative to positional event_id)")
	cmd.Flags().BoolVar(&checkedInOnly, "checked-in", false, "Show only checked-in attendees")
	cmd.Flags().BoolVar(&notCheckedInOnly, "not-checked-in", false, "Show only attendees not checked in")
	return cmd
}
