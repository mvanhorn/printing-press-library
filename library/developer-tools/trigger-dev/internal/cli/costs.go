package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newCostsCmd(flags *rootFlags) *cobra.Command {
	var period string
	var groupBy string
	var topN int

	cmd := &cobra.Command{
		Use:   "costs",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Analyze run costs by task, time period, and machine type",
		Long: `Track spending by task and time period. Find cost spikes and anomalies before
they hit your bill. Aggregates costInCents across runs.`,
		Example: `  # Cost breakdown by task over 30 days
  trigger-dev-pp-cli costs --period 30d

  # Top 5 most expensive tasks
  trigger-dev-pp-cli costs --period 30d --top 5

  # JSON output
  trigger-dev-pp-cli costs --period 7d --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Would analyze costs for period=%s grouped by %s\n", period, groupBy)
				return nil
			}

			params := map[string]string{
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

			type run struct {
				TaskIdentifier  string  `json:"taskIdentifier"`
				Status          string  `json:"status"`
				CostInCents     float64 `json:"costInCents"`
				BaseCostInCents float64 `json:"baseCostInCents"`
				DurationMs      int     `json:"durationMs"`
			}

			type costBucket struct {
				Key        string  `json:"key"`
				Runs       int     `json:"runs"`
				TotalCost  float64 `json:"total_cost_cents"`
				BaseCost   float64 `json:"base_cost_cents"`
				AvgCost    float64 `json:"avg_cost_cents"`
				TotalDurMs int     `json:"total_duration_ms"`
			}

			buckets := make(map[string]*costBucket)
			var totalCost float64
			for _, raw := range envelope.Data {
				var r run
				if err := json.Unmarshal(raw, &r); err != nil {
					continue
				}

				key := r.TaskIdentifier
				if groupBy == "status" {
					key = r.Status
				}

				b, ok := buckets[key]
				if !ok {
					b = &costBucket{Key: key}
					buckets[key] = b
				}
				b.Runs++
				b.TotalCost += r.CostInCents
				b.BaseCost += r.BaseCostInCents
				b.TotalDurMs += r.DurationMs
				totalCost += r.CostInCents
			}

			var sorted []*costBucket
			for _, b := range buckets {
				if b.Runs > 0 {
					b.AvgCost = b.TotalCost / float64(b.Runs)
				}
				sorted = append(sorted, b)
			}
			sort.Slice(sorted, func(i, j int) bool {
				return sorted[i].TotalCost > sorted[j].TotalCost
			})

			if topN > 0 && topN < len(sorted) {
				sorted = sorted[:topN]
			}

			if flags.asJSON {
				result := map[string]any{
					"period":     period,
					"total_cost": totalCost,
					"buckets":    sorted,
				}
				return flags.printJSON(cmd, result)
			}

			if len(sorted) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No runs found in the last %s.\n", period)
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Cost Analysis (last %s) - Total: $%.4f\n\n", period, totalCost/100)

			fmt.Fprintf(cmd.OutOrStdout(), "%-35s %6s %12s %10s %10s\n",
				groupBy, "runs", "total cost", "avg cost", "% of total")
			fmt.Fprintf(cmd.OutOrStdout(), "%-35s %6s %12s %10s %10s\n",
				strings.Repeat("-", 35), "------", "------------", "----------", "----------")

			for _, b := range sorted {
				pct := 0.0
				if totalCost > 0 {
					pct = b.TotalCost / totalCost * 100
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-35s %6d $%10.4f $%8.4f %9.1f%%\n",
					truncate(b.Key, 35), b.Runs, b.TotalCost/100, b.AvgCost/100, pct)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&period, "period", "30d", "Time period to analyze (e.g., 1d, 7d, 30d)")
	cmd.Flags().StringVar(&groupBy, "group-by", "task", "Group by: task or status")
	cmd.Flags().IntVar(&topN, "top", 0, "Show only top N (0 for all)")

	return cmd
}
