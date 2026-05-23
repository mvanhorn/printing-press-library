package cli

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type ebTopBuyersRow struct {
	Email      string  `json:"email"`
	Name       string  `json:"name"`
	Currency   string  `json:"currency"`
	Orders     int     `json:"orders"`
	Events     int     `json:"events"`
	TotalSpend float64 `json:"total_spend"`
}

func newTopBuyersCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var eventID string

	cmd := &cobra.Command{
		Use:   "top-buyers",
		Short: "Rank ticket buyers by total spend across all your events",
		Example: strings.Trim(`
  eventbrite-pp-cli top-buyers
  eventbrite-pp-cli top-buyers --limit 50
  eventbrite-pp-cli top-buyers --event 1234567890
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
			results := make([]ebTopBuyersRow, 0)
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

			type agg struct {
				name       string
				currency   string
				orders     int
				spendMinor int64
				events     map[string]struct{}
			}
			byEmail := make(map[string]*agg)
			for _, o := range orders {
				if eventID != "" && o.EventID != eventID {
					continue
				}
				if o.Email == "" {
					continue
				}
				// Exclude refunded/cancelled orders — the money was returned, so
				// it must not inflate a buyer's total spend.
				if ebOrderRefundedOrCancelled(o.Status) {
					continue
				}
				a := byEmail[o.Email]
				if a == nil {
					a = &agg{events: map[string]struct{}{}}
					byEmail[o.Email] = a
				}
				if a.name == "" && o.Name != "" {
					a.name = o.Name
				}
				if a.currency == "" && o.Currency != "" {
					a.currency = o.Currency
				}
				a.orders++
				a.spendMinor += o.GrossMinor
				if o.EventID != "" {
					a.events[o.EventID] = struct{}{}
				}
			}

			for email, a := range byEmail {
				results = append(results, ebTopBuyersRow{
					Email:      email,
					Name:       a.name,
					Currency:   a.currency,
					Orders:     a.orders,
					Events:     len(a.events),
					TotalSpend: ebRound2(ebMajor(a.spendMinor)),
				})
			}

			sort.Slice(results, func(i, j int) bool {
				if results[i].TotalSpend == results[j].TotalSpend {
					return results[i].Email < results[j].Email
				}
				return results[i].TotalSpend > results[j].TotalSpend
			})
			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum rows to return (0 = all)")
	cmd.Flags().StringVar(&eventID, "event", "", "Filter by event ID")
	return cmd
}
