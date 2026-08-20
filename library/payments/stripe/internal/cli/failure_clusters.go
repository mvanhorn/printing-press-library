// Copyright 2026 Chris Rodriguez and contributors. Licensed under Apache-2.0. See LICENSE.

// PATCH: add `failure-clusters` analytics command. Groups failed charges by
// failure_code, optionally also by payment_method type, and surfaces the top
// failure patterns. Different from dunning-queue (invoice-level vs charge-level).
// Ported from an aggregateOperations.declineReasons rollup pattern.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/stripe/internal/store"

	"github.com/spf13/cobra"
)

type failureCluster struct {
	FailureCode      string   `json:"failure_code"`
	PaymentMethod    string   `json:"payment_method_type,omitempty"`
	Count            int      `json:"count"`
	PercentOfTotal   float64  `json:"percent_of_total"`
	SampleChargeIDs  []string `json:"sample_charge_ids,omitempty"`
	TotalAmountCents int64    `json:"total_amount_cents"`
}

type failureClusterReport struct {
	Days               int              `json:"window_days"`
	TotalFailedCharges int              `json:"total_failed_charges"`
	MinCount           int              `json:"min_count"`
	ByPaymentMethod    bool             `json:"by_payment_method_type"`
	Clusters           []failureCluster `json:"clusters"`
}

func newFailureClustersCmd(flags *rootFlags) *cobra.Command {
	var days int
	var minCount int
	var byPMT bool
	var dbPath string
	var sampleSize int

	cmd := &cobra.Command{
		Use:   "failure-clusters",
		Short: "Group failed charges by failure_code; surface top failure patterns",
		Long: `Walk failed charges (resource_type=charges AND status='failed') within the
window. Bucket by failure_code; optionally also by payment_method type. Clusters
below --min-count are suppressed so noise doesn't crowd the signal. Sample charge
IDs are included so an operator can drill into a specific failure cluster.`,
		Example: `  # Last 30 days, default
  stripe-pp-cli failure-clusters --json

  # Last 7 days, breakdown by payment-method type
  stripe-pp-cli failure-clusters --days 7 --by-payment-method-type --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if days < 1 {
				return usageErr(fmt.Errorf("--days must be >= 1 (got %d)", days))
			}
			if minCount < 1 {
				return usageErr(fmt.Errorf("--min-count must be >= 1 (got %d)", minCount))
			}

			path := transcendenceDBPath(dbPath)
			db, err := store.OpenReadOnly(path)
			if err != nil {
				return configErr(fmt.Errorf("opening local database (%s): %w\nRun 'stripe-pp-cli sync' first.", path, err))
			}
			defer db.Close()

			report, err := computeFailureClusters(db.DB(), days, minCount, byPMT, sampleSize)
			if err != nil {
				return apiErr(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}

	cmd.Flags().IntVar(&days, "days", 30, "Lookback window in days")
	cmd.Flags().IntVar(&minCount, "min-count", 3, "Suppress clusters with fewer than N charges")
	cmd.Flags().BoolVar(&byPMT, "by-payment-method-type", false, "Sub-group each failure_code by payment_method type")
	cmd.Flags().IntVar(&sampleSize, "sample-size", 5, "Max sample charge IDs to include per cluster")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/stripe-pp-cli/data.db)")

	return cmd
}

// computeFailureClusters walks failed charges within `days` and aggregates by
// failure_code (and optionally by payment_method.type as a second axis).
func computeFailureClusters(db *sql.DB, days, minCount int, byPMT bool, sampleSize int) (failureClusterReport, error) {
	now := time.Now()
	cutoff := now.Add(-time.Duration(days) * 24 * time.Hour).Unix()

	rs, err := db.Query(`SELECT id, data FROM resources WHERE resource_type='charges'
		AND json_extract(data,'$.status') = 'failed'
		AND CAST(IFNULL(json_extract(data,'$.created'),0) AS INTEGER) >= ?`, cutoff)
	if err != nil {
		return failureClusterReport{}, fmt.Errorf("querying charges: %w", err)
	}
	defer rs.Close()

	// key -> cluster aggregator
	type agg struct {
		count   int
		total   int64
		samples []string
		code    string
		pmType  string
	}
	buckets := make(map[string]*agg)
	totalFailed := 0

	for rs.Next() {
		var id, data string
		if err := rs.Scan(&id, &data); err != nil {
			return failureClusterReport{}, err
		}
		raw := json.RawMessage(data)
		totalFailed++

		code, ok := jsonGet(raw, "failure_code")
		if !ok || code == "" {
			code = "unknown"
		}
		pmType := ""
		if byPMT {
			pmType = extractPaymentMethodType(raw)
			if pmType == "" {
				pmType = "unknown"
			}
		}
		key := code
		if byPMT {
			key = code + "|" + pmType
		}
		b, exists := buckets[key]
		if !exists {
			b = &agg{code: code, pmType: pmType}
			buckets[key] = b
		}
		b.count++
		if amount, ok := jsonGetInt(raw, "amount"); ok {
			b.total += amount
		}
		if len(b.samples) < sampleSize {
			b.samples = append(b.samples, id)
		}
	}
	if err := rs.Err(); err != nil {
		return failureClusterReport{}, err
	}

	clusters := make([]failureCluster, 0, len(buckets))
	for _, b := range buckets {
		if b.count < minCount {
			continue
		}
		pct := 0.0
		if totalFailed > 0 {
			pct = float64(b.count) / float64(totalFailed) * 100
		}
		clusters = append(clusters, failureCluster{
			FailureCode:      b.code,
			PaymentMethod:    b.pmType,
			Count:            b.count,
			PercentOfTotal:   roundTo(pct, 2),
			SampleChargeIDs:  b.samples,
			TotalAmountCents: b.total,
		})
	}
	sort.SliceStable(clusters, func(i, j int) bool {
		return clusters[i].Count > clusters[j].Count
	})

	return failureClusterReport{
		Days:               days,
		TotalFailedCharges: totalFailed,
		MinCount:           minCount,
		ByPaymentMethod:    byPMT,
		Clusters:           clusters,
	}, nil
}

// extractPaymentMethodType pulls payment_method_details.type from a charge JSON
// blob, defending against a missing or non-object payment_method_details.
func extractPaymentMethodType(raw json.RawMessage) string {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	pmd, ok := obj["payment_method_details"]
	if !ok || string(pmd) == "null" {
		return ""
	}
	var inner map[string]json.RawMessage
	if err := json.Unmarshal(pmd, &inner); err != nil {
		return ""
	}
	if v, ok := inner["type"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			return s
		}
	}
	return ""
}

// roundTo rounds a float to n decimal places. Used to keep percent strings
// tidy in JSON without forcing a string format.
func roundTo(v float64, n int) float64 {
	pow := 1.0
	for i := 0; i < n; i++ {
		pow *= 10
	}
	// add 0.5 for banker's round-half-up; simple and good enough for percents.
	if v >= 0 {
		return float64(int64(v*pow+0.5)) / pow
	}
	return float64(int64(v*pow-0.5)) / pow
}
