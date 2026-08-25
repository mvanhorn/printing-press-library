// Copyright 2026 matthew.martin and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type reconcileCloseSummary struct {
	LocationID               string `json:"location_id"`
	LocationName             string `json:"location_name,omitempty"`
	Currency                 string `json:"currency,omitempty"`
	OrderCount               int    `json:"completed_order_count"`
	OrderSales               int64  `json:"order_sales"`
	PaymentCount             int    `json:"completed_payment_count"`
	GrossPayments            int64  `json:"gross_payments"`
	RefundCount              int    `json:"refund_count"`
	Refunds                  int64  `json:"refunds"`
	DisputeCount             int    `json:"dispute_count"`
	Disputes                 int64  `json:"dispute_amount"`
	DisputeLosses            int64  `json:"dispute_loss_amount"`
	PayoutCount              int    `json:"completed_payout_count"`
	Payouts                  int64  `json:"payouts"`
	NetCollected             int64  `json:"net_collected"`
	OrderSalesMinusPayments  int64  `json:"order_sales_minus_payments"`
	NetCollectedMinusPayouts int64  `json:"net_collected_minus_payouts"`
}

func newNovelReconcileCloseCmd(flags *rootFlags) *cobra.Command {
	var flagSince string

	cmd := &cobra.Command{
		Use:         "close",
		Short:       "Explain the difference between sales, collected payments, refunds, disputes, and payouts for each location.",
		Example:     "  square-pp-cli reconcile close --since 24h --agent",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "reconcile close")
			}
			cutoff, err := parseSinceDuration(flagSince)
			if err != nil {
				return fmt.Errorf("invalid value %q for --since: %s", flagSince, err)
			}
			resources := []string{"locations", "orders", "payments", "refunds", "disputes", "payouts"}
			db, err := openNovelLocalStore(cmd, flags, resources)
			if err != nil {
				return err
			}
			defer db.Close()
			records, err := loadLocalSquareRecords(cmd.Context(), db, resources)
			if err != nil {
				return err
			}

			locations := map[string]string{}
			paymentLocations := map[string]string{}
			orderLocations := map[string]string{}
			for _, record := range records {
				if record.ResourceType == "locations" {
					locations[record.ID] = firstString(record.Data, []string{"name"}, []string{"business_name"})
				} else if record.ResourceType == "orders" {
					orderLocations[record.ID] = stringValue(record.Data, "location_id")
				}
			}
			for _, record := range records {
				if record.ResourceType == "payments" {
					locationID := stringValue(record.Data, "location_id")
					if locationID == "" {
						locationID = orderLocations[stringValue(record.Data, "order_id")]
					}
					paymentLocations[record.ID] = locationID
				}
			}

			summaries := map[string]*reconcileCloseSummary{}
			get := func(locationID string) *reconcileCloseSummary {
				if locationID == "" {
					locationID = "unknown"
				}
				if summaries[locationID] == nil {
					summaries[locationID] = &reconcileCloseSummary{LocationID: locationID, LocationName: locations[locationID]}
				}
				return summaries[locationID]
			}
			for _, record := range records {
				if reconcileCloseRecordTime(record).Before(cutoff) {
					continue
				}
				switch record.ResourceType {
				case "orders":
					if strings.EqualFold(stringValue(record.Data, "state"), "COMPLETED") {
						s := get(stringValue(record.Data, "location_id"))
						s.OrderCount++
						s.OrderSales += firstInt(record.Data, []string{"total_money", "amount"}, []string{"net_amounts", "total_money", "amount"})
						setReconcileCurrency(s, record.Data, []string{"total_money", "currency"}, []string{"net_amounts", "total_money", "currency"})
					}
				case "payments":
					status := strings.ToUpper(stringValue(record.Data, "status"))
					if status == "COMPLETED" {
						s := get(paymentLocations[record.ID])
						s.PaymentCount++
						amount, currency := reconcilePaymentMoney(record.Data)
						s.GrossPayments += amount
						if s.Currency == "" {
							s.Currency = currency
						}
					}
				case "refunds":
					if !reconcileRefundCompleted(record.Data) {
						continue
					}
					locationID := firstString(record.Data, []string{"location_id"}, []string{"destination", "location_id"}, []string{"destination_details", "location_id"})
					if paymentID := stringValue(record.Data, "payment_id"); paymentID != "" {
						if paymentLocation := paymentLocations[paymentID]; paymentLocation != "" {
							locationID = paymentLocation
						}
					}
					s := get(locationID)
					s.RefundCount++
					s.Refunds += intValue(record.Data, "amount_money", "amount")
					setReconcileCurrency(s, record.Data, []string{"amount_money", "currency"})
				case "disputes":
					locationID := stringValue(record.Data, "location_id")
					if locationID == "" {
						locationID = paymentLocations[stringValue(record.Data, "payment_id")]
					}
					s := get(locationID)
					s.DisputeCount++
					s.Disputes += intValue(record.Data, "amount_money", "amount")
					status := strings.ToUpper(stringValue(record.Data, "state"))
					if status == "LOST" || status == "ACCEPTED" {
						s.DisputeLosses += intValue(record.Data, "amount_money", "amount")
					}
					setReconcileCurrency(s, record.Data, []string{"amount_money", "currency"})
				case "payouts":
					status := strings.ToUpper(stringValue(record.Data, "status"))
					if status == "SENT" || status == "PAID" || status == "COMPLETED" {
						s := get(firstString(record.Data, []string{"location_id"}, []string{"destination", "location_id"}))
						s.PayoutCount++
						s.Payouts += intValue(record.Data, "amount_money", "amount")
						setReconcileCurrency(s, record.Data, []string{"amount_money", "currency"})
					}
				}
			}

			out := make([]reconcileCloseSummary, 0, len(summaries))
			for _, s := range summaries {
				s.NetCollected = s.GrossPayments - s.Refunds - s.DisputeLosses
				s.OrderSalesMinusPayments = s.OrderSales - s.GrossPayments
				s.NetCollectedMinusPayouts = s.NetCollected - s.Payouts
				out = append(out, *s)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].LocationID < out[j].LocationID })
			return flags.printJSON(cmd, map[string]any{
				"data_source": "local", "since": flagSince, "cutoff": cutoff, "locations": out,
				"limitations": []string{
					"Amounts are Square minor currency units.",
					"Order sales, payments, refunds, disputes, and payouts are each filtered by their own record timestamp; timing differences are expected.",
					"Net collected subtracts completed refunds and LOST or ACCEPTED dispute losses; other dispute amounts remain visible as exposure.",
					"Payouts include only locally synced SENT, PAID, or COMPLETED records and can cover payments outside this window.",
				},
			})
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "24h", "Reconcile records updated within this period (for example 24h or 7d)")
	return cmd
}

