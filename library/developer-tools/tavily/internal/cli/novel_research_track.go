// Copyright 2026 mani. Licensed under Apache-2.0. See LICENSE.
// PATCH: novel research-track command — background polling for async research tasks.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/tavily/internal/store"
)

func newResearchTrackCmd(flags *rootFlags) *cobra.Command {
	var requestID string
	var input string
	var session string
	var pollInterval time.Duration
	var timeout time.Duration
	var listPending bool

	cmd := &cobra.Command{
		Use:   "research-track",
		Short: "Background-poll a research task and persist result in SQLite",
		Long: `Track an async research task by polling for completion and persisting
the final report to the local SQLite store. Survives terminal restarts —
if interrupted, re-run with the same --request-id to continue polling.

Start a research task with 'tavily-pp-cli research run', then pass the
returned request_id to this command.`,
		Example: `  tavily-pp-cli research run --input "AI agent frameworks" --json | jq -r .request_id
  tavily-pp-cli research-track --request-id <id> --session my-research
  tavily-pp-cli research-track --list`,
		Annotations: map[string]string{"pp:novel": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if pollInterval <= 0 {
				pollInterval = 10 * time.Second
			}
			if timeout <= 0 {
				timeout = 10 * time.Minute
			}

			st, err := store.Open()
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer st.Close()

			// List pending tasks
			if listPending {
				tasks, err := st.PendingResearch()
				if err != nil {
					return fmt.Errorf("listing pending tasks: %w", err)
				}
				if len(tasks) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No pending research tasks.")
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%d pending research tasks:\n\n", len(tasks))
				for _, t := range tasks {
					fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s\n    Input: %s\n    Updated: %s\n\n",
						t.Status, t.RequestID, t.Input, t.UpdatedAt.Format("2006-01-02 15:04"))
				}
				return nil
			}

			// Start a new task if --input provided
			if requestID == "" && input != "" {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				body := map[string]any{"input": input}
				data, _, serr := c.Post("/research", body)
				if serr != nil {
					return classifyAPIError(serr, flags)
				}
				var resp struct {
					RequestID string `json:"request_id"`
				}
				if err := json.Unmarshal(data, &resp); err != nil || resp.RequestID == "" {
					return fmt.Errorf("unexpected research response (no request_id)")
				}
				requestID = resp.RequestID
				st.InsertResearch(requestID, input, session)
				fmt.Fprintf(cmd.OutOrStdout(), "Research started: %s\n", requestID)
			}

			if requestID == "" && len(args) > 0 {
				requestID = args[0]
			}
			if requestID == "" {
				return fmt.Errorf("required: --request-id or --input or --list")
			}

			// Ensure task is tracked in store
			existing, err := st.GetResearch(requestID)
			if err != nil {
				return fmt.Errorf("checking store: %w", err)
			}
			if existing == nil {
				if input == "" {
					input = requestID // fallback label
				}
				st.InsertResearch(requestID, input, session)
			} else if existing.Status == "complete" {
				fmt.Fprintf(cmd.OutOrStdout(), "Research %s already complete.\n", requestID)
				if existing.Report != "" && !flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), existing.Report)
				}
				return nil
			}

			c, cerr := flags.newClient()
			if cerr != nil {
				return cerr
			}

			deadline := time.Now().Add(timeout)
			attempt := 0
			for time.Now().Before(deadline) {
				attempt++
				data, gerr := c.Get(fmt.Sprintf("/research/%s", requestID), nil)
				if gerr != nil {
					return classifyAPIError(gerr, flags)
				}

				var resp struct {
					Status  string `json:"status"`
					Report  string `json:"report"`
					Results struct {
						Report string `json:"report"`
					} `json:"results"`
				}
				if err := json.Unmarshal(data, &resp); err != nil {
					return fmt.Errorf("parsing response: %w", err)
				}

				report := resp.Report
				if report == "" {
					report = resp.Results.Report
				}

				if !flags.asJSON {
					fmt.Fprintf(cmd.OutOrStdout(), "  [%d] status: %s\n", attempt, resp.Status)
				}

				if resp.Status == "complete" || resp.Status == "succeeded" {
					st.UpdateResearch(requestID, "complete", report)
					st.InsertCredit("research", 10.0, session)
					if flags.asJSON {
						return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "\nResearch complete!\n\n%s\n", report)
					return nil
				} else if resp.Status == "failed" || resp.Status == "error" {
					st.UpdateResearch(requestID, "failed", "")
					return fmt.Errorf("research task failed: %s", requestID)
				}

				st.UpdateResearch(requestID, resp.Status, "")
				if time.Now().Add(pollInterval).Before(deadline) {
					time.Sleep(pollInterval)
				}
			}
			return fmt.Errorf("research task %s did not complete within %s", requestID, timeout)
		},
	}

	cmd.Flags().StringVar(&requestID, "request-id", "", "Research task request ID to track")
	cmd.Flags().StringVar(&input, "input", "", "Start a new research task with this input")
	cmd.Flags().StringVar(&session, "session", "", "Session label")
	cmd.Flags().DurationVar(&pollInterval, "poll-interval", 10*time.Second, "Polling interval")
	cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Minute, "Maximum wait time")
	cmd.Flags().BoolVar(&listPending, "list", false, "List pending research tasks")
	return cmd
}
