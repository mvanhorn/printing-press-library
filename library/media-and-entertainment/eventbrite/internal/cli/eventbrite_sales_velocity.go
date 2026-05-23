package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type ebSalesVelocityRow struct {
	EventID              string  `json:"event_id"`
	EventName            string  `json:"event_name"`
	Status               string  `json:"status"`
	TicketsSold          int64   `json:"tickets_sold"`
	Capacity             int64   `json:"capacity"`
	Remaining            int64   `json:"remaining"`
	Gross                float64 `json:"gross"`
	OrdersInWindow       int     `json:"orders_in_window"`
	TicketsPerDay        float64 `json:"tickets_per_day"`
	ProjectedSelloutDays float64 `json:"projected_sellout_days"`
}

func parseEBSince(s string) (time.Time, bool) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return time.Time{}, false
	}
	now := time.Now()
	if len(s) >= 2 {
		n, err := strconv.Atoi(s[:len(s)-1])
		if err == nil && n >= 0 {
			switch s[len(s)-1] {
			case 'd':
				return now.Add(-time.Duration(n) * 24 * time.Hour), true
			case 'h':
				return now.Add(-time.Duration(n) * time.Hour), true
			case 'w':
				return now.Add(-time.Duration(n) * 7 * 24 * time.Hour), true
			}
		}
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return time.Time{}, false
	}
	return now.Add(-d), true
}

func newSalesVelocityCmd(flags *rootFlags) *cobra.Command {
	var since string
	var limit int

	cmd := &cobra.Command{
		Use:   "sales-velocity",
		Short: "Rank live events by tickets sold per day with a projected sell-out date",
		Example: strings.Trim(`
  eventbrite-pp-cli sales-velocity --since 30d --limit 10
  eventbrite-pp-cli sales-velocity --since 24h
  eventbrite-pp-cli sales-velocity --since 2w
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
			results := make([]ebSalesVelocityRow, 0)
			if s == nil {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			defer s.Close()

			if !hintIfUnsynced(cmd, s, "") {
				hintIfStale(cmd, s, "", flags.maxAge)
			}

			cutoff, hasCutoff := parseEBSince(since)
			if strings.TrimSpace(since) != "" && !hasCutoff {
				return fmt.Errorf("invalid --since %q: use values like 30d, 24h, 2w, 15m", since)
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

			ticketsSoldByEvent := make(map[string]int64)
			capacityByEvent := make(map[string]int64)
			for _, tc := range ticketClasses {
				ticketsSoldByEvent[tc.EventID] += tc.QtySold
				capacityByEvent[tc.EventID] += tc.QtyTotal
			}

			type orderAgg struct {
				grossMinor int64
				count      int
				first      time.Time
				last       time.Time
				hasFirst   bool
				hasLast    bool
			}
			aggByEvent := make(map[string]*orderAgg)
			for _, o := range orders {
				if o.EventID == "" {
					continue
				}
				// Exclude refunded/cancelled orders so gross and order counts
				// reflect kept revenue, consistent with org-rollup/top-buyers.
				if ebOrderRefundedOrCancelled(o.Status) {
					continue
				}
				if hasCutoff {
					ot, err := time.Parse(time.RFC3339, o.Created)
					if err == nil && ot.Before(cutoff) {
						continue
					}
				}
				a := aggByEvent[o.EventID]
				if a == nil {
					a = &orderAgg{}
					aggByEvent[o.EventID] = a
				}
				a.grossMinor += o.GrossMinor
				a.count++
				if t, err := time.Parse(time.RFC3339, o.Created); err == nil {
					if !a.hasFirst || t.Before(a.first) {
						a.first = t
						a.hasFirst = true
					}
					if !a.hasLast || t.After(a.last) {
						a.last = t
						a.hasLast = true
					}
				}
			}

			now := time.Now()
			for _, e := range events {
				// "live events" command: skip ended/completed/canceled/draft
				// events whose sell-out projection would be meaningless.
				if !ebIsLiveEvent(e.Status) {
					continue
				}
				sold := ticketsSoldByEvent[e.ID]
				capTotal := capacityByEvent[e.ID]
				remaining := capTotal - sold
				a := aggByEvent[e.ID]
				var grossMinor int64
				var orderCount int
				if a != nil {
					grossMinor = a.grossMinor
					orderCount = a.count
				}
				// tickets_per_day is an all-time average: `sold` comes from
				// ticket-class lifetime totals, which cannot be windowed, so the
				// denominator must be the event's full selling lifetime
				// (created -> now), not the first order inside --since (which
				// would inflate the rate). --since scopes orders_in_window /
				// gross only.
				daysActive := 1.0
				if e.Created != "" {
					if ct, err := time.Parse(time.RFC3339, e.Created); err == nil {
						if d := now.Sub(ct).Hours() / 24.0; d > 1 {
							daysActive = d
						}
					}
				} else if a != nil && a.hasFirst {
					if d := now.Sub(a.first).Hours() / 24.0; d > 1 {
						daysActive = d
					}
				}
				tpd := 0.0
				if sold > 0 {
					tpd = float64(sold) / daysActive
				}
				projected := 0.0
				if tpd > 0 && remaining > 0 {
					projected = float64(remaining) / tpd
				}
				results = append(results, ebSalesVelocityRow{
					EventID:              e.ID,
					EventName:            e.Name,
					Status:               e.Status,
					TicketsSold:          sold,
					Capacity:             capTotal,
					Remaining:            remaining,
					Gross:                ebRound2(ebMajor(grossMinor)),
					OrdersInWindow:       orderCount,
					TicketsPerDay:        ebRound2(tpd),
					ProjectedSelloutDays: ebRound2(projected),
				})
			}

			sort.Slice(results, func(i, j int) bool {
				if results[i].TicketsPerDay == results[j].TicketsPerDay {
					return results[i].EventID < results[j].EventID
				}
				return results[i].TicketsPerDay > results[j].TicketsPerDay
			})
			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}

	cmd.Flags().StringVar(&since, "since", "", "Optional time window (e.g. 30d, 24h, 2w, 15m)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows to return (0 = all)")

	return cmd
}
