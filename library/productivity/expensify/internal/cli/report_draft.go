// Copyright 2026 matt-van-horn. Licensed under Apache-2.0.
// `report draft` — create a report and auto-attach all local unreported expenses in range.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/expensify/internal/store"

	"github.com/spf13/cobra"
)

func newReportDraftCmd(flags *rootFlags) *cobra.Command {
	var sinceStr, untilStr, title, policyID string
	var previewOnly bool
	cmd := &cobra.Command{
		Use:     "draft",
		Short:   "Create a report and attach every local unreported expense in a date range",
		Example: `  expensify-pp-cli report draft --since 2026-04-01 --title "April expenses" --policy ABC123`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would create a draft report and attach un-reported expenses in the date range")
				return nil
			}
			if sinceStr == "" {
				return usageErr(fmt.Errorf("--since is required"))
			}
			if title == "" {
				return usageErr(fmt.Errorf("--title is required"))
			}
			if policyID == "" {
				return usageErr(fmt.Errorf("--policy is required"))
			}
			since, err := time.Parse("2006-01-02", sinceStr)
			if err != nil {
				return usageErr(fmt.Errorf("--since must be YYYY-MM-DD: %w", err))
			}
			until := time.Now()
			if untilStr != "" {
				until, err = time.Parse("2006-01-02", untilStr)
				if err != nil {
					return usageErr(fmt.Errorf("--until must be YYYY-MM-DD: %w", err))
				}
			}

			st, err := store.Open("")
			if err != nil {
				return configErr(err)
			}
			defer st.Close()
			all, err := st.ListUnreportedSince(since, policyID)
			if err != nil {
				return apiErr(err)
			}
			var candidates []store.Expense
			untilStr := until.Format("2006-01-02")
			for _, e := range all {
				if e.Date != "" && e.Date > untilStr {
					continue
				}
				candidates = append(candidates, e)
			}

			w := cmd.OutOrStdout()
			if previewOnly || flags.dryRun {
				fmt.Fprintf(w, "DRY RUN: would draft report %q on policy %s and attach %d expenses\n",
					title, policyID, len(candidates))
				for _, e := range candidates {
					fmt.Fprintf(w, "  %s  %s  %.2f  %s\n",
						e.Date, e.TransactionID, float64(e.Amount)/100, truncate(e.Merchant, 30))
				}
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// 1. CreateReport
			createBody := map[string]any{
				"reportName": title,
				"policyID":   policyID,
				"type":       "expense",
			}
			data, status, err := c.Post(cmd.Context(), "/CreateReport", createBody)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if status < 200 || status >= 300 {
				return apiErr(fmt.Errorf("CreateReport returned HTTP %d", status))
			}
			reportID := extractReportID(data)
			if reportID == "" {
				return apiErr(fmt.Errorf("CreateReport did not return a reportID: %s", truncate(string(data), 200)))
			}

			// 2. AddExpensesToReport
			if len(candidates) > 0 {
				ids := make([]string, 0, len(candidates))
				for _, e := range candidates {
					ids = append(ids, e.TransactionID)
				}
				addBody := map[string]any{
					"reportID":       reportID,
					"transactionIDs": ids,
				}
				if _, astatus, aerr := c.Post(cmd.Context(), "/AddExpensesToReport", addBody); aerr != nil {
					return classifyAPIError(fmt.Errorf("AddExpensesToReport failed: %w", aerr), flags)
				} else if astatus < 200 || astatus >= 300 {
					return apiErr(fmt.Errorf("AddExpensesToReport returned HTTP %d", astatus))
				}
			}

			fmt.Fprintf(w, "Drafted report %s with %d expenses attached (range %s .. %s, policy %s).\n",
				reportID, len(candidates), sinceStr, until.Format("2006-01-02"), policyID)
			return nil
		},
	}
	cmd.Flags().StringVar(&sinceStr, "since", "", "Start date YYYY-MM-DD (required)")
	cmd.Flags().StringVar(&untilStr, "until", "", "End date YYYY-MM-DD (default today)")
	cmd.Flags().StringVar(&title, "title", "", "Report title (required)")
	cmd.Flags().StringVar(&policyID, "policy", "", "Workspace/policy ID (required)")
	cmd.Flags().BoolVar(&previewOnly, "dry-run", false, "Preview without sending")
	return cmd
}

func extractReportID(data json.RawMessage) string {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	for _, k := range []string{"reportID", "report_id", "id"} {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
		if v, ok := m[k].(float64); ok && v != 0 {
			return fmt.Sprintf("%d", int64(v))
		}
	}
	// Nested report object
	if r, ok := m["report"].(map[string]any); ok {
		for _, k := range []string{"reportID", "report_id", "id"} {
			if v, ok := r[k].(string); ok && v != "" {
				return v
			}
			if v, ok := r[k].(float64); ok && v != 0 {
				return fmt.Sprintf("%d", int64(v))
			}
		}
	}
	return ""
}
