package cli

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type ebOrgRollupRow struct {
	OrganizationID string  `json:"organization_id"`
	Events         int     `json:"events"`
	Orders         int     `json:"orders"`
	TicketsSold    int64   `json:"tickets_sold"`
	Gross          float64 `json:"gross"`
	TopEventID     string  `json:"top_event_id"`
	TopEventName   string  `json:"top_event_name"`
}

func newOrgRollupCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "org-rollup",
		Short: "Single pane across every organization: events, orders, gross, top event",
		Example: strings.Trim(`
  eventbrite-pp-cli org-rollup
  eventbrite-pp-cli org-rollup --limit 10
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
			results := make([]ebOrgRollupRow, 0)
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
			orders, err := readEBOrders(cmd.Context(), db)
			if err != nil {
				return err
			}
			ticketClasses, err := readEBTicketClasses(cmd.Context(), db)
			if err != nil {
				return err
			}

			eventByID := make(map[string]ebEvent)
			eventsPerOrg := make(map[string]int)
			for _, e := range events {
				eventByID[e.ID] = e
				orgID := e.OrgID
				if orgID == "" {
					orgID = "(unknown)"
				}
				eventsPerOrg[orgID]++
			}

			type orgAgg struct {
				orders      int
				ticketsSold int64
				grossMinor  int64
				eventGross  map[string]int64
			}
			orgs := make(map[string]*orgAgg)
			for orgID := range eventsPerOrg {
				orgs[orgID] = &orgAgg{eventGross: make(map[string]int64)}
			}

			for _, tc := range ticketClasses {
				e, ok := eventByID[tc.EventID]
				if !ok {
					continue
				}
				orgID := e.OrgID
				if orgID == "" {
					orgID = "(unknown)"
				}
				orgs[orgID].ticketsSold += tc.QtySold
			}
			for _, o := range orders {
				e, ok := eventByID[o.EventID]
				if !ok {
					continue
				}
				// Refunded/cancelled orders keep a positive gross value; exclude
				// them so per-org gross isn't overstated.
				if ebOrderRefundedOrCancelled(o.Status) {
					continue
				}
				orgID := e.OrgID
				if orgID == "" {
					orgID = "(unknown)"
				}
				a := orgs[orgID]
				a.orders++
				a.grossMinor += o.GrossMinor
				a.eventGross[o.EventID] += o.GrossMinor
			}

			if _, hasEmpty := orgs[""]; hasEmpty && len(orgs) > 1 {
				delete(orgs, "")
			}

			for orgID, a := range orgs {
				topEventID := ""
				var topGross int64
				for eid, g := range a.eventGross {
					if g > topGross || (g == topGross && (topEventID == "" || eid < topEventID)) {
						topEventID = eid
						topGross = g
					}
				}
				topEventName := ""
				if e, ok := eventByID[topEventID]; ok {
					topEventName = e.Name
				}
				results = append(results, ebOrgRollupRow{
					OrganizationID: orgID,
					Events:         eventsPerOrg[orgID],
					Orders:         a.orders,
					TicketsSold:    a.ticketsSold,
					Gross:          ebRound2(ebMajor(a.grossMinor)),
					TopEventID:     topEventID,
					TopEventName:   topEventName,
				})
			}

			sort.Slice(results, func(i, j int) bool {
				if results[i].Gross == results[j].Gross {
					return results[i].OrganizationID < results[j].OrganizationID
				}
				return results[i].Gross > results[j].Gross
			})
			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows to return (0 = all)")
	return cmd
}
