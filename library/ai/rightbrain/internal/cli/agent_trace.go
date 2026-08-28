// Copyright 2026 Farouk Umar and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source auto

// Agent run trace.
//
// The run-events endpoint returns a flat array of conversation events: text
// turns, tool calls, and tool results interleaved, each with a timestamp and
// nothing else. Reading it means holding the call/result pairing in your head
// while scanning timestamps by eye. This command does that pairing, derives the
// elapsed time of each step and of each tool call from the surrounding event
// timestamps, and rolls the result up into a per-tool duration histogram, so
// the step that actually took nine seconds is visible instead of buried.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/rightbrain/internal/store"
)

// agentEvent is the subset of TaskAgentEvent this command consumes. tool_calls
// and tool_results are decoded loosely because their item shape is not pinned
// by the spec.
type agentEvent struct {
	ID           string           `json:"id"`
	Author       string           `json:"author"`
	Content      string           `json:"content"`
	Timestamp    string           `json:"timestamp"`
	EventType    string           `json:"event_type"`
	ToolCalls    []map[string]any `json:"tool_calls"`
	ToolResults  []map[string]any `json:"tool_results"`
	ErrorMessage string           `json:"error_message"`
}

// traceStep is one rendered line of the timeline.
type traceStep struct {
	Index      int    `json:"index"`
	Kind       string `json:"kind"`
	Author     string `json:"author,omitempty"`
	ToolName   string `json:"tool_name,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
	Outcome    string `json:"outcome,omitempty"`
	// PayloadError is set when the result body shows a failure that the
	// outcome flag reported as success.
	PayloadError string  `json:"payload_error,omitempty"`
	Text         string  `json:"text,omitempty"`
	Timestamp    string  `json:"timestamp,omitempty"`
	ElapsedSecs  float64 `json:"elapsed_seconds,omitempty"`
	DurationSecs float64 `json:"duration_seconds,omitempty"`
	Error        string  `json:"error,omitempty"`
}

type toolStat struct {
	ToolName    string  `json:"tool_name"`
	Calls       int     `json:"calls"`
	Errors      int     `json:"errors"`
	TotalSecs   float64 `json:"total_seconds"`
	SlowestSecs float64 `json:"slowest_seconds"`
}

type agentTraceReport struct {
	RunID      string      `json:"run_id"`
	AgentID    string      `json:"agent_id"`
	SessionID  string      `json:"session_id"`
	Steps      []traceStep `json:"steps"`
	ToolStats  []toolStat  `json:"tool_stats"`
	TotalSecs  float64     `json:"total_seconds"`
	EventCount int         `json:"event_count"`
	ToolCalls  int         `json:"tool_call_count"`
	ToolErrors int         `json:"tool_error_count"`
	// SilentToolFailures counts calls Rightbrain reported as successful whose
	// payload showed a failure. A non-zero value here means tool_outcome alone
	// would have under-reported this run.
	SilentToolFailures int    `json:"silent_tool_failures"`
	TerminatedBy       string `json:"terminated_by,omitempty"`
	Note               string `json:"note,omitempty"`
}

// toolErrorCodeRe matches an error code sitting in a JSON code field, e.g.
// {"code":"object_not_found"} or {"error_code": "unauthorized"}.
//
// The anchoring is the point. Matching a bare "_not_found" substring anywhere in
// the payload flags a tool that succeeded and merely returned text containing
// it — a documentation search hitting a page about error handling, a code search
// returning a source file with a page_not_found identifier, a successful query
// listing an API's error codes. That turns this command's failure count into the
// same confident-but-wrong answer it exists to correct, only inverted.
var toolErrorCodeRe = regexp.MustCompile(`"(?:code|error_code)"\s*:\s*"(?:[a-z0-9_]*_not_found|unauthorized)"`)

// toolResultPayloadError reports a tool failure that the tool_outcome flag does
// not, returning a short reason or "" when the payload looks clean.
//
// Rightbrain sets tool_outcome from whether the tool call completed, not from
// whether the tool succeeded. An MCP tool that returns a 404 to the model is a
// completed call, so it is recorded as "success" with the error only present in
// the result body. Counting failures from the flag alone therefore reports zero
// errors on a run where every call failed — which is the exact opposite of what
// someone tracing a broken run needs.
//
// The signals below are deliberately narrow. A result merely containing the word
// "error" is not enough; each pattern is either an explicit tool-error marker or
// a structured HTTP/JSON error envelope, so a legitimate payload that happens to
// discuss errors is not flagged.
func toolResultPayloadError(res map[string]any) string {
	raw, ok := res["tool_result"]
	if !ok {
		raw = res["result"]
	}
	if raw == nil {
		return ""
	}
	var body string
	switch v := raw.(type) {
	case string:
		body = v
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		body = string(encoded)
	}
	if body == "" {
		return ""
	}
	// Cap the scan: tool payloads can be large and the markers are near the top.
	const scanLimit = 4096
	if len(body) > scanLimit {
		body = body[:scanLimit]
	}
	lower := strings.ToLower(body)

	switch {
	case strings.Contains(body, "[Tool Error]"):
		return "payload carries a [Tool Error] marker"
	case strings.Contains(lower, `"object":"error"`), strings.Contains(lower, `"object": "error"`):
		return "payload is a JSON error object"
	case strings.Contains(lower, "apiresponseerror"):
		return "payload carries an APIResponseError"
	case toolErrorCodeRe.MatchString(lower):
		return "payload carries an error code"
	}
	// A structured status field in the 4xx/5xx range.
	for _, marker := range []string{`"status":`, `"status": `, `"status_code":`, `"status_code": `} {
		idx := strings.Index(lower, marker)
		if idx < 0 {
			continue
		}
		// Trim the opening quote of a stringified status: APIs report both
		// {"status": 404} and {"status": "404 Not Found"}, and matching only the
		// bare-number form misses half of them.
		rest := strings.TrimPrefix(strings.TrimSpace(lower[idx+len(marker):]), `"`)
		if len(rest) >= 3 && (rest[0] == '4' || rest[0] == '5') &&
			rest[1] >= '0' && rest[1] <= '9' && rest[2] >= '0' && rest[2] <= '9' {
			return "payload carries HTTP " + rest[:3]
		}
	}
	return ""
}

