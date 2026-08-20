// Copyright 2026 Chris Rodriguez and contributors. Licensed under Apache-2.0. See LICENSE.

// PATCH: add `refund-rate` analytics command. Computes daily refund-rate
// trending over a window, optionally grouped by product, customer, or
// payment-method type, with threshold-flagging. Refund -> charge join is
// via the refund.charge field.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/stripe/internal/store"

	"github.com/spf13/cobra"
)

type refundRateDay struct {
	Date              string  `json:"date"`
	Group             string  `json:"group,omitempty"`
	SucceededCharges  int     `json:"succeeded_charges"`
	RefundedCharges   int     `json:"refunded_charges"`
	RefundRatePct     float64 `json:"refund_rate_pct"`
	OverThreshold     bool    `json:"over_threshold"`
}

type refundRateReport struct {
	Window         string          `json:"window"`
	From           string          `json:"from"`
	To             string          `json:"to"`
	By             string          `json:"by"`
	ThresholdPct   float64         `json:"threshold_pct"`
	OverThreshold  int             `json:"days_over_threshold"`
	Days           []refundRateDay `json:"days"`
}

func newRefundRateCmd(flags *rootFlags) *cobra.Command {
	var windowStr string
	var by string
	var thresholdPct float64
	var dbPath string

	cmd := &cobra.Command{
		Use:   "refund-rate",
		Short: "Daily refund-rate trending with optional grouping and threshold flagging",
		Long: `Compute per-day refund rate (refunded_charges / succeeded_charges) within
the window. Optional --by groups by product (via charge -> invoice_item -> price
-> product, falling back to "untagged"), customer, or payment_method type. Days
where the rate exceeds --threshold-pct are flagged with over_threshold=true.`,
		Example: `  # Last 7 days, global
  stripe-pp-cli refund-rate --json

  # Last 30 days, by product, flag days >5%
  stripe-pp-cli refund-rate --window 30d --by product --threshold-pct 5 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			window, err := parseHumanDuration(windowStr)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --window: %w", err))
			}
			if window == 0 {
				window = 7 * 24 * time.Hour
			}
			by = strings.ToLower(strings.TrimSpace(by))
			switch by {
			case "", "global", "product", "customer", "payment_method":
				// ok
			default:
				return usageErr(fmt.Errorf("--by must be one of: product, customer, payment_method (got %q)", by))
			}

			path := transcendenceDBPath(dbPath)
			db, err := store.OpenReadOnly(path)
			if err != nil {
				return configErr(fmt.Errorf("opening local database (%s): %w\nRun 'stripe-pp-cli sync' first.", path, err))
			}
			defer db.Close()

			report, err := computeRefundRate(db.DB(), window, by, thresholdPct)
			if err != nil {
				return apiErr(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}

	cmd.Flags().StringVar(&windowStr, "window", "7d", "Lookback window (e.g. 7d, 30d, 168h)")
	cmd.Flags().StringVar(&by, "by", "", "Grouping: product, customer, or payment_method (default: global)")
	cmd.Flags().Float64Var(&thresholdPct, "threshold-pct", 5.0, "Flag days where refund rate exceeds this percentage")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/stripe-pp-cli/data.db)")

	return cmd
}

// computeRefundRate aggregates charges and refunds by day (and optionally
// by group dimension) within the window. Refund→charge join is via the
// charge_id field on the refund record.
func computeRefundRate(db *sql.DB, window time.Duration, by string, thresholdPct float64) (refundRateReport, error) {
	now := time.Now().UTC()
	from := now.Add(-window)
	fromTS := from.Unix()
	toTS := now.Unix()

	// Pull charges (succeeded only — refund rate denominator is charges that
	// actually went through). Build a map of charge_id -> (day, group_label).
	chargeRows, err := db.Query(`SELECT id, data FROM resources WHERE resource_type='charges'
		AND json_extract(data,'$.status') = 'succeeded'
		AND CAST(IFNULL(json_extract(data,'$.created'),0) AS INTEGER) >= ?
		AND CAST(IFNULL(json_extract(data,'$.created'),0) AS INTEGER) <= ?`, fromTS, toTS)
	if err != nil {
		return refundRateReport{}, fmt.Errorf("querying charges: %w", err)
	}
	defer chargeRows.Close()

	type chargeMeta struct {
		day   string
		group string
	}
	chargeMetaByID := make(map[string]chargeMeta)
	dayGroupCharges := make(map[string]int) // key = day + "|" + group

	for chargeRows.Next() {
		var id, data string
		if err := chargeRows.Scan(&id, &data); err != nil {
			return refundRateReport{}, err
		}
		raw := json.RawMessage(data)
		ts, _ := jsonGetInt(raw, "created")
		day := time.Unix(ts, 0).UTC().Format("2006-01-02")
		group := chargeGroupLabel(db, raw, by)
		chargeMetaByID[id] = chargeMeta{day: day, group: group}
		key := day + "|" + group
		dayGroupCharges[key]++
	}
	if err := chargeRows.Err(); err != nil {
		return refundRateReport{}, err
	}

	// Pull refunds in window. Each refund's charge_id is the join key.
	refundRows, err := db.Query(`SELECT data FROM resources WHERE resource_type='refunds'
		AND CAST(IFNULL(json_extract(data,'$.created'),0) AS INTEGER) >= ?
		AND CAST(IFNULL(json_extract(data,'$.created'),0) AS INTEGER) <= ?`, fromTS, toTS)
	if err != nil {
		return refundRateReport{}, fmt.Errorf("querying refunds: %w", err)
	}
	defer refundRows.Close()

	dayGroupRefunds := make(map[string]int)
	for refundRows.Next() {
		var data string
		if err := refundRows.Scan(&data); err != nil {
			return refundRateReport{}, err
		}
		raw := json.RawMessage(data)
		chargeID, _ := jsonGet(raw, "charge")
		meta, hit := chargeMetaByID[chargeID]
		if !hit {
			// Charge isn't in this window (older than window OR missing from mirror).
			// Use the refund's own created timestamp + unknown group as a fallback.
			ts, _ := jsonGetInt(raw, "created")
			meta = chargeMeta{
				day:   time.Unix(ts, 0).UTC().Format("2006-01-02"),
				group: "untagged",
			}
		}
		key := meta.day + "|" + meta.group
		dayGroupRefunds[key]++
	}
	if err := refundRows.Err(); err != nil {
		return refundRateReport{}, err
	}

	// Emit one row per (day, group) where there was at least one charge.
	out := make([]refundRateDay, 0, len(dayGroupCharges))
	for key, charges := range dayGroupCharges {
		parts := strings.SplitN(key, "|", 2)
		day, group := parts[0], parts[1]
		refunds := dayGroupRefunds[key]
		pct := 0.0
		if charges > 0 {
			pct = float64(refunds) / float64(charges) * 100
		}
		out = append(out, refundRateDay{
			Date:             day,
			Group:            group,
			SucceededCharges: charges,
			RefundedCharges:  refunds,
			RefundRatePct:    roundTo(pct, 2),
			OverThreshold:    pct > thresholdPct,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date < out[j].Date
		}
		return out[i].Group < out[j].Group
	})

	overCount := 0
	for _, d := range out {
		if d.OverThreshold {
			overCount++
		}
	}

	return refundRateReport{
		Window:        window.String(),
		From:          from.Format("2006-01-02"),
		To:            now.Format("2006-01-02"),
		By:            by,
		ThresholdPct:  thresholdPct,
		OverThreshold: overCount,
		Days:          out,
	}, nil
}

// chargeGroupLabel computes a charge's group label based on the --by axis.
// Returns "global" for empty/global, otherwise the customer id, payment-method
// type, or product id (via charge -> invoice -> line.price.product walking).
// Falls back to "untagged" when the join can't complete.
func chargeGroupLabel(db *sql.DB, raw json.RawMessage, by string) string {
	switch by {
	case "", "global":
		return "global"
	case "customer":
		if v, ok := jsonGet(raw, "customer"); ok && v != "" {
			return v
		}
		return "untagged"
	case "payment_method":
		t := extractPaymentMethodType(raw)
		if t == "" {
			return "untagged"
		}
		return t
	case "product":
		// charges have an optional 'invoice' pointer; invoices have lines with prices.
		invoiceID, _ := jsonGet(raw, "invoice")
		if invoiceID == "" {
			return "untagged"
		}
		var invData string
		if err := db.QueryRow(
			`SELECT data FROM resources WHERE resource_type='invoices' AND id=?`, invoiceID,
		).Scan(&invData); err != nil {
			return "untagged"
		}
		pid := firstProductFromInvoice(json.RawMessage(invData))
		if pid == "" {
			return "untagged"
		}
		return pid
	}
	return "global"
}

// firstProductFromInvoice extracts the product id of the invoice's first
// line item's price. Stripe's invoice line shape: lines.data[].price.product.
// Returns "" if the path can't be walked.
func firstProductFromInvoice(raw json.RawMessage) string {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	linesRaw, ok := obj["lines"]
	if !ok || string(linesRaw) == "null" {
		return ""
	}
	var lines map[string]json.RawMessage
	if err := json.Unmarshal(linesRaw, &lines); err != nil {
		return ""
	}
	dataRaw, ok := lines["data"]
	if !ok {
		return ""
	}
	var arr []map[string]json.RawMessage
	if err := json.Unmarshal(dataRaw, &arr); err != nil || len(arr) == 0 {
		return ""
	}
	first := arr[0]
	priceRaw, ok := first["price"]
	if !ok || string(priceRaw) == "null" {
		return ""
	}
	var price map[string]json.RawMessage
	if err := json.Unmarshal(priceRaw, &price); err != nil {
		return ""
	}
	if v, ok := price["product"]; ok {
		var s string
		if json.Unmarshal(v, &s) == nil {
			return s
		}
		var inner map[string]json.RawMessage
		if json.Unmarshal(v, &inner) == nil {
			if idRaw, ok := inner["id"]; ok {
				var id string
				if json.Unmarshal(idRaw, &id) == nil {
					return id
				}
			}
		}
	}
	return ""
}
