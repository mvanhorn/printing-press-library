// Copyright 2026 Nimrod Astarhan and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel command — monitor a dbt Cloud run to completion.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/dbt-cloud/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/dbt-cloud/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/dbt-cloud/internal/config"

	"github.com/spf13/cobra"
)

// dbtRunStatus maps the integer status codes returned by the dbt Cloud API.
const (
	dbtStatusQueued    = 1
	dbtStatusStarting  = 2
	dbtStatusRunning   = 3
	dbtStatusSuccess   = 10
	dbtStatusError     = 20
	dbtStatusCancelled = 30
)

// dbtRunData is the subset of fields from /runs/{id}/?include_related=["run_steps"]
// that monitor cares about.
type dbtRunData struct {
	ID              int64        `json:"id"`
	Status          int          `json:"status"`
	StatusHumanized string       `json:"status_humanized"`
	IsComplete      bool         `json:"is_complete"`
	IsError         bool         `json:"is_error"`
	IsCancelled     bool         `json:"is_cancelled"`
	IsSuccess       bool         `json:"is_success"`
	FinishedAt      string       `json:"finished_at"`
	Duration        string       `json:"duration"`
	JobDefinitionID int64        `json:"job_definition_id"`
	GitBranch       string       `json:"git_branch"`
	GitSha          string       `json:"git_sha"`
	CreatedAt       string       `json:"created_at"`
	RunSteps        []dbtRunStep `json:"run_steps"`
}

type dbtRunStep struct {
	Index           int    `json:"index"`
	Name            string `json:"name"`
	Status          int    `json:"status"`
	StatusHumanized string `json:"status_humanized"`
	Logs            string `json:"logs"`
}

// fetchRun fetches run details from the dbt Cloud API.
func fetchRun(ctx context.Context, c *client.Client, accountID, runID string) (*dbtRunData, error) {
	path := fmt.Sprintf("/api/v2/accounts/%s/runs/%s/", accountID, runID)
	raw, err := c.GetNoCache(ctx, path, map[string]string{
		"include_related": `["run_steps"]`,
	})
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("parsing run response: %w", err)
	}
	var run dbtRunData
	if err := json.Unmarshal(envelope.Data, &run); err != nil {
		return nil, fmt.Errorf("parsing run data: %w", err)
	}
	return &run, nil
}

// isTerminalStatus reports whether the run has reached a terminal state.
func isTerminalStatus(run *dbtRunData) bool {
	if run.IsComplete {
		return true
	}
	switch run.Status {
	case dbtStatusSuccess, dbtStatusError, dbtStatusCancelled:
		return true
	}
	return false
}

// monitorPollOptions controls the polling behavior.
type monitorPollOptions struct {
	Interval time.Duration
	Timeout  time.Duration
	// MaxPolls, if > 0, stops polling after this many attempts (used in dogfood mode).
	MaxPolls int
}

// MonitorResult is the outcome of a monitoring session.
type MonitorResult struct {
	Run      *dbtRunData
	Timedout bool
}

// pollRunToCompletion polls run until terminal. It emits human-readable status
// lines to stderr (or JSON lines if asJSON). Returns the final run state.
func pollRunToCompletion(ctx context.Context, c *client.Client, accountID, runID string, opts monitorPollOptions, asJSON bool) (*MonitorResult, error) {
	interval := opts.Interval
	if interval <= 0 {
		interval = 15 * time.Second
	}

	var cancel context.CancelFunc
	if opts.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	polls := 0
	for {
		polls++
		run, err := fetchRun(ctx, c, accountID, runID)
		if err != nil {
			return nil, fmt.Errorf("fetching run %s: %w", runID, err)
		}

		if asJSON {
			ev := map[string]any{
				"event":            "poll",
				"run_id":           run.ID,
				"status":           run.Status,
				"status_humanized": run.StatusHumanized,
				"is_complete":      run.IsComplete,
			}
			b, _ := json.Marshal(ev)
			fmt.Fprintf(os.Stderr, "%s\n", b)
		} else {
			ts := time.Now().Format("15:04:05")
			fmt.Fprintf(os.Stderr, "[%s] Run %s: %s\n", ts, runID, run.StatusHumanized)
		}

		if isTerminalStatus(run) {
			return &MonitorResult{Run: run}, nil
		}

		if opts.MaxPolls > 0 && polls >= opts.MaxPolls {
			return &MonitorResult{Run: run, Timedout: true}, nil
		}

		select {
		case <-ctx.Done():
			// Fetch one last time to get terminal state if it just finished.
			// Bound it with a short deadline so a slow or unavailable API
			// cannot hang the CLI indefinitely past the monitoring timeout.
			finalCtx, finalCancel := context.WithTimeout(context.Background(), 15*time.Second)
			final, ferr := fetchRun(finalCtx, c, accountID, runID)
			finalCancel()
			if ferr == nil {
				return &MonitorResult{Run: final, Timedout: !isTerminalStatus(final)}, nil
			}
			return nil, fmt.Errorf("monitoring timed out for run %s", runID)
		case <-time.After(interval):
		}
	}
}

