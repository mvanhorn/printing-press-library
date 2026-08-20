package cli

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type ebDiscountPerformanceRow struct {
	Code           string  `json:"code"`
	EventID        string  `json:"event_id"`
	Type           string  `json:"type"`
	PercentOff     float64 `json:"percent_off"`
	AmountOff      float64 `json:"amount_off"`
	Redemptions    int64   `json:"redemptions"`
	Available      int64   `json:"available"`
	RedemptionRate float64 `json:"redemption_rate"`
}

func newDiscountPerformanceCmd(flags *rootFlags) *cobra.Command {
	var eventID string
	var limit int

	cmd := &cobra.Command{
		Use:   "discount-performance",
		Short: "Per discount code: redemptions, type, and redemption rate",
		Example: strings.Trim(`
  eventbrite-pp-cli discount-performance
  eventbrite-pp-cli discount-performance --event 1234567890 --limit 20
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
			results := make([]ebDiscountPerformanceRow, 0)
			if s == nil {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			defer s.Close()
			if !hintIfUnsynced(cmd, s, "") {
				hintIfStale(cmd, s, "", flags.maxAge)
			}

			db := s.DB()
			discounts, err := readEBDiscounts(cmd.Context(), db)
			if err != nil {
				return err
			}

			for _, d := range discounts {
				if eventID != "" && d.EventID != eventID {
					continue
				}
				// Eventbrite's quantity_available is the discount's total cap
				// (max times the code can be used), not the remaining count, so
				// the redemption rate is sold / cap — not sold / (sold + cap).
				rate := 0.0
				if d.QtyAvail > 0 {
					rate = ebRound2(float64(d.QtySold) / float64(d.QtyAvail))
				}
				results = append(results, ebDiscountPerformanceRow{
					Code:           d.Code,
					EventID:        d.EventID,
					Type:           d.Type,
					PercentOff:     d.PercentOff,
					AmountOff:      ebRound2(ebMajor(d.AmountOff)),
					Redemptions:    d.QtySold,
					Available:      d.QtyAvail,
					RedemptionRate: rate,
				})
			}

			sort.Slice(results, func(i, j int) bool {
				if results[i].Redemptions == results[j].Redemptions {
					return results[i].Code < results[j].Code
				}
				return results[i].Redemptions > results[j].Redemptions
			})
			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}

	cmd.Flags().StringVar(&eventID, "event", "", "Filter by event ID")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows to return (0 = all)")
	return cmd
}
