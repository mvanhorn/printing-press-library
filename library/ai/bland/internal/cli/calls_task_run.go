// Copyright 2026 Greg Mushen and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/bland/internal/client"
	"github.com/mvanhorn/printing-press-library/library/ai/bland/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/bland/internal/store"
	"github.com/spf13/cobra"
)

type blandCallRecord struct {
	CallID      string          `json:"call_id"`
	Status      string          `json:"status"`
	Task        string          `json:"task"`
	PhoneNumber string          `json:"phone_number,omitempty"`
	Transcript  json.RawMessage `json:"transcripts,omitempty"`
	Summary     string          `json:"summary,omitempty"`
	Variables   json.RawMessage `json:"variables,omitempty"`
	Error       string          `json:"error_message,omitempty"`
}

const blandTaskHistoryResource = "bland_task_calls"

func newNovelCallsTaskRunCmd(flags *rootFlags) *cobra.Command {
	var task, phone string
	var pollEvery time.Duration
	var maxWait time.Duration
	cmd := &cobra.Command{
		Use:         "run",
		Short:       "Place one task-driven call, wait for its final result, and cache the returned call record.",
		Example:     "  bland-pp-cli calls task run --phone-number +12065550123 --task 'Ask whether a table is available tonight' --agent",
		Annotations: map[string]string{"pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "calls task run")
			}
			if flags.dataSource == "local" {
				return usageErr(fmt.Errorf("calls task run has no local data source; omit --data-source local or use --data-source live"))
			}
			if cliutil.IsAnyHarness() {
				return writeHarnessRefusal(cmd.OutOrStdout(), flags, "place an outbound phone call")
			}
			if phone == "" {
				return usageErr(fmt.Errorf("--phone-number is required (E.164 format)"))
			}
			if task == "" {
				return usageErr(fmt.Errorf("--task is required"))
			}
			if !strings.HasPrefix(phone, "+") {
				return usageErr(fmt.Errorf("--phone-number must use E.164 format, such as +12065550123"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			body := map[string]any{"phone_number": phone, "task": task}
			created, _, err := c.Post(cmd.Context(), "/v1/calls", body)
			if err != nil {
				return apiErr(fmt.Errorf("creating task call: %w", err))
			}
			var start struct {
				CallID string `json:"call_id"`
				Status string `json:"status"`
			}
			if err := json.Unmarshal(created, &start); err != nil {
				return apiErr(fmt.Errorf("decoding call creation response: %w", err))
			}
			if start.CallID == "" {
				return apiErr(fmt.Errorf("Bland response did not include call_id"))
			}
			record := blandCallRecord{CallID: start.CallID, Status: start.Status, Task: task, PhoneNumber: phone}
			if record.Status == "" {
				record.Status = "submitted"
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Call started: %s. If monitoring is interrupted, inspect it with 'bland-pp-cli calls get %s'.\n", start.CallID, start.CallID)
			persistCtx, cancelPersist := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancelPersist()
			localStore, openErr := store.OpenWithContext(persistCtx, defaultDBPath("bland-pp-cli"))
			if openErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not open local task history for call %s: %v\n", start.CallID, openErr)
			} else {
				defer localStore.Close()
				if err := persistBlandTaskCall(localStore, record); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not cache started call %s: %v\n", start.CallID, err)
				}
			}
			if pollEvery <= 0 {
				pollEvery = 2 * time.Second
			}
			if maxWait <= 0 {
				maxWait = 10 * time.Minute
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), maxWait)
			defer cancel()
			ticker := time.NewTicker(pollEvery)
			defer ticker.Stop()
			var lastFetchErr error
			for {
				data, fetchErr := c.GetNoCache(ctx, "/v1/calls/"+start.CallID, nil)
				if fetchErr == nil {
					lastFetchErr = nil
					if err := json.Unmarshal(data, &record); err != nil {
						return reportBlandTaskCallFailure(cmd, flags, localStore, record, fmt.Errorf("decoding call details: %w", err))
					}
					if record.CallID == "" {
						record.CallID = start.CallID
					}
					if isTerminalBlandCall(record.Status) {
						break
					}
				} else {
					lastFetchErr = fetchErr
					var apiError *client.APIError
					if errors.As(fetchErr, &apiError) && apiError.StatusCode != 429 && apiError.StatusCode < 500 {
						return reportBlandTaskCallFailure(cmd, flags, localStore, record, fmt.Errorf("fetching call status: %w", fetchErr))
					}
				}
				select {
				case <-ctx.Done():
					if lastFetchErr != nil {
						return reportBlandTaskCallFailure(cmd, flags, localStore, record, fmt.Errorf("fetching call status before timeout: %w", lastFetchErr))
					}
					return reportBlandTaskCallFailure(cmd, flags, localStore, record, fmt.Errorf("call did not finish within %s (last status %q)", maxWait, record.Status))
				case <-ticker.C:
				}
			}
			if record.Task == "" {
				record.Task = task
			}
			if record.PhoneNumber == "" {
				record.PhoneNumber = phone
			}
			if localStore != nil {
				if err := persistBlandTaskCall(localStore, record); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not cache completed call %s: %v\n", start.CallID, err)
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), record, flags)
		},
	}
	cmd.Flags().StringVar(&phone, "phone-number", "", "Destination phone number in E.164 format")
	cmd.Flags().StringVar(&task, "task", "", "Instructions for the AI agent on the call")
	cmd.Flags().DurationVar(&pollEvery, "poll-interval", 2*time.Second, "How often to refresh the call status")
	cmd.Flags().DurationVar(&maxWait, "max-wait", 10*time.Minute, "Maximum time to wait for the completed call")
	return cmd
}

func persistBlandTaskCall(db *store.Store, record blandCallRecord) error {
	raw, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return db.Upsert(blandTaskHistoryResource, record.CallID, raw)
}

func reportBlandTaskCallFailure(cmd *cobra.Command, flags *rootFlags, db *store.Store, record blandCallRecord, cause error) error {
	record.Error = cause.Error()
	if db != nil {
		if err := persistBlandTaskCall(db, record); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not cache interrupted call %s: %v\n", record.CallID, err)
		}
	}
	if flags.asJSON {
		if err := printJSONFilteredKeep(cmd.OutOrStdout(), map[string]any{
			"call_id": record.CallID, "status": record.Status, "error": cause.Error(),
		}, flags, "call_id", "status", "error"); err != nil {
			return err
		}
	}
	return apiErr(fmt.Errorf("call %s was started; inspect it with 'bland-pp-cli calls get %s': %w", record.CallID, record.CallID, cause))
}

func isTerminalBlandCall(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "failed", "error", "no-answer", "busy", "canceled", "cancelled", "voicemail":
		return true
	default:
		return false
	}
}

func init() {
}

func newNovelCallsTaskCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "task",
		Short:       "Run a task-driven call and follow it through to the result.",
		Annotations: map[string]string{"pp:novel-scaffold": "true"},
	}
	cmd.AddCommand(newNovelCallsTaskRunCmd(flags))
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		callsCmd, _, err := root.Find([]string{"calls"})
		if err != nil {
			return
		}
		addNovelCommandIfAbsent(callsCmd, newNovelCallsTaskCmd(flags))
	})
}
