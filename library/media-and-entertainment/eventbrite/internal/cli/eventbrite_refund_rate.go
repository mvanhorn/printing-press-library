package cli

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type ebRefundRateRow struct {
	EventID         string  `json:"event_id"`
	EventName       string  `json:"event_name"`
	Orders          int     `json:"orders"`
	Refunded        int     `json:"refunded"`
	Cancelled       int     `json:"cancelled"`
	RefundedRevenue float64 `json:"refunded_revenue"`
	RefundRate      float64 `json:"refund_rate"`
}

func newRefundRateCmd(flags *rootFlags) *cobra.Command {
	var orgID string
	var limit int

	cmd := &cobra.Command{
		Use:   "refund-rate",
		Short: "Refund and cancellation counts, refunded revenue, and rate per event",
		Example: strings.Trim(`
  eventbrite-pp-cli refund-rate
  eventbrite-pp-cli refund-rate --org 1234567890 --limit 25
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
			results := make([]ebRefundRateRow, 0)
			if s == nil {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			defer s.Close()
			if !hintIfUnsynced(cmd, s, "") {
				hintIfStale(cmd, s, "", flags.maxAge)
			}

			db := s.DB()
			orders, err := readEBOrders(cmd.Context(), db)
			if err != nil {
				return err
			}
			events, err := readEBEvents(cmd.Context(), db)
			if err != nil {
				return err
			}

			eventByID := make(map[string]ebEvent)
			for _, e := range events {
				eventByID[e.ID] = e
			}

			type agg struct {
				orders, refunded, cancelled int
				refundedRevenueMinor        int64
			}
			byEvent := make(map[string]*agg)
			for _, o := range orders {
				a := byEvent[o.EventID]
				if a == nil {
					a = &agg{}
					byEvent[o.EventID] = a
				}
				a.orders++
				status := strings.ToLower(strings.TrimSpace(o.Status))
				if strings.Contains(status, "refund") {
					a.refunded++
					a.refundedRevenueMinor += o.GrossMinor
				}
				if status == "cancelled" || status == "canceled" || status == "deleted" {
					a.cancelled++
				}
			}

			for eventID, a := range byEvent {
				e := eventByID[eventID]
				if orgID != "" && e.OrgID != orgID {
					continue
				}
				rate := 0.0
				if a.orders > 0 {
					rate = ebRound2(float64(a.refunded+a.cancelled) / float64(a.orders))
				}
				results = append(results, ebRefundRateRow{
					EventID:         eventID,
					EventName:       e.Name,
					Orders:          a.orders,
					Refunded:        a.refunded,
					Cancelled:       a.cancelled,
					RefundedRevenue: ebRound2(ebMajor(a.refundedRevenueMinor)),
					RefundRate:      rate,
				})
			}

			sort.Slice(results, func(i, j int) bool {
				if results[i].RefundRate == results[j].RefundRate {
					return results[i].EventID < results[j].EventID
				}
				return results[i].RefundRate > results[j].RefundRate
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