// firstString returns the first non-empty string value among the given keys.
func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

// buildAgentTrace pairs tool calls with their results and derives per-step and
// per-tool timings. Split out from RunE so it is unit-testable offline.
func buildAgentTrace(runID, agentID, sessionID string, events []agentEvent) agentTraceReport {
	report := agentTraceReport{
		RunID:      runID,
		AgentID:    agentID,
		SessionID:  sessionID,
		Steps:      make([]traceStep, 0, len(events)),
		ToolStats:  make([]toolStat, 0),
		EventCount: len(events),
	}

	// callStartedAt records when each tool_call_id was issued so the matching
	// result can be given a real duration rather than a guess.
	callStartedAt := map[string]time.Time{}
	callToolName := map[string]string{}
	stats := map[string]*toolStat{}

	var firstTS, lastTS time.Time
	var haveFirst bool
	var prevTS time.Time
	var havePrev bool

	idx := 0
	for _, ev := range events {
		ts, tsOK := parseAPITime(ev.Timestamp)
		if tsOK {
			if !haveFirst {
				firstTS, haveFirst = ts, true
			}
			lastTS = ts
		}

		elapsed := 0.0
		if tsOK && havePrev {
			elapsed = ts.Sub(prevTS).Seconds()
		}

		emit := func(step traceStep) {
			step.Index = idx
			idx++
			step.Timestamp = ev.Timestamp
			step.ElapsedSecs = elapsed
			report.Steps = append(report.Steps, step)
			// Only the first emitted step of an event carries the gap from the
			// previous event; siblings within the same event are simultaneous.
			elapsed = 0
		}

		for _, call := range ev.ToolCalls {
			name := firstString(call, "tool_name", "name", "function", "tool")
			id := firstString(call, "tool_call_id", "id", "call_id")
			if tsOK && id != "" {
				callStartedAt[id] = ts
			}
			if id != "" && name != "" {
				callToolName[id] = name
			}
			report.ToolCalls++
			if _, ok := stats[name]; !ok {
				stats[name] = &toolStat{ToolName: name}
			}
			stats[name].Calls++
			emit(traceStep{Kind: "tool_call", ToolName: name, ToolCallID: id, Author: ev.Author})
		}

		for _, res := range ev.ToolResults {
			id := firstString(res, "tool_call_id", "id", "call_id")
			name := firstString(res, "tool_name", "name", "tool")
			if name == "" {
				name = callToolName[id]
			}
			outcome := firstString(res, "tool_outcome", "outcome", "status")
			// tool_outcome reports whether the tool CALL completed, not whether
			// the tool DID anything. A Notion fetch of a missing page comes back
			// as outcome "success" carrying a 404 object_not_found payload, so a
			// count that trusts the flag alone reports zero failures on a run
			// where the tool failed every time. Inspect the payload as well.
			payloadErr := toolResultPayloadError(res)
			isErr := outcome == "error" || outcome == "failed" || payloadErr != ""
			duration := 0.0
			if started, ok := callStartedAt[id]; ok && tsOK {
				duration = ts.Sub(started).Seconds()
			}
			if _, ok := stats[name]; !ok {
				stats[name] = &toolStat{ToolName: name}
			}
			stats[name].TotalSecs += duration
			if duration > stats[name].SlowestSecs {
				stats[name].SlowestSecs = duration
			}
			if isErr {
				stats[name].Errors++
				report.ToolErrors++
				if payloadErr != "" && outcome != "error" && outcome != "failed" {
					report.SilentToolFailures++
				}
			}
			emit(traceStep{
				Kind: "tool_result", ToolName: name, ToolCallID: id,
				Outcome: outcome, DurationSecs: duration, Author: ev.Author,
				PayloadError: payloadErr,
			})
		}

		if txt := strings.TrimSpace(ev.Content); txt != "" {
			emit(traceStep{Kind: "text", Author: ev.Author, Text: txt})
		}
		if ev.ErrorMessage != "" {
			report.TerminatedBy = "error"
			emit(traceStep{Kind: "error", Author: ev.Author, Error: ev.ErrorMessage})
		}
		if ev.EventType != "" && len(ev.ToolCalls) == 0 && len(ev.ToolResults) == 0 &&
			strings.TrimSpace(ev.Content) == "" && ev.ErrorMessage == "" {
			emit(traceStep{Kind: ev.EventType, Author: ev.Author})
		}

		if tsOK {
			prevTS, havePrev = ts, true
		}
	}

	if haveFirst {
		report.TotalSecs = lastTS.Sub(firstTS).Seconds()
	}
	for _, st := range stats {
		report.ToolStats = append(report.ToolStats, *st)
	}
	// Slowest tool first — that is the question this histogram exists to answer.
	sort.SliceStable(report.ToolStats, func(i, j int) bool {
		return report.ToolStats[i].TotalSecs > report.ToolStats[j].TotalSecs
	})
	if len(events) == 0 {
		report.Note = "the API returned no events for this run; it may have been created moments ago, or the run id may belong to a different agent"
	}
	return report
}

