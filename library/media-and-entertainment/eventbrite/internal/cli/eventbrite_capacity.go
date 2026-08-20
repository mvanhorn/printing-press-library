package cli

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type ebCapacityRow struct {
	EventID   string  `json:"event_id"`
	EventName string  `json:"event_name"`
	Sold      int64   `json:"sold"`
	Capacity  int64   `json:"capacity"`
	Remaining int64   `json:"remaining"`
	PctSold   float64 `json:"pct_sold"`
}

func newCapacityCmd(flags *rootFlags) *cobra.Command {
	var orgID string
	var limit int

	cmd := &cobra.Command{
		Use:   "capacity",
		Short: "Sold vs total capacity and percent remaining across all live events",
		Example: strings.Trim(`
  eventbrite-pp-cli capacity
  eventbrite-pp-cli capacity --org 1234567890 --limit 20
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
			results := make([]ebCapacityRow, 0)
			if s == nil {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			defer s.Close()
			if !hintIfUnsynced(cmd, s, "") {
				hintIfStale(cmd, s, "", flags.maxAge)
			}

			db := s.DB()
			events, err := readEBEvents(cmd.Context(), db)
			if err != nil {
				return err
			}
			ticketClasses, err := readEBTicketClasses(cmd.Context(), db)
			if err != nil {
				return err
			}

			eventByID := make(map[string]ebEvent)
			for _, e := range events {
				eventByID[e.ID] = e
			}

			type agg struct{ sold, total int64 }
			byEvent := make(map[string]*agg)
			for _, tc := range ticketClasses {
				a := byEvent[tc.EventID]
				if a == nil {
					a = &agg{}
					byEvent[tc.EventID] = a
				}
				a.sold += tc.QtySold
				a.total += tc.QtyTotal
			}

			for eventID, a := range byEvent {
				e := eventByID[eventID]
				// "all live events" command: exclude ended/completed/canceled
				// events from the cross-event capacity headroom view.
				if !ebIsLiveEvent(e.Status) {
					continue
				}
				if orgID != "" && e.OrgID != orgID {
					continue
				}
				remaining := a.total - a.sold
				pct := 0.0
				if a.total > 0 {
					pct = ebRound2((float64(a.sold) / float64(a.total)) * 100.0)
				}
				results = append(results, ebCapacityRow{
					EventID:   eventID,
					EventName: e.Name,
					Sold:      a.sold,
					Capacity:  a.total,
					Remaining: remaining,
					PctSold:   pct,
				})
			}

			sort.Slice(results, func(i, j int) bool {
				if results[i].PctSold == results[j].PctSold {
					return results[i].EventID < results[j].EventID
				}
				return results[i].PctSold > results[j].PctSold
			})
			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}

	cmd.Flags().StringVar(&orgID, "org", "", "Filter by organization ID")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows to return (0 = all)")
	return cmd
}
