// Copyright 2026 Derick Ng and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command. Implemented body — regen preserves this file.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelRecognizedRevenueCmd(flags *rootFlags) *cobra.Command {
	var flagFrom, flagTo, flagBucket, flagItemType, flagBudget, flagCompany, flagProject string

	cmd := &cobra.Command{
		Use:   "recognized-revenue",
		Short: "Recognized (earned) revenue grouped by budget and month across a date range.",
		Long: "Query the financial_item_reports endpoint for recognized revenue grouped by budget and " +
			"time period. Money values are integer smallest-currency-units (cents). Use 'invoiced' for " +
			"the billed side and 'reconcile' to compare the two.",
		Example:     "  productive-pp-cli recognized-revenue --from 2026-01-01 --to 2026-06-30 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would query financial_item_reports grouped by budget,date")
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
			params := map[string]string{}
			applyReportGroupAndDates(params, "budget", flagBucket, flagFrom, flagTo)
			if flagItemType != "" {
				params["filter[financial_item_type][eq]"] = flagItemType
			}
			if flagBudget != "" {
				params["filter[budget_id][eq]"] = flagBudget
			}
			if flagCompany != "" {
				params["filter[company_id][eq]"] = flagCompany
			}
			if flagProject != "" {
				params["filter[project_id][eq]"] = flagProject
			}
			rows, included, err := fetchFinancialItemReport(ctx, c, params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			out := make([]map[string]any, 0, len(rows))
			for _, r := range rows {
				out = append(out, flattenReportRow(r, included))
			}
			sort.SliceStable(out, func(i, j int) bool {
				bi := rowStringMap(out[i], "budget_name")
				bj := rowStringMap(out[j], "budget_name")
				if bi != bj {
					return bi < bj
				}
				return rowStringMap(out[i], "date_period") < rowStringMap(out[j], "date_period")
			})
			data, err := json.Marshal(out)
			if err != nil {
				return err
			}
			return printOutputWithFlagsMeta(cmd.OutOrStdout(), data, flags, map[string]any{"source": "live", "rows": len(out)})
		},
	}
	cmd.Flags().StringVar(&flagFrom, "from", "", "Start of date range, YYYY-MM-DD (required)")
	cmd.Flags().StringVar(&flagTo, "to", "", "End of date range, YYYY-MM-DD (required)")
	cmd.Flags().StringVar(&flagBucket, "bucket", "month", "Time bucket: day|week|month|quarter|year")
	cmd.Flags().StringVar(&flagItemType, "financial-item-type", "service", "Financial item type filter (service, invoice_attribution, time_entry, expense, booking_item, revenue_item); empty for all")
	cmd.Flags().StringVar(&flagBudget, "budget", "", "Restrict to a single budget/deal id")
	cmd.Flags().StringVar(&flagCompany, "company", "", "Restrict to a company id")
	cmd.Flags().StringVar(&flagProject, "project", "", "Restrict to a project id")
	return cmd
}

// rowStringMap reads a string field from a flattened row map (helper for sorts).
func rowStringMap(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", m[key])
	}
	return ""
}