// monitorExitError returns a non-nil error (exit 1) when the run failed or was cancelled.
func monitorExitError(run *dbtRunData) error {
	if run == nil {
		return fmt.Errorf("run did not reach a terminal state")
	}
	if run.IsError || run.Status == dbtStatusError {
		failedSteps := findFailedSteps(run)
		msg := fmt.Sprintf("run %d failed (status: %s)", run.ID, run.StatusHumanized)
		if len(failedSteps) > 0 {
			msg += "\nFailed steps:"
			for _, s := range failedSteps {
				msg += fmt.Sprintf("\n  - %s", s.Name)
				if s.Logs != "" {
					msg += "\n" + lastNLines(s.Logs, 10)
				}
			}
		}
		return fmt.Errorf("%s", msg)
	}
	if run.IsCancelled || run.Status == dbtStatusCancelled {
		return fmt.Errorf("run %d was cancelled", run.ID)
	}
	return nil
}

// findFailedSteps returns run steps whose status indicates failure.
func findFailedSteps(run *dbtRunData) []dbtRunStep {
	var failed []dbtRunStep
	for _, s := range run.RunSteps {
		if s.Status == dbtStatusError || s.StatusHumanized == "Error" {
			failed = append(failed, s)
		}
	}
	return failed
}

// lastNLines returns the last n lines of s.
func lastNLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	// Indent each line
	for i, l := range lines {
		lines[i] = "    " + l
	}
	return strings.Join(lines, "\n")
}

// pp:data-source live
func newNovelMonitorCmd(flags *rootFlags) *cobra.Command {
	var flagInterval time.Duration
	var flagTimeout time.Duration
	var flagAccountID string

	cmd := &cobra.Command{
		Use:   "monitor <run_id>",
		Short: "Watch a dbt Cloud run until it finishes, with live status and an exit code that reflects success or failure.",
		Long: `Poll a dbt Cloud run until it reaches a terminal state (Success, Error, or Cancelled).

Exits 0 on success, 1 on failure or cancellation. Prints live status to stderr
every --interval seconds (default 15s). Uses --account-id flag or DBT_CLOUD_ACCOUNT_ID env var.`,
		Example:     "  dbt-cloud-pp-cli monitor 12345678\n  dbt-cloud-pp-cli monitor 12345678 --interval 30s --timeout 30m",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,1"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				if len(args) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "would monitor run %s\n", args[0])
				}
				return nil
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("run_id is required\nUsage: %s", cmd.UseLine()))
			}
			runID := args[0]

			accountID := config.AccountID(flagAccountID)
			if accountID == "" {
				return usageErr(fmt.Errorf("account_id is required: pass --account-id or set DBT_CLOUD_ACCOUNT_ID"))
			}

			// Verify env: print intent, no network
			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would monitor run %s on account %s\n", runID, accountID)
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			opts := monitorPollOptions{
				Interval: flagInterval,
				Timeout:  flagTimeout,
			}
			// Dogfood: poll at most 2 times so the 30s budget isn't consumed
			if cliutil.IsDogfoodEnv() {
				opts.MaxPolls = 2
				if opts.Interval <= 0 {
					opts.Interval = 5 * time.Second
				}
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			result, err := pollRunToCompletion(ctx, c, accountID, runID, opts, flags.asJSON)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			run := result.Run
			if result.Timedout {
				fmt.Fprintf(os.Stderr, "monitor: timed out waiting for run %s (current status: %s)\n", runID, run.StatusHumanized)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), run, flags)
			}

			// Human output summary
			fmt.Fprintf(cmd.OutOrStdout(), "Run %d finished: %s\n", run.ID, run.StatusHumanized)
			if run.Duration != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Duration: %s\n", run.Duration)
			}

			exitErr := monitorExitError(run)
			if exitErr != nil {
				return exitErr
			}
			return nil
		},
	}
	cmd.Flags().DurationVar(&flagInterval, "interval", 15*time.Second, "Poll interval (default 15s)")
	cmd.Flags().DurationVar(&flagTimeout, "timeout", 0, "Maximum time to wait (0 = no timeout)")
	cmd.Flags().StringVar(&flagAccountID, "account-id", "", "dbt Cloud account ID (default: DBT_CLOUD_ACCOUNT_ID env var)")
	return cmd
}

// resolveAccountIDArg helps commands that take account_id as a positional arg
// fall back to the env var when no arg is supplied.
// Usage: accountID, args, err := resolveAccountIDArg(args, flagAccountID)
func resolveAccountIDArg(args []string, flagVal string) (accountID string, remaining []string, err error) {
	// Explicit flag wins
	if flagVal != "" {
		return flagVal, args, nil
	}
	// First arg might be the account_id
	if len(args) > 0 {
		// Check if it looks like a numeric ID
		if _, convErr := strconv.ParseInt(args[0], 10, 64); convErr == nil {
			return args[0], args[1:], nil
		}
	}
	// Fall back to env
	if v := config.AccountID(""); v != "" {
		return v, args, nil
	}
	return "", args, fmt.Errorf("account_id is required: pass --account-id or set DBT_CLOUD_ACCOUNT_ID")
}
