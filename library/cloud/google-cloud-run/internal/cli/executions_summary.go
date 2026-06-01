// Copyright 2026 never-mind-3 and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

type executionSummaryRow struct {
	Name           string `json:"name"`
	Status         string `json:"status"`
	StartedAt      string `json:"started_at"`
	CompletedAt    string `json:"completed_at,omitempty"`
	DurationSec    int    `json:"duration_sec,omitempty"`
	TasksSucceeded int    `json:"tasks_succeeded"`
	TasksFailed    int    `json:"tasks_failed"`
	TasksTotal     int    `json:"tasks_total"`
}

type executionsSummaryView struct {
	Job        string                `json:"job"`
	Executions []executionSummaryRow `json:"executions"`
}

func newNovelExecutionsSummaryCmd(flags *rootFlags) *cobra.Command {
	var flagJob string
	var flagLast int

	cmd := &cobra.Command{
		Use:   "summary",
		Short: "Show a per-execution summary of a Cloud Run Job with succeeded/failed task counts and duration.",
		Long:  "The git log for Cloud Run Jobs. Lists the last N executions of a job with aggregated succeeded/failed task counts and wall-clock duration per execution.",
		Example: strings.Trim(`
  google-cloud-run-pp-cli executions summary --job projects/my-proj/locations/us-central1/jobs/nightly-etl --last 7
  google-cloud-run-pp-cli executions summary --job projects/my-proj/locations/us-central1/jobs/nightly-etl --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagJob == "" && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would summarize executions for job:", flagJob)
				return nil
			}
			if flagJob == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--job is required (format: projects/{project}/locations/{region}/jobs/{job})"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := context.Background()

			// List executions for the job
			execData, err := c.Get(ctx, "/v2/"+flagJob+"/executions", map[string]string{})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var execResp struct {
				Executions []struct {
					Name           string    `json:"name"`
					CreateTime     time.Time `json:"createTime"`
					CompletionTime time.Time `json:"completionTime"`
					Conditions     []struct {
						Type  string `json:"type"`
						State string `json:"state"`
					} `json:"conditions"`
					TaskCount          int `json:"taskCount"`
					CompletedTaskCount int `json:"completedTaskCount"`
					FailedTaskCount    int `json:"failedTaskCount"`
				} `json:"executions"`
			}
			if err := json.Unmarshal(execData, &execResp); err != nil {
				return fmt.Errorf("parsing executions: %w", err)
			}

			execs := execResp.Executions
			if flagLast > 0 && len(execs) > flagLast {
				execs = execs[:flagLast]
			}

			var rows []executionSummaryRow
			for _, e := range execs {
				status := "RUNNING"
				for _, cond := range e.Conditions {
					if cond.Type == "Completed" {
						if cond.State == "CONDITION_SUCCEEDED" {
							status = "SUCCEEDED"
						} else if cond.State == "CONDITION_FAILED" {
							status = "FAILED"
						}
					}
				}
				durationSec := 0
				completedAt := ""
				if !e.CompletionTime.IsZero() {
					completedAt = e.CompletionTime.Format(time.RFC3339)
					if !e.CreateTime.IsZero() {
						durationSec = int(e.CompletionTime.Sub(e.CreateTime).Seconds())
					}
				}
				succeeded := e.CompletedTaskCount - e.FailedTaskCount
				if succeeded < 0 {
					succeeded = 0
				}
				rows = append(rows, executionSummaryRow{
					Name:           shortName(e.Name),
					Status:         status,
					StartedAt:      e.CreateTime.Format(time.RFC3339),
					CompletedAt:    completedAt,
					DurationSec:    durationSec,
					TasksSucceeded: succeeded,
					TasksFailed:    e.FailedTaskCount,
					TasksTotal:     e.TaskCount,
				})
			}

			view := executionsSummaryView{Job: shortName(flagJob), Executions: rows}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "EXECUTION\tSTATUS\tSTARTED\tDURATION(s)\tSUCCEEDED\tFAILED\tTOTAL")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%d\t%d\t%d\n",
					r.Name, r.Status, r.StartedAt, r.DurationSec, r.TasksSucceeded, r.TasksFailed, r.TasksTotal)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&flagJob, "job", "", "Full resource name of the job (projects/{project}/locations/{region}/jobs/{job})")
	cmd.Flags().IntVar(&flagLast, "last", 10, "Number of most recent executions to show")
	return cmd
}