// lookupAgentIDForRun finds the owning agent for an agent-run id in the local
// mirror, so the caller does not have to supply --agent-id for a run they
// already synced.
func lookupAgentIDForRun(ctx context.Context, dbPath, runID string) string {
	if _, err := os.Stat(dbPath); err != nil {
		return ""
	}
	db, err := store.OpenReadOnlyContext(ctx, dbPath)
	if err != nil {
		return ""
	}
	defer db.Close()

	var agentID sql.NullString
	err = db.DB().QueryRowContext(ctx, `
		SELECT json_extract(data, '$.task_agent_id')
		FROM resources
		WHERE resource_type = 'project_task_agent_run' AND id = ?
		LIMIT 1`, runID).Scan(&agentID)
	if err != nil {
		return ""
	}
	return agentID.String
}

func newNovelAgentTraceCmd(flags *rootFlags) *cobra.Command {
	var flagTools bool
	var flagAgentID string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "agent-trace <run_id>",
		Short: "Reconstruct one agent run as a timeline of paired tool calls and results with per-step elapsed time",
		Long: "Reconstruct a single agent run from its event stream.\n\n" +
			"Pairs each tool call with its result, derives how long each step and each tool " +
			"call took, and summarizes the run as a per-tool duration histogram.\n\n" +
			"The owning agent is resolved from the local mirror when the run has been synced; " +
			"pass --agent-id when it has not.\n\n" +
			"Use this command to reconstruct one agent run's event timeline by run id. " +
			"Do NOT use it to find which runs are currently parked awaiting approval; use 'approvals' instead.",
		Example: "  rightbrain-pp-cli agent-trace 0195d207-32bb-d03d-cfdc-f4516e9222c8 --tools --agent",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch and reconstruct the agent run event timeline")
				return nil
			}
			if len(args) < 1 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<run_id> is required"))
			}
			runID := args[0]

			orgID, projectID, err := requireScope(flags)
			if err != nil {
				return err
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			agentID := flagAgentID
			if agentID == "" {
				if dbPath == "" {
					dbPath = defaultDBPath("rightbrain-pp-cli")
				}
				agentID = lookupAgentIDForRun(ctx, dbPath, runID)
			}
			if agentID == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf(
					"could not resolve the agent for run %s\nPass --agent-id <id>, or sync agent runs first:\n  rightbrain-pp-cli sync --resources project_task_agent_run",
					runID))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := fmt.Sprintf("/org/%s/project/%s/task-agent/%s/run/%s/events",
				orgID, projectID, agentID, runID)
			data, err := c.Get(ctx, path, map[string]string{})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var envelope struct {
				RunID     string       `json:"run_id"`
				SessionID string       `json:"session_id"`
				Events    []agentEvent `json:"events"`
			}
			if err := json.Unmarshal(data, &envelope); err != nil {
				return fmt.Errorf("parsing run events: %w", err)
			}
			if envelope.RunID == "" {
				envelope.RunID = runID
			}

			report := buildAgentTrace(envelope.RunID, agentID, envelope.SessionID, envelope.Events)

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				out := cmd.OutOrStdout()
				if report.Note != "" {
					fmt.Fprintln(out, report.Note)
					return nil
				}
				fmt.Fprintf(out, "run %s  session %s  %.1fs  %d events  %d tool calls (%d errors)\n\n",
					report.RunID, orDash(report.SessionID), report.TotalSecs,
					report.EventCount, report.ToolCalls, report.ToolErrors)
				for _, s := range report.Steps {
					switch s.Kind {
					case "tool_call":
						fmt.Fprintf(out, "  +%6.2fs  -> %s\n", s.ElapsedSecs, orDash(s.ToolName))
					case "tool_result":
						marker := "<-"
						if s.Outcome == "error" || s.Outcome == "failed" {
							marker = "<! "
						}
						fmt.Fprintf(out, "  %8s %s %s (%.2fs)\n", "", marker, orDash(s.ToolName), s.DurationSecs)
					case "error":
						fmt.Fprintf(out, "  +%6.2fs  !! %s\n", s.ElapsedSecs, s.Error)
					case "text":
						fmt.Fprintf(out, "  +%6.2fs  %s: %s\n", s.ElapsedSecs, orDash(s.Author), truncateForTrace(s.Text))
					default:
						fmt.Fprintf(out, "  +%6.2fs  [%s]\n", s.ElapsedSecs, s.Kind)
					}
				}
				if flagTools && len(report.ToolStats) > 0 {
					fmt.Fprintln(out, "\nTOOL\tCALLS\tERRORS\tTOTAL\tSLOWEST")
					for _, st := range report.ToolStats {
						fmt.Fprintf(out, "%s\t%d\t%d\t%.2fs\t%.2fs\n",
							orDash(st.ToolName), st.Calls, st.Errors, st.TotalSecs, st.SlowestSecs)
					}
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}

	cmd.Flags().BoolVar(&flagTools, "tools", false,
		"Also print the per-tool call count and duration histogram")
	cmd.Flags().StringVar(&flagAgentID, "agent-id", "",
		"Owning task agent ID (resolved from the local mirror when omitted)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path")
	return cmd
}

// truncateForTrace keeps a text turn to one readable line.
func truncateForTrace(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	const max = 120
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
