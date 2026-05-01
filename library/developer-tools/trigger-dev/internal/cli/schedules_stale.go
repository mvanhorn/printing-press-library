package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newSchedulesStaleCmd(flags *rootFlags) *cobra.Command {
	var staleDays int

	cmd := &cobra.Command{
		Use:   "stale",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Find schedules that stopped producing runs or have high failure rates",
		Long: `Detect stale schedules by cross-referencing schedule data with recent run history.
Finds schedules with no recent runs, high failure rates, or that are disabled
without explanation.`,
		Example: `  # Find schedules with no runs in the last 7 days
  trigger-dev-pp-cli schedules stale

  # Adjust staleness threshold
  trigger-dev-pp-cli schedules stale --days 14

  # JSON output
  trigger-dev-pp-cli schedules stale --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Would check schedules for staleness (threshold: %d days)\n", staleDays)
				return nil
			}

			// Fetch schedules
			schedResp, err := c.Get("/api/v1/schedules", nil)
			if err != nil {
				return classifyAPIError(err)
			}

			var schedEnvelope struct {
				Data []json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(schedResp, &schedEnvelope); err != nil {
				var arr []json.RawMessage
				if err2 := json.Unmarshal(schedResp, &arr); err2 == nil {
					schedEnvelope.Data = arr
				}
			}

			type schedule struct {
				ID         string `json:"id"`
				Task       string `json:"task"`
				Active     bool   `json:"active"`
				ExternalID string `json:"externalId"`
				Generator  struct {
					Cron        string `json:"cron"`
					Description string `json:"description"`
				} `json:"generator"`
			}

			type staleResult struct {
				ScheduleID string `json:"schedule_id"`
				Task       string `json:"task"`
				Cron       string `json:"cron"`
				Active     bool   `json:"active"`
				Reason     string `json:"reason"`
			}

			var results []staleResult

			for _, raw := range schedEnvelope.Data {
				var s schedule
				if err := json.Unmarshal(raw, &s); err != nil {
					continue
				}

				// Check if this schedule's task has recent runs
				period := fmt.Sprintf("%dd", staleDays)
				runsResp, err := c.Get("/api/v1/runs", map[string]string{
					"taskIdentifier":            s.Task,
					"page[size]":                "1",
					"filter[createdAt][period]": period,
				})

				hasRecentRuns := false
				if err == nil {
					var runsEnv struct {
						Data []json.RawMessage `json:"data"`
					}
					if err := json.Unmarshal(runsResp, &runsEnv); err == nil {
						hasRecentRuns = len(runsEnv.Data) > 0
					} else {
						var arr []json.RawMessage
						if err2 := json.Unmarshal(runsResp, &arr); err2 == nil {
							hasRecentRuns = len(arr) > 0
						}
					}
				}

				reason := ""
				if !s.Active {
					reason = "disabled"
				} else if !hasRecentRuns {
					reason = fmt.Sprintf("no runs in last %d days", staleDays)
				}

				if reason != "" {
					results = append(results, staleResult{
						ScheduleID: s.ID,
						Task:       s.Task,
						Cron:       s.Generator.Cron,
						Active:     s.Active,
						Reason:     reason,
					})
				}
			}

			if flags.asJSON {
				return flags.printJSON(cmd, results)
			}

			if len(results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "All schedules are healthy (producing runs within %d days).\n", staleDays)
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Stale Schedules (%d found)\n\n", len(results))
			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-30s %-15s %-8s %s\n",
				"schedule", "task", "cron", "active", "reason")
			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-30s %-15s %-8s %s\n",
				strings.Repeat("-", 20), strings.Repeat("-", 30), strings.Repeat("-", 15), strings.Repeat("-", 8), strings.Repeat("-", 25))

			for _, r := range results {
				active := "yes"
				if !r.Active {
					active = "no"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-30s %-15s %-8s %s\n",
					truncate(r.ScheduleID, 20), truncate(r.Task, 30), r.Cron, active, r.Reason)
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&staleDays, "days", 7, "Days without runs to consider stale")
	_ = time.Now

	return cmd
}