// Close reports care about when financial state last became effective, not
// when an entity was originally created. Square exposes that differently by
// resource, so prefer status/update timestamps before the generic fallback.
func reconcileCloseRecordTime(record localSquareRecord) time.Time {
	var keys []string
	switch record.ResourceType {
	case "orders":
		keys = []string{"updated_at", "closed_at", "created_at"}
	case "payments":
		keys = []string{"updated_at", "completed_at", "created_at"}
	case "refunds":
		keys = []string{"updated_at", "created_at"}
	case "disputes":
		keys = []string{"updated_at", "created_at"}
	case "payouts":
		keys = []string{"updated_at", "created_at", "arrival_date"}
	default:
		return recordTime(record)
	}
	for _, key := range keys {
		if raw := stringValue(record.Data, key); raw != "" {
			if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
				return parsed
			}
			if parsed, err := time.Parse("2006-01-02", raw); err == nil {
				return parsed
			}
		}
	}
	return record.SyncedAt
}

func firstInt(data map[string]any, paths ...[]string) int64 {
	for _, path := range paths {
		if value := intValue(data, path...); value != 0 {
			return value
		}
	}
	return 0
}

func reconcilePaymentMoney(data map[string]any) (int64, string) {
	for _, key := range []string{"total_money", "amount_money"} {
		money, ok := data[key].(map[string]any)
		if !ok {
			continue
		}
		amountRaw, ok := money["amount"]
		if !ok {
			continue
		}
		var amount int64
		switch value := amountRaw.(type) {
		case float64:
			amount = int64(value)
		case json.Number:
			amount, _ = value.Int64()
		default:
			continue
		}
		currency, _ := money["currency"].(string)
		return amount, currency
	}
	return 0, ""
}

func reconcileRefundCompleted(data map[string]any) bool {
	return strings.EqualFold(stringValue(data, "status"), "COMPLETED")
}

func setReconcileCurrency(summary *reconcileCloseSummary, data map[string]any, paths ...[]string) {
	if summary.Currency == "" {
		summary.Currency = firstString(data, paths...)
	}
}
