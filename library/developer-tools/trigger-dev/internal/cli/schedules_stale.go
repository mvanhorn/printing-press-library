// Hand-authored novel feature: stale-schedule detection.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newSchedulesStaleCmd(flags *rootFlags) *cobra.Command {
	var staleDays int
	var minSuccessRate float64

	cmd := &cobra.Command{
		Use:         "stale",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Find schedules that stopped firing or whose recent runs all fail",
		Long: `Detects zombie schedules by cross-referencing the live schedules list
with the runs window for each schedule's task. A schedule is "stale" if:
  - it is disabled (Active=false), OR
  - it has zero runs in the last N days, OR
  - its run success rate falls below --min-success-rate.`,
		Example: `  trigger-dev-pp-cli schedules stale
  trigger-dev-pp-cli schedules stale --days 14 --min-success-rate 0.8 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would scan schedules for staleness over %d days (min success rate %.2f)\n", staleDays, minSuccessRate)
				return nil
			}
			schedResp, err := c.Get("/api/v1/schedules", map[string]string{"page[size]": "100"})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			type schedule struct {
				ID         string `json:"id"`
				Task       string `json:"task"`
				Active     bool   `json:"active"`
				ExternalID string `json:"externalId"`
				Generator  struct {
					Cron string `json:"cron"`
				} `json:"generator"`
			}
			type result struct {
				ScheduleID   string  `json:"schedule_id"`
				Task         string  `json:"task"`
				Cron         string  `json:"cron"`
				Active       bool    `json:"active"`
				Reason       string  `json:"reason"`
				RunsInWindow int     `json:"runs_in_window"`
				SuccessRate  float64 `json:"success_rate"`
			}
			period := fmt.Sprintf("%dd", staleDays)
			var results []result
			for _, raw := range unwrapEnvelope(schedResp) {
				var s schedule
				if err := json.Unmarshal(raw, &s); err != nil {
					continue
				}
				runsResp, _ := c.Get("/api/v1/runs", map[string]string{
					"taskIdentifier":            s.Task,
					"page[size]":                "100",
					"filter[createdAt][period]": period,
				})
				items := unwrapEnvelope(runsResp)
				totalRuns := len(items)
				succeeded := 0
				for _, r := range items {
					var run struct {
						Status string `json:"status"`
					}
					if json.Unmarshal(r, &run) == nil && run.Status == "COMPLETED" {
						succeeded++
					}
				}
				rate := 1.0
				if totalRuns > 0 {
					rate = float64(succeeded) / float64(totalRuns)
				}
				reason := ""
				switch {
				case !s.Active:
					reason = "disabled"
				case totalRuns == 0:
					reason = fmt.Sprintf("no runs in last %d days", staleDays)
				case rate < minSuccessRate:
					reason = fmt.Sprintf("success rate %.2f < %.2f", rate, minSuccessRate)
				}
				if reason != "" {
					results = append(results, result{
						ScheduleID:   s.ID,
						Task:         s.Task,
						Cron:         s.Generator.Cron,
						Active:       s.Active,
						Reason:       reason,
						RunsInWindow: totalRuns,
						SuccessRate:  rate,
					})
				}
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			if len(results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "All schedules are healthy (last %d days, min success %.2f).\n", staleDays, minSuccessRate)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Stale Schedules (%d found)\n\n", len(results))
			fmt.Fprintf(cmd.OutOrStdout(), "%-22s %-30s %-15s %-7s %-6s %s\n",
				"schedule", "task", "cron", "active", "rate", "reason")
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", strings.Repeat("-", 100))
			for _, r := range results {
				active := "yes"
				if !r.Active {
					active = "no"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-22s %-30s %-15s %-7s %-6.2f %s\n",
					truncate(r.ScheduleID, 22), truncate(r.Task, 30), truncate(r.Cron, 15), active, r.SuccessRate, r.Reason)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&staleDays, "days", 14, "Days without successful runs to consider stale")
	cmd.Flags().Float64Var(&minSuccessRate, "min-success-rate", 0.5, "Minimum success rate to be considered healthy")
	return cmd
}
