package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newChurnCmd implements `churn` — customers who haven't booked in N days.
// Uses the `customer-activity-export` report for the visit-history view, then
// filters to absence threshold. Optional `--has-voucher` overlay joins
// against the vouchers endpoint.
func newChurnCmd(flags *rootFlags) *cobra.Command {
	var notBookedSince string
	var hasVoucher bool
	cmd := &cobra.Command{
		Use:         "churn",
		Short:       "Customers who haven't booked in N days; --has-voucher overlays unused credits",
		Example:     "  goose-pp-cli churn --not-booked-since 60d --has-voucher --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}
			days, err := parseDayWindow(notBookedSince)
			if err != nil {
				return err
			}
			now := time.Now()
			cutoff := now.AddDate(0, 0, -days).Format("2006-01-02")

			// 1. customer-activity export gives us per-customer last-visit info.
			//    Range parameters span the past year so we catch enough activity.
			startOfYear := now.AddDate(-1, 0, 0).Format("2006-01-02")
			params := map[string]string{
				"date":      now.Format("2006-01-02"),
				"startDate": startOfYear,
				"endDate":   now.Format("2006-01-02"),
			}
			raw, err := c.Get("/reports/customer-activity-export", params)
			if err != nil {
				return fmt.Errorf("fetching customer-activity export: %w", err)
			}
			rows := decodeReportRows(raw)
			out := []map[string]any{}
			for _, r := range rows {
				lastBooked := pickLastBookedField(r)
				if lastBooked == "" {
					continue
				}
				if lastBooked > cutoff {
					continue
				}
				r["lastBookedAt"] = lastBooked
				out = append(out, r)
			}

			// 2. --has-voucher: intersect with active CASH vouchers.
			if hasVoucher && len(out) > 0 {
				vparams := map[string]string{
					"status":              "ACTIVE",
					"type":                "CASH",
					"isUsed":              "false",
					"expired":             "false",
					"atLeastOneAvailable": "true",
					"includeShared":       "false",
					"limit":               "500",
				}
				vouchers, verr := c.Get("/vouchers", vparams)
				if verr == nil {
					withVoucher := collectVoucherCustomerIds(vouchers)
					filtered := out[:0]
					for _, r := range out {
						id := strOrEmpty(pickCustomerID(r))
						if id != "" && withVoucher[id] {
							r["hasUnusedVoucher"] = true
							filtered = append(filtered, r)
						}
					}
					out = filtered
				}
			}

			if flags.asJSON || flags.compact || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Customers not booked since %s", cutoff)
			if hasVoucher {
				fmt.Fprint(cmd.OutOrStdout(), " (with unused voucher)")
			}
			fmt.Fprintf(cmd.OutOrStdout(), ": %d\n", len(out))
			for _, r := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s — last booked %s\n", customerLabel(r), strOrEmpty(r["lastBookedAt"]))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&notBookedSince, "not-booked-since", "60d", "Absence threshold (e.g. 30d, 60d, 90d)")
	cmd.Flags().BoolVar(&hasVoucher, "has-voucher", false, "Filter to customers with an unused active voucher")
	return cmd
}

// pickLastBookedField tolerates several plausible field names in the export.
func pickLastBookedField(r map[string]any) string {
	for _, k := range []string{"lastBookedAt", "lastVisitDate", "lastBookingDate", "last_visit", "last_booking", "lastVisit", "lastBooking"} {
		if s, ok := r[k].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

func pickCustomerID(r map[string]any) any {
	for _, k := range []string{"customerId", "locationUserProfileId", "id"} {
		if v, ok := r[k]; ok {
			return v
		}
	}
	return nil
}

func customerLabel(r map[string]any) string {
	if s := strOrEmpty(r["customerName"]); s != "" {
		return s
	}
	first := strOrEmpty(r["firstName"])
	last := strOrEmpty(r["lastName"])
	if first != "" || last != "" {
		return strings.TrimSpace(first + " " + last)
	}
	return strOrEmpty(r["email"])
}

func collectVoucherCustomerIds(raw json.RawMessage) map[string]bool {
	out := map[string]bool{}
	var env struct {
		Results []struct {
			LocationUserProfile struct {
				ID string `json:"id"`
			} `json:"locationUserProfile"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &env); err == nil {
		for _, r := range env.Results {
			if r.LocationUserProfile.ID != "" {
				out[r.LocationUserProfile.ID] = true
			}
		}
	}
	return out
}
