// Copyright 2026 Nimrod Astarhan and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel command — trigger a dbt Cloud job run.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/dbt-cloud/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/dbt-cloud/internal/config"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelTriggerCmd(flags *rootFlags) *cobra.Command {
	var flagCause string
	var flagGitSha string
	var flagGitBranch string
	var flagSchemaOverride string
	var flagWait bool
	var flagInterval time.Duration
	var flagTimeout time.Duration
	var flagAccountID string

	cmd := &cobra.Command{
		Use:   "trigger <job_id>",
		Short: "Trigger a dbt Cloud job and optionally watch the resulting run to completion.",
		Long: `POST /api/v2/accounts/{account_id}/jobs/{job_id}/run/ to trigger a job run.

Prints the new run ID. With --wait, polls the run to completion and exits with
the run's status code (0=success, 1=failure/cancelled). This command MUTATES
remote state — use --dry-run to preview the request without sending it.`,
		Example: `  dbt-cloud-pp-cli trigger 12345 --cause "manual test"
  dbt-cloud-pp-cli trigger 12345 --cause "CI" --wait
  dbt-cloud-pp-cli trigger 12345 --cause "feature" --git-branch main --wait --timeout 30m
  dbt-cloud-pp-cli trigger 12345 --dry-run`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}

			// dry-run / verify intent print
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				jobID := "<job_id>"
				if len(args) > 0 {
					jobID = args[0]
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would trigger job %s (cause: %q)\n", jobID, flagCause)
				return nil
			}

			if len(args) == 0 {
				return usageErr(fmt.Errorf("job_id is required\nUsage: %s", cmd.UseLine()))
			}
			jobID := args[0]

			accountID := config.AccountID(flagAccountID)
			if accountID == "" {
				return usageErr(fmt.Errorf("account_id is required: pass --account-id or set DBT_CLOUD_ACCOUNT_ID"))
			}
			if flagCause == "" {
				return usageErr(fmt.Errorf("--cause is required"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// Build trigger body — only include non-empty optional fields
			body := map[string]any{
				"cause": flagCause,
			}
			if flagGitSha != "" {
				body["git_sha"] = flagGitSha
			}
			if flagGitBranch != "" {
				body["git_branch"] = flagGitBranch
			}
			if flagSchemaOverride != "" {
				body["schema_override"] = flagSchemaOverride
			}

			path := fmt.Sprintf("/api/v2/accounts/%s/jobs/%s/run/", accountID, jobID)
			raw, _, err := c.Post(ctx, path, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Parse the returned run id
			var envelope struct {
				Data struct {
					ID              int64  `json:"id"`
					StatusHumanized string `json:"status_humanized"`
				} `json:"data"`
			}
			if jsonErr := json.Unmarshal(raw, &envelope); jsonErr != nil {
				return fmt.Errorf("parsing trigger response: %w", jsonErr)
			}
			runID := envelope.Data.ID

			if flags.asJSON {
				out := map[string]any{
					"run_id":           runID,
					"status_humanized": envelope.Data.StatusHumanized,
					"job_id":           jobID,
					"account_id":       accountID,
				}
				if err := printJSONFiltered(cmd.OutOrStdout(), out, flags); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Triggered job %s → run %d (%s)\n", jobID, runID, envelope.Data.StatusHumanized)
			}

			if !flagWait {
				return nil
			}

			// Monitor to completion
			fmt.Fprintf(cmd.ErrOrStderr(), "Waiting for run %d to complete...\n", runID)
			opts := monitorPollOptions{
				Interval: flagInterval,
				Timeout:  flagTimeout,
			}
			if cliutil.IsDogfoodEnv() {
				opts.MaxPolls = 2
				if opts.Interval <= 0 {
					opts.Interval = 5 * time.Second
				}
			}

			runIDStr := fmt.Sprintf("%d", runID)
			result, err := pollRunToCompletion(ctx, c, accountID, runIDStr, opts, flags.asJSON)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			run := result.Run
			if result.Timedout {
				fmt.Fprintf(cmd.ErrOrStderr(), "trigger --wait: timed out for run %d (current status: %s)\n", runID, run.StatusHumanized)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), run, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Run %d finished: %s\n", run.ID, run.StatusHumanized)

			return monitorExitError(run)
		},
	}
	cmd.Flags().StringVar(&flagCause, "cause", "", "Reason for triggering the run (required)")
	cmd.Flags().StringVar(&flagGitSha, "git-sha", "", "Git commit SHA to run against")
	cmd.Flags().StringVar(&flagGitBranch, "git-branch", "", "Git branch to run against")
	cmd.Flags().StringVar(&flagSchemaOverride, "schema-override", "", "Override the target schema")
	cmd.Flags().BoolVar(&flagWait, "wait", false, "Wait for the run to complete and exit with its status code")
	cmd.Flags().DurationVar(&flagInterval, "interval", 15*time.Second, "Poll interval when --wait is set (default 15s)")
	cmd.Flags().DurationVar(&flagTimeout, "timeout", 0, "Maximum time to wait when --wait is set (0 = no timeout)")
	cmd.Flags().StringVar(&flagAccountID, "account-id", "", "dbt Cloud account ID (default: DBT_CLOUD_ACCOUNT_ID env var)")
	return cmd
}
