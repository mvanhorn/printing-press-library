// Copyright 2026 Derick Ng and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command. Implemented body — regen preserves this file.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

// reconcileRow is one budget×period reconciliation record. Money is integer cents.
type reconcileRow struct {
	BudgetID          string `json:"budget_id"`
	BudgetName        string `json:"budget_name,omitempty"`
	Period            string `json:"period"`
	RecognizedRevenue int64  `json:"recognized_revenue"`
	Invoiced          int64  `json:"invoiced"`
	Delta             int64  `json:"delta"`
	Flagged           bool   `json:"flagged"`
}

// pp:data-source computed
func newNovelReconcileCmd(flags *rootFlags) *cobra.Command {
	var flagFrom, flagTo, flagBucket, flagRecognizedType string
	var flagThreshold int64

	cmd := &cobra.Command{
		Use:   "reconcile",
		Short: "Reconcile recognized revenue vs invoiced per budget×period and flag deltas.",
		Long: "Fetches recognized revenue and invoiced (invoice_attribution) series from " +
			"financial_item_reports over the same budget×period grouping, joins them locally, and " +
			"reports delta = recognized - invoiced (integer cents). Rows whose absolute delta exceeds " +
			"--threshold are flagged. This is the core revenue-recognition reconciliation.",
		Example:     "  productive-pp-cli reconcile --from 2026-01-01 --to 2026-06-30 --threshold 100 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would query recognized + invoiced financial_item_reports and join by budget,period")
				return nil
			}
			if flagFrom == "" || flagTo == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--from and --to (YYYY-MM-DD) are required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Recognized series (default financial_item_type=service).
			recParams := map[string]string{}
			applyReportGroupAndDates(recParams, "budget", flagBucket, flagFrom, flagTo)
			if flagRecognizedType != "" {
				recParams["filter[financial_item_type][eq]"] = flagRecognizedType
			}
			recRows, recInc, err := fetchFinancialItemReport(ctx, c, recParams)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Invoiced series (financial_item_type=invoice_attribution).
			invParams := map[string]string{}
			applyReportGroupAndDates(invParams, "budget", flagBucket, flagFrom, flagTo)
			invParams["filter[financial_item_type][eq]"] = "invoice_attribution"
			invRows, invInc, err := fetchFinancialItemReport(ctx, c, invParams)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Join by budget_id|period.
			type key struct{ budget, period string }
			joined := map[key]*reconcileRow{}
			name := map[string]string{}

			absorb := func(rows []japiResource, inc map[string]japiResource, recognized bool) {
				for _, r := range rows {
					bid := relID(r.Relationships["budget"])
					period := rowString(r.Attributes, "date_period", "date", "financial_item_date")
					k := key{bid, period}
					row := joined[k]
					if row == nil {
						row = &reconcileRow{BudgetID: bid, Period: period}
						joined[k] = row
					}
					if bn := budgetName(inc, bid); bn != "" {
						name[bid] = bn
					}
					if recognized {
						row.RecognizedRevenue += rowMoneyCents(r.Attributes, "total_recognized_revenue")
					} else {
						row.Invoiced += rowMoneyCents(r.Attributes, "total_invoiced")
					}
				}
			}
			absorb(recRows, recInc, true)
			absorb(invRows, invInc, false)

			out := make([]reconcileRow, 0, len(joined))
			flaggedCount := 0
			for k, row := range joined {
				row.BudgetName = name[k.budget]
				row.Delta = row.RecognizedRevenue - row.Invoiced
				d := row.Delta
				if d < 0 {
					d = -d
				}
				row.Flagged = d > flagThreshold
				if row.Flagged {
					flaggedCount++
				}
				out = append(out, *row)
			}
			sort.SliceStable(out, func(i, j int) bool {
				if out[i].BudgetName != out[j].BudgetName {
					return out[i].BudgetName < out[j].BudgetName
				}
				return out[i].Period < out[j].Period
			})
			data, err := json.Marshal(out)
			if err != nil {
				return err
			}
			return printOutputWithFlagsMeta(cmd.OutOrStdout(), data, flags, map[string]any{
				"source": "live", "rows": len(out), "flagged": flaggedCount, "threshold_cents": flagThreshold,
			})
		},
	}
	cmd.Flags().StringVar(&flagFrom, "from", "", "Start of date range, YYYY-MM-DD (required)")
	cmd.Flags().StringVar(&flagTo, "to", "", "End of date range, YYYY-MM-DD (required)")
	cmd.Flags().StringVar(&flagBucket, "bucket", "month", "Time bucket: day|week|month|quarter|year")
	cmd.Flags().StringVar(&flagRecognizedType, "recognized-type", "service", "financial_item_type for the recognized series; empty for all types")
	cmd.Flags().Int64Var(&flagThreshold, "threshold", 0, "Flag rows whose |recognized - invoiced| exceeds this many cents")
	return cmd
}
