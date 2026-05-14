// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type waitResult struct {
	ExecutionID string `json:"execution_id"`
	Status      string `json:"status"`
	Finished    bool   `json:"finished"`
	WaitedSec   int    `json:"waited_sec"`
	TimedOut    bool   `json:"timed_out,omitempty"`
}

func newExecutionsWaitCmd(flags *rootFlags) *cobra.Command {
	var timeout int
	var pollInterval int

	cmd := &cobra.Command{
		Use:   "wait <execution-id>",
		Short: "Poll an execution until it reaches a terminal state (CI gate)",
		Long: `Poll a running execution until it finishes (success, error, or canceled).
Exits 0 on success, 1 on error/canceled, 2 on timeout. Designed for CI/CD
pipelines that need to block on an n8n workflow result.`,
		Example: strings.Trim(`
  # Wait up to 5 minutes for execution 42 to finish
  n8n-pp-cli executions wait 42

  # Custom timeout and poll interval
  n8n-pp-cli executions wait 42 --timeout 120 --interval 5

  # JSON output for CI scripts
  n8n-pp-cli executions wait 42 --json --agent`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			execID := args[0]

			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), `{"dry_run":true,"execution_id":%q}`+"\n", execID)
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			start := time.Now()
			deadline := time.Duration(timeout) * time.Second
			poll := time.Duration(pollInterval) * time.Second

			for {
				data, err := c.Get("/executions/"+execID, nil)
				if err != nil {
					return classifyAPIError(err, flags)
				}

				var exec struct {
					ID       string `json:"id"`
					Status   string `json:"status"`
					Finished bool   `json:"finished"`
				}
				if err := json.Unmarshal(data, &exec); err != nil {
					return fmt.Errorf("parsing execution response: %w", err)
				}

				elapsed := int(time.Since(start).Seconds())
				isTerminal := exec.Finished ||
					exec.Status == "success" ||
					exec.Status == "error" ||
					exec.Status == "canceled" ||
					exec.Status == "crashed"

				if isTerminal {
					result := waitResult{
						ExecutionID: execID,
						Status:      exec.Status,
						Finished:    exec.Finished,
						WaitedSec:   elapsed,
					}
					if err := printJSONFiltered(cmd.OutOrStdout(), result, flags); err != nil {
						return err
					}
					if exec.Status == "success" {
						return nil
					}
					return &cliError{code: 1, err: fmt.Errorf("execution %s ended with status: %s", execID, exec.Status)}
				}

				timedOut := deadline > 0 && time.Since(start) >= deadline
				if timedOut {
					result := waitResult{
						ExecutionID: execID,
						Status:      exec.Status,
						Finished:    false,
						WaitedSec:   elapsed,
						TimedOut:    true,
					}
					if err := printJSONFiltered(cmd.OutOrStdout(), result, flags); err != nil {
						return err
					}
					return &cliError{code: 2, err: fmt.Errorf("timed out waiting for execution %s after %ds", execID, elapsed)}
				}

				if !flags.quiet && !flags.asJSON {
					fmt.Fprintf(cmd.ErrOrStderr(), "waiting... status=%s elapsed=%ds\n", exec.Status, elapsed)
				}
				time.Sleep(poll)
			}
		},
	}
	cmd.Flags().IntVar(&timeout, "timeout", 300, "Maximum seconds to wait (0 = wait forever)")
	cmd.Flags().IntVar(&pollInterval, "interval", 3, "Seconds between status polls")
	return cmd
}
