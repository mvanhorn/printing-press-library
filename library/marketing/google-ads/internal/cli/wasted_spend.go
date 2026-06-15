// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

// gaqlSearchTermRow is one results[] entry from a search_term_view GAQL query.
type gaqlSearchTermRow struct {
	SearchTermView struct {
		SearchTerm string `json:"searchTerm"`
	} `json:"searchTermView"`
	Metrics struct {
		CostMicros  flexInt `json:"costMicros"`
		Clicks      flexInt `json:"clicks"`
		Conversions float64 `json:"conversions"`
	} `json:"metrics"`
}

// wastedSpendResult is one ranked row of money spent with zero conversions.
type wastedSpendResult struct {
	SearchTerm  string  `json:"search_term"`
	Cost        float64 `json:"cost"`
	CostMicros  int64   `json:"cost_micros"`
	Clicks      int64   `json:"clicks"`
	Conversions float64 `json:"conversions"`
}

func newWastedSpendCmd(flags *rootFlags) *cobra.Command {
	var flagCustomerID string
	var flagDays int
	var flagMinCost float64
	var flagLimit int

	cmd := &cobra.Command{
		Use:         "wasted-spend",
		Short:       "Find search terms burning budget with zero conversions",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  google-ads-pp-cli wasted-spend --customer-id 1234567890
  google-ads-pp-cli wasted-spend --customer-id 1234567890 --days 30 --min-cost 5 --limit 25`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagCustomerID == "" {
				return fmt.Errorf("required flag %q not set", "customer-id")
			}
			if flagDays <= 0 {
				return fmt.Errorf("--days must be greater than 0")
			}
			if flagMinCost < 0 {
				return fmt.Errorf("--min-cost cannot be negative")
			}

			query := buildWastedSpendQuery(flagDays, time.Now().UTC())

			// Dry-run: emit nothing (composite read-only contract).
			if flags.dryRun {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			rows, err := fetchGAQLRows[gaqlSearchTermRow](c, flagCustomerID, query)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			results := scoreWastedSpend(rows, flagMinCost, flagLimit)

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				headers := []string{"SEARCH TERM", "COST", "CLICKS", "CONV"}
				tableRows := make([][]string, 0, len(results))
				for _, r := range results {
					tableRows = append(tableRows, []string{
						r.SearchTerm,
						fmt.Sprintf("%.2f", r.Cost),
						strconv.FormatInt(r.Clicks, 10),
						fmt.Sprintf("%.0f", r.Conversions),
					})
				}
				return flags.printTable(cmd, headers, tableRows)
			}

			payload := map[string]any{
				"customer_id": flagCustomerID,
				"window_days": flagDays,
				"min_cost":    flagMinCost,
				"count":       len(results),
				"results":     results,
			}
			return flags.printJSON(cmd, payload)
		},
	}

	cmd.Flags().StringVar(&flagCustomerID, "customer-id", "", "Google Ads customer ID (required).")
	cmd.Flags().IntVar(&flagDays, "days", 7, "Look-back window in days.")
	cmd.Flags().Float64Var(&flagMinCost, "min-cost", 1, "Minimum spend (in account currency) for a term to count as wasted.")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum rows to return after ranking.")
	return cmd
}

// buildWastedSpendQuery builds the GAQL over search_term_view for an inclusive
// window of `days` ending at `now`. Filtering to zero-conversion terms happens
// client-side in scoreWastedSpend so the rule stays testable.
func buildWastedSpendQuery(days int, now time.Time) string {
	end := now
	start := end.AddDate(0, 0, -(days - 1))
	return fmt.Sprintf(
		"SELECT search_term_view.search_term, metrics.cost_micros, metrics.clicks, metrics.conversions "+
			"FROM search_term_view WHERE segments.date BETWEEN '%s' AND '%s' "+
			"ORDER BY metrics.cost_micros DESC",
		start.Format("2006-01-02"), end.Format("2006-01-02"),
	)
}

// scoreWastedSpend keeps terms with spend >= minCost and zero conversions,
// ranks them by cost descending, and caps at limit (limit <= 0 means no cap).
func scoreWastedSpend(rows []gaqlSearchTermRow, minCost float64, limit int) []wastedSpendResult {
	results := make([]wastedSpendResult, 0, len(rows))
	for _, row := range rows {
		if row.Metrics.Conversions > 0 {
			continue
		}
		cost := float64(row.Metrics.CostMicros) / 1e6
		if cost < minCost {
			continue
		}
		results = append(results, wastedSpendResult{
			SearchTerm:  row.SearchTermView.SearchTerm,
			Cost:        cost,
			CostMicros:  int64(row.Metrics.CostMicros),
			Clicks:      int64(row.Metrics.Clicks),
			Conversions: row.Metrics.Conversions,
		})
	}
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].CostMicros > results[j].CostMicros
	})
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results
}
