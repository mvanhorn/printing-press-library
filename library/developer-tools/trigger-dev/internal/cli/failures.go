package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newFailuresCmd(flags *rootFlags) *cobra.Command {
	var period string
	var topN int
	var groupBy string

	cmd := &cobra.Command{
		Use:   "failures",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Analyze failure patterns across tasks and time periods",
		Long: `Identify recurring failure patterns by analyzing failed runs across tasks,
error types, and time-of-day. Helps you find systemic issues that one-off
alerts miss.

Requires synced data (run 'sync --full' first for best results, or uses live API).`,
		Example: `  # Show failure patterns from the last 7 days
  trigger-dev-pp-cli failures --period 7d

  # Group failures by task
  trigger-dev-pp-cli failures --period 30d --group-by task

  # Top 5 failure patterns
  trigger-dev-pp-cli failures --period 7d --top 5 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Would analyze failures for period=%s grouped by %s\n", period, groupBy)
				return nil
			}

			// Fetch failed runs
			params := map[string]string{
				"status":                    "FAILED,CRASHED,SYSTEM_FAILURE",
				"page[size]":                "100",
				"filter[createdAt][period]": period,
			}

			resp, err := c.Get("/api/v1/runs", params)
			if err != nil {
				return classifyAPIError(err)
			}

			var envelope struct {
				Data []json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(resp, &envelope); err != nil {
				var arr []json.RawMessage
				if err2 := json.Unmarshal(resp, &arr); err2 == nil {
					envelope.Data = arr
				}
			}

			type failedRun struct {
				ID             string    `json:"id"`
				Status         string    `json:"status"`
				TaskIdentifier string    `json:"taskIdentifier"`
				CreatedAt      time.Time `json:"createdAt"`
				DurationMs     int       `json:"durationMs"`
				CostInCents    float64   `json:"costInCents"`
				Tags           []string  `json:"tags"`
			}

			var runs []failedRun
			for _, raw := range envelope.Data {
				var r failedRun
				if err := json.Unmarshal(raw, &r); err == nil {
					runs = append(runs, r)
				}
			}

			if len(runs) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No failures found in the last %s.\n", period)
				return nil
			}

			// Analysis
			type pattern struct {
				Key       string         `json:"key"`
				Count     int            `json:"count"`
				TotalCost float64        `json:"total_cost_cents"`
				AvgDurMs  float64        `json:"avg_duration_ms"`
				LastSeen  string         `json:"last_seen"`
				Statuses  map[string]int `json:"statuses"`
			}

			patterns := make(map[string]*pattern)
			for _, r := range runs {
				var key string
				switch groupBy {
				case "task":
					key = r.TaskIdentifier
				case "status":
					key = r.Status
				case "hour":
					key = fmt.Sprintf("%02d:00", r.CreatedAt.Local().Hour())
				default:
					key = r.TaskIdentifier
				}

				p, ok := patterns[key]
				if !ok {
					p = &pattern{Key: key, Statuses: make(map[string]int)}
					patterns[key] = p
				}
				p.Count++
				p.TotalCost += r.CostInCents
				p.AvgDurMs += float64(r.DurationMs)
				p.Statuses[r.Status]++
				if r.CreatedAt.Format(time.RFC3339) > p.LastSeen {
					p.LastSeen = r.CreatedAt.Local().Format("2006-01-02 15:04")
				}
			}

			// Finalize averages
			var sorted []*pattern
			for _, p := range patterns {
				p.AvgDurMs = p.AvgDurMs / float64(p.Count)
				sorted = append(sorted, p)
			}
			sort.Slice(sorted, func(i, j int) bool {
				return sorted[i].Count > sorted[j].Count
			})

			if topN > 0 && topN < len(sorted) {
				sorted = sorted[:topN]
			}

			if flags.asJSON {
				result := map[string]any{
					"period":          period,
					"total_failures":  len(runs),
					"unique_patterns": len(patterns),
					"patterns":        sorted,
				}
				return flags.printJSON(cmd, result)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Failure Analysis (last %s)\n", period)
			fmt.Fprintf(cmd.OutOrStdout(), "Total failures: %d across %d %s(s)\n\n", len(runs), len(patterns), groupBy)

			fmt.Fprintf(cmd.OutOrStdout(), "%-35s %6s %12s %10s  %s\n", groupBy, "count", "avg duration", "cost", "last seen")
			fmt.Fprintf(cmd.OutOrStdout(), "%-35s %6s %12s %10s  %s\n", strings.Repeat("-", 35), "------", "------------", "----------", "----------------")
			for _, p := range sorted {
				fmt.Fprintf(cmd.OutOrStdout(), "%-35s %6d %10.0fms $%8.4f  %s\n",
					truncate(p.Key, 35), p.Count, p.AvgDurMs, p.TotalCost/100, p.LastSeen)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&period, "period", "7d", "Time period to analyze (e.g., 1d, 7d, 30d)")
	cmd.Flags().IntVar(&topN, "top", 0, "Show only top N patterns (0 for all)")
	cmd.Flags().StringVar(&groupBy, "group-by", "task", "Group by: task, status, or hour")

	return cmd
}
