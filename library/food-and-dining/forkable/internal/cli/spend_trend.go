// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
//
// spend-trend: bucket lunch spend into per-week or per-month totals. Live
// GraphQL fetch of myDeliveries with per-delivery receipts. Hand-authored;
// preserved across generate --force.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

const spendTrendQuery = `query { myDeliveries (from: "2000-01-01") { id forDeliveryAt userReceipt { id subtotal feesTotal due copayAmount } } }`

type spendDelivery struct {
	ForDeliveryAt string `json:"forDeliveryAt"`
	UserReceipt   struct {
		Subtotal  float64 `json:"subtotal"`
		FeesTotal float64 `json:"feesTotal"`
		Due       float64 `json:"due"`
		CopayAmt  float64 `json:"copayAmount"`
	} `json:"userReceipt"`
}

type spendBucket struct {
	Period     string  `json:"period"`
	Deliveries int     `json:"deliveries"`
	Subtotal   float64 `json:"subtotal"`
	Fees       float64 `json:"fees"`
	Due        float64 `json:"due"`
	Copay      float64 `json:"copay"`
}

type spendTrendView struct {
	Buckets []spendBucket `json:"buckets"`
	By      string        `json:"by"`
	Since   string        `json:"since,omitempty"`
	Total   spendBucket   `json:"total"`
}

func newNovelSpendTrendCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagBy string

	cmd := &cobra.Command{
		Use:         "spend-trend",
		Short:       "Bucket lunch spend into per-week or per-month totals with CSV export.",
		Long:        "Aggregates your Forkable delivery receipts (subtotal, fees, copay, due) into per-week or per-month buckets. Forkable's app shows one receipt at a time and has no cross-period view.",
		Example:     "  forkable-pp-cli spend-trend --since 6mo --by month --csv",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				emitDryRunShortCircuit(cmd, flags, "aggregate lunch spend over time")
				return nil
			}
			if flagBy != "week" && flagBy != "month" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--by must be 'week' or 'month'"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			data, err := fetchGraphQL(ctx, flags, spendTrendQuery)
			if err != nil {
				return err
			}
			var wrap struct {
				MyDeliveries []spendDelivery `json:"myDeliveries"`
			}
			if err := json.Unmarshal(data, &wrap); err != nil {
				return fmt.Errorf("parsing deliveries: %w", err)
			}
			cutoff := sinceCutoffISO(flagSince)
			bucketMap := map[string]*spendBucket{}
			var total spendBucket
			total.Period = "total"
			for _, d := range wrap.MyDeliveries {
				if !dateOnOrAfter(d.ForDeliveryAt, cutoff) || len(d.ForDeliveryAt) < 10 {
					continue
				}
				key := periodKey(d.ForDeliveryAt, flagBy)
				b := bucketMap[key]
				if b == nil {
					b = &spendBucket{Period: key}
					bucketMap[key] = b
				}
				b.Deliveries++
				b.Subtotal += d.UserReceipt.Subtotal
				b.Fees += d.UserReceipt.FeesTotal
				b.Due += d.UserReceipt.Due
				b.Copay += d.UserReceipt.CopayAmt
				total.Deliveries++
				total.Subtotal += d.UserReceipt.Subtotal
				total.Fees += d.UserReceipt.FeesTotal
				total.Due += d.UserReceipt.Due
				total.Copay += d.UserReceipt.CopayAmt
			}
			buckets := make([]spendBucket, 0, len(bucketMap))
			for _, b := range bucketMap {
				buckets = append(buckets, *b)
			}
			sort.Slice(buckets, func(i, j int) bool { return buckets[i].Period < buckets[j].Period })
			view := spendTrendView{Buckets: buckets, By: flagBy, Since: cutoff, Total: total}
			if flags.asJSON || flags.agent || flags.csv || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(buckets) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No spend found for the given window.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-10s %10s %10s %10s %10s %6s\n", "period", "subtotal", "fees", "copay", "due", "n")
			for _, b := range buckets {
				fmt.Fprintf(cmd.OutOrStdout(), "%-10s %10.2f %10.2f %10.2f %10.2f %6d\n", b.Period, b.Subtotal, b.Fees, b.Copay, b.Due, b.Deliveries)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-10s %10.2f %10.2f %10.2f %10.2f %6d\n", "TOTAL", total.Subtotal, total.Fees, total.Copay, total.Due, total.Deliveries)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Only include deliveries on or after this window (e.g. 6mo, 12w)")
	cmd.Flags().StringVar(&flagBy, "by", "month", "Bucket period: week or month")
	return cmd
}

// periodKey returns the bucket key for an ISO date string given "week" or
// "month" granularity. Week keys use ISO year-week (e.g. 2026-W07).
func periodKey(iso, by string) string {
	t, err := time.Parse("2006-01-02", iso[:10])
	if err != nil {
		return iso[:min(7, len(iso))]
	}
	if by == "week" {
		y, w := t.ISOWeek()
		return fmt.Sprintf("%d-W%02d", y, w)
	}
	return t.Format("2006-01")
}
