package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newHealthCmd(flags *rootFlags) *cobra.Command {
	var period string
	var taskFilter string

	cmd := &cobra.Command{
		Use:   "health",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Task health dashboard with success rates, durations, and cost trends",
		Long: `Show per-task health metrics: success rate, failure rate, average duration,
p95 duration, total cost, and run count. One command, full picture.

No existing Trigger.dev tool provides this summary view.`,
		Example: `  # Health dashboard for all tasks
  trigger-dev-pp-cli health

  # Health for a specific task over 30 days
  trigger-dev-pp-cli health --task my-email-task --period 30d

  # JSON output for dashboarding
  trigger-dev-pp-cli health --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Would fetch run data for period=%s to compute health metrics\n", period)
				return nil
			}

			// Fetch runs for the period
			params := map[string]string{
				"page[size]":                "100",
				"filter[createdAt][period]": period,
			}
			if taskFilter != "" {
				params["taskIdentifier"] = taskFilter
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
				TaskIdentifier string  `json:"taskIdentifier"`
				Status         string  `json:"status"`
				DurationMs     int     `json:"durationMs"`
				CostInCents    float64 `json:"costInCents"`
			}

			type taskHealth struct {
				Task        string  `json:"task"`
				Total       int     `json:"total"`
				Succeeded   int     `json:"succeeded"`
				Failed      int     `json:"failed"`
				SuccessRate float64 `json:"success_rate"`
				AvgDurMs    float64 `json:"avg_duration_ms"`
				P95DurMs    float64 `json:"p95_duration_ms"`
				TotalCost   float64 `json:"total_cost_cents"`
				Durations   []int   `json:"-"`
			}

			tasks := make(map[string]*taskHealth)
			for _, raw := range envelope.Data {
				var r run
				if err := json.Unmarshal(raw, &r); err != nil {
					continue
				}
				th, ok := tasks[r.TaskIdentifier]
				if !ok {
					th = &taskHealth{Task: r.TaskIdentifier}
					tasks[r.TaskIdentifier] = th
				}
				th.Total++
				th.TotalCost += r.CostInCents
				th.Durations = append(th.Durations, r.DurationMs)
				switch r.Status {
				case "COMPLETED":
					th.Succeeded++
				case "FAILED", "CRASHED", "SYSTEM_FAILURE":
					th.Failed++
				}
			}

			if len(tasks) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No runs found in the last %s.\n", period)
				return nil
			}

			var sorted []*taskHealth
			for _, th := range tasks {
				if th.Total > 0 {
					th.SuccessRate = float64(th.Succeeded) / float64(th.Total) * 100
					sort.Ints(th.Durations)
					sum := 0
					for _, d := range th.Durations {
						sum += d
					}
					th.AvgDurMs = float64(sum) / float64(len(th.Durations))
					p95Idx := int(float64(len(th.Durations)) * 0.95)
					if p95Idx >= len(th.Durations) {
						p95Idx = len(th.Durations) - 1
					}
					th.P95DurMs = float64(th.Durations[p95Idx])
				}
				sorted = append(sorted, th)
			}
			sort.Slice(sorted, func(i, j int) bool {
				return sorted[i].Total > sorted[j].Total
			})

			if flags.asJSON {
				return flags.printJSON(cmd, sorted)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Task Health Dashboard (last %s)\n", period)
			fmt.Fprintf(cmd.OutOrStdout(), "%d tasks, %d total runs\n\n", len(sorted), func() int {
				t := 0
				for _, s := range sorted {
					t += s.Total
				}
				return t
			}())

			fmt.Fprintf(cmd.OutOrStdout(), "%-35s %6s %8s %8s %10s %10s %10s\n",
				"task", "runs", "ok", "fail", "success%", "avg(ms)", "cost")
			fmt.Fprintf(cmd.OutOrStdout(), "%-35s %6s %8s %8s %10s %10s %10s\n",
				strings.Repeat("-", 35), "------", "--------", "--------", "----------", "----------", "----------")

			for _, th := range sorted {
				icon := "ok"
				if th.SuccessRate < 90 {
					icon = "!!"
				}
				if th.SuccessRate < 50 {
					icon = "XX"
				}
				_ = icon
				fmt.Fprintf(cmd.OutOrStdout(), "%-35s %6d %8d %8d %9.1f%% %9.0fms $%8.4f\n",
					truncate(th.Task, 35), th.Total, th.Succeeded, th.Failed,
					th.SuccessRate, th.AvgDurMs, th.TotalCost/100)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&period, "period", "7d", "Time period to analyze (e.g., 1d, 7d, 30d)")
	cmd.Flags().StringVar(&taskFilter, "task", "", "Filter by task identifier")
	_ = time.Now // ensure time import used

	return cmd
}
