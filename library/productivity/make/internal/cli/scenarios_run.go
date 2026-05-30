// Copyright 2026 Wade Carpenter and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/make/internal/client"

	"github.com/spf13/cobra"
)

func newScenariosRunCmd(flags *rootFlags) *cobra.Command {
	var flagResponsive bool
	var bodyData string
	var stdinBody bool
	var flagWait bool
	var flagTimeout time.Duration
	var flagPollInterval time.Duration
	var flagReplay string

	cmd := &cobra.Command{
		Use:     "run <scenarioId>",
		Short:   "Trigger one execution of a scenario (use --wait to block until finished)",
		Example: "  make-pp-cli scenarios run 3041366 --wait --timeout 5m --json",
		Annotations: map[string]string{
			"pp:endpoint":   "scenarios.run",
			"pp:method":     "POST",
			"pp:path":       "/scenarios/{scenarioId}/run",
			"mcp:read-only": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			scenarioID := args[0]
			if _, err := strconv.Atoi(scenarioID); err != nil {
				return usageErr(fmt.Errorf("scenarioId must be an integer: %q", scenarioID))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			body, err := buildRunBody(stdinBody, bodyData, flagReplay, c, cmd.Context(), scenarioID)
			if err != nil {
				return err
			}

			runStartedMs := time.Now().UnixMilli()
			postPath := "/scenarios/" + scenarioID + "/run"
			params := map[string]string{}
			if flagResponsive {
				params["responsive"] = "true"
			}
			runResp, status, err := c.PostWithParams(cmd.Context(), postPath, params, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if status < 200 || status >= 300 {
				return apiErr(fmt.Errorf("scenarios run returned HTTP %d: %s", status, truncate(string(runResp), 200)))
			}

			if !flagWait {
				return printOutputWithFlags(cmd.OutOrStdout(), runResp, flags)
			}

			executionID := extractExecutionID(runResp)
			result, err := waitForExecution(cmd.Context(), c, scenarioID, executionID, runStartedMs, flagTimeout, flagPollInterval, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().BoolVar(&flagResponsive, "responsive", false, "Ask Make to return a synchronous response (not all scenario types support this)")
	cmd.Flags().StringVar(&bodyData, "data", "", "JSON input bundle for the scenario")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")
	cmd.Flags().BoolVar(&flagWait, "wait", false, "Block until the execution reaches a terminal status; emit the final result as JSON")
	cmd.Flags().DurationVar(&flagTimeout, "timeout", 5*time.Minute, "Maximum time to wait when --wait is set")
	cmd.Flags().DurationVar(&flagPollInterval, "poll-interval", 2*time.Second, "Polling interval when --wait is set; backs off up to 8s")
	cmd.Flags().StringVar(&flagReplay, "replay", "", "Replay a prior execution: fetches its execution bundle (--replay <executionId>) and reuses it as the input")

	return cmd
}

func buildRunBody(stdinBody bool, bodyData, replayExecID string, c *client.Client, ctx context.Context, scenarioID string) (any, error) {
	if stdinBody {
		raw, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("reading stdin: %w", err)
		}
		var parsed map[string]any
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return nil, usageErr(fmt.Errorf("parsing stdin JSON: %w", err))
		}
		return parsed, nil
	}
	if replayExecID != "" {
		// Fetch prior execution body and reuse as input.
		path := "/scenarios/" + scenarioID + "/executions/" + replayExecID
		raw, err := c.Get(ctx, path, nil)
		if err != nil {
			return nil, fmt.Errorf("fetching replay execution: %w", err)
		}
		var wrap map[string]json.RawMessage
		_ = json.Unmarshal(raw, &wrap)
		if d, ok := wrap["execution"]; ok {
			var inner map[string]json.RawMessage
			_ = json.Unmarshal(d, &inner)
			if bundle, ok := inner["input"]; ok {
				var parsed any
				_ = json.Unmarshal(bundle, &parsed)
				return map[string]any{"data": parsed}, nil
			}
		}
		return map[string]any{"data": json.RawMessage(raw)}, nil
	}
	body := map[string]any{}
	if bodyData != "" {
		var parsed any
		if err := json.Unmarshal([]byte(bodyData), &parsed); err == nil {
			body["data"] = parsed
		} else {
			body["data"] = bodyData
		}
	}
	return body, nil
}

func extractExecutionID(runResp []byte) string {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(runResp, &top); err != nil {
		return ""
	}
	for _, k := range []string{"executionId", "execution_id", "id"} {
		if raw, ok := top[k]; ok {
			var s string
			if json.Unmarshal(raw, &s) == nil && s != "" {
				return s
			}
			var n int64
			if json.Unmarshal(raw, &n) == nil && n != 0 {
				return strconv.FormatInt(n, 10)
			}
		}
	}
	// Some Make responses are wrapped in {"response": {...}}.
	if inner, ok := top["response"]; ok {
		return extractExecutionID(inner)
	}
	return ""
}

// waitForExecution polls /scenarios/{id}/logs from runStartedMs until a terminal
// status appears or timeout elapses. Returns the terminal log entry + a fetched
// execution-detail envelope as a JSON object.
func waitForExecution(ctx context.Context, c *client.Client, scenarioID, executionID string, runStartedMs int64, timeout, initialInterval time.Duration, stderr io.Writer) ([]byte, error) {
	deadline := time.Now().Add(timeout)
	interval := initialInterval
	if interval <= 0 {
		interval = 2 * time.Second
	}
	maxInterval := 8 * time.Second

	for {
		if time.Now().After(deadline) {
			return nil, apiErr(fmt.Errorf("scenarios run --wait: timeout after %s without terminal status (executionId=%s)", timeout, executionID))
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
		interval = interval + interval/2
		if interval > maxInterval {
			interval = maxInterval
		}

		params := map[string]string{
			"from": strconv.FormatInt(runStartedMs-1000, 10),
			"to":   strconv.FormatInt(time.Now().UnixMilli(), 10),
		}
		raw, err := c.Get(ctx, "/scenarios/"+scenarioID+"/logs", params)
		if err != nil {
			// transient — keep polling unless ctx is cancelled
			fmt.Fprintf(stderr, "wait: log poll failed (will retry): %v\n", err)
			continue
		}
		var wrap struct {
			ScenarioLogs []map[string]any `json:"scenarioLogs"`
		}
		if err := json.Unmarshal(raw, &wrap); err != nil {
			continue
		}
		matched := pickLogEntry(wrap.ScenarioLogs, executionID)
		if matched == nil {
			continue
		}
		statusVal, _ := matched["status"].(float64)
		// status: 1 = success, 3 = error, 4 = incomplete
		if int(statusVal) == 1 || int(statusVal) == 3 || int(statusVal) == 4 {
			result := map[string]any{
				"scenarioId":  scenarioID,
				"log":         matched,
				"executionId": pickExecutionFromLog(matched, executionID),
			}
			if execID, ok := result["executionId"].(string); ok && execID != "" {
				if detail, err := c.Get(ctx, "/scenarios/"+scenarioID+"/executions/"+execID, nil); err == nil {
					var parsed any
					_ = json.Unmarshal(detail, &parsed)
					result["execution"] = parsed
				}
			}
			out, err := json.Marshal(result)
			if err != nil {
				return nil, err
			}
			return out, nil
		}
	}
}

func pickLogEntry(logs []map[string]any, executionID string) map[string]any {
	if executionID != "" {
		for _, e := range logs {
			if eid, ok := e["imtId"].(string); ok && eid == executionID {
				return e
			}
			if eid, ok := e["id"].(string); ok && eid == executionID {
				return e
			}
		}
	}
	if len(logs) > 0 {
		return logs[0] // logs come newest-first
	}
	return nil
}

func pickExecutionFromLog(log map[string]any, fallback string) string {
	for _, k := range []string{"imtId", "id", "executionId"} {
		if v, ok := log[k].(string); ok && v != "" {
			return v
		}
	}
	return fallback
}

// Compile-time sentinel that the package's import list isn't pruned by tooling.
var _ = errors.New
