package cli

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type ebRepeatAttendeeRow struct {
	Email       string   `json:"email"`
	Name        string   `json:"name"`
	EventsCount int      `json:"events_count"`
	Tickets     int      `json:"tickets"`
	TotalSpend  float64  `json:"total_spend"`
	EventIDs    []string `json:"event_ids"`
}

func newRepeatAttendeesCmd(flags *rootFlags) *cobra.Command {
	var minEvents int
	var limit int

	cmd := &cobra.Command{
		Use:   "repeat-attendees",
		Short: "Find fans who attended two or more of your events",
		Example: strings.Trim(`
  eventbrite-pp-cli repeat-attendees
  eventbrite-pp-cli repeat-attendees --min 3 --limit 50
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
			results := make([]ebRepeatAttendeeRow, 0)
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
				name       string
				tickets    int
				spendMinor int64
				events     map[string]struct{}
			}
			byEmail := make(map[string]*agg)
			for _, a := range attendees {
				if a.Email == "" {
					continue
				}
				// A cancelled or refunded registration is not real attendance —
				// exclude it so events_count and spend reflect kept tickets only.
				if a.Cancelled || a.Refunded || ebOrderRefundedOrCancelled(a.Status) {
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
				g.tickets++
				g.spendMinor += a.GrossMinor
				if a.EventID != "" {
					g.events[a.EventID] = struct{}{}
				}
			}

			if minEvents < 1 {
				minEvents = 1
			}
			for email, g := range byEmail {
				if len(g.events) < minEvents {
					continue
				}
				eventIDs := make([]string, 0, len(g.events))
				for id := range g.events {
					eventIDs = append(eventIDs, id)
				}
				sort.Strings(eventIDs)
				results = append(results, ebRepeatAttendeeRow{
					Email:       email,
					Name:        g.name,
					EventsCount: len(g.events),
					Tickets:     g.tickets,
					TotalSpend:  ebRound2(ebMajor(g.spendMinor)),
					EventIDs:    eventIDs,
				})
			}

			sort.Slice(results, func(i, j int) bool {
				if results[i].EventsCount != results[j].EventsCount {
					return results[i].EventsCount > results[j].EventsCount
				}
				if results[i].TotalSpend != results[j].TotalSpend {
					return results[i].TotalSpend > results[j].TotalSpend
				}
				return results[i].Email < results[j].Email
			})
			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}

	cmd.Flags().IntVar(&minEvents, "min", 2, "Minimum distinct events attended")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows to return (0 = all)")
	return cmd
}
