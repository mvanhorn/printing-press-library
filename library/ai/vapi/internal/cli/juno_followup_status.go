// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// Private Juno follow-up and status workflows.

package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/vapi/internal/client"
	"github.com/spf13/cobra"
)

func newJunoFollowupCmd(flags *rootFlags) *cobra.Command {
	var common junoCommonOptions
	workflow := junoWorkflowOptions{TaskType: "followup"}
	var callID string
	var leaveMessage bool
	cmd := &cobra.Command{
		Use:   "followup",
		Short: "Re-call or leave a message using a previous Juno call context",
		Example: `  vapi-pp-cli juno followup --call-id call_123 --note "They asked us to call back after 2pm" --dry-run --agent

  vapi-pp-cli juno followup --assistant-id asst_123 --phone-number-id pn_123 --to +15551234567 \
    --business "Front desk" --goal "Follow up on the reservation request" --leave-message --dry-run --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if leaveMessage {
				workflow.Notes = append(workflow.Notes, "leave a concise voicemail or message if no human answers")
				workflow.SuccessCriteria = append(workflow.SuccessCriteria, "Confirm whether a human answered or a message was left.")
			}
			if callID == "" {
				if workflow.Goal == "" {
					return fmt.Errorf("--goal is required when --call-id is not provided")
				}
				return runJunoWorkflow(cmd, flags, common, workflow)
			}
			if !flags.dryRun && !flags.yes {
				return fmt.Errorf("refusing to place a live Juno follow-up call without --yes; rerun with --dry-run to preview")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.DryRun = false
			original, err := fetchJunoCall(cmd, c, callID)
			if err != nil {
				return err
			}
			body, err := buildJunoFollowupBodyFromCall(original, callID, common, workflow)
			if err != nil {
				return err
			}
			return executeJunoCall(cmd, flags, body, "followup")
		},
	}
	addJunoCommonFlags(cmd, &common)
	addJunoWorkflowFlags(cmd, &workflow)
	cmd.Flags().StringVar(&callID, "call-id", "", "Previous Vapi call ID to follow up on")
	cmd.Flags().BoolVar(&leaveMessage, "leave-message", false, "Tell Juno to leave a concise message if no human answers")
	return cmd
}

func buildJunoFollowupBodyFromCall(original map[string]any, callID string, common junoCommonOptions, workflow junoWorkflowOptions) (map[string]any, error) {
	body := map[string]any{}
	copyFirstPresent(body, original, "assistantId", "assistant")
	copyFirstPresent(body, original, "phoneNumberId", "phoneNumber")
	copyFirstPresent(body, original, "customer", "customers")
	if _, ok := body["assistantId"]; !ok {
		if _, ok := body["assistant"]; !ok {
			return nil, fmt.Errorf("previous call %s does not include assistant details; provide the same task flags instead", callID)
		}
	}
	if _, ok := body["phoneNumberId"]; !ok {
		if _, ok := body["phoneNumber"]; !ok {
			return nil, fmt.Errorf("previous call %s does not include phone number details; provide the same task flags instead", callID)
		}
	}
	if _, ok := body["customer"]; !ok {
		if _, ok := body["customers"]; !ok {
			return nil, fmt.Errorf("previous call %s does not include destination details; provide the same task flags instead", callID)
		}
	}
	if common.CallName != "" {
		body["name"] = common.CallName
	} else {
		body["name"] = "juno-followup"
	}
	overrides := mapFromAny(original["assistantOverrides"])
	variables := mapFromAny(overrides["variableValues"])
	if variables == nil {
		variables = map[string]any{}
	}
	if workflow.Goal == "" {
		if priorGoal, ok := variables["juno_goal"].(string); ok && strings.TrimSpace(priorGoal) != "" {
			workflow.Goal = "Follow up on: " + priorGoal
		} else {
			workflow.Goal = "Follow up on the previous call"
		}
	}
	variables["juno_task_type"] = "followup"
	variables["juno_previous_call_id"] = callID
	variables["juno_goal"] = workflow.Goal
	variables["juno_followup_notes"] = nonNilStrings(workflow.Notes)
	variables["juno_script"] = buildJunoScript(workflow)
	if len(workflow.SuccessCriteria) > 0 {
		variables["juno_success_criteria"] = nonNilStrings(workflow.SuccessCriteria)
	}
	overrides["variableValues"] = variables
	body["assistantOverrides"] = overrides
	applyDialRecordingDefault(body, common.NoRecord)
	return body, nil
}

func newJunoStatusCmd(flags *rootFlags) *cobra.Command {
	var watch bool
	var downloadPath string
	var includeTranscript bool
	var pollInterval time.Duration
	var watchTimeout time.Duration
	cmd := &cobra.Command{
		Use:   "status <call-id>",
		Short: "Show Juno call outcome, transcript, cost, and recording",
		Example: `  vapi-pp-cli juno status call_123 --watch --download --transcript --agent
  vapi-pp-cli juno status call_123 --download ./call.wav --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			callID := args[0]
			call, err := waitForJunoCallStatus(cmd, c, callID, watch, pollInterval, watchTimeout)
			if err != nil {
				return err
			}
			out := summarizeJunoCall(callID, call, includeTranscript)
			if cmd.Flags().Changed("download") {
				saved, err := downloadJunoRecording(cmd, c, callID, downloadPath)
				if err != nil {
					return err
				}
				out["recording_saved"] = saved
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().BoolVar(&watch, "watch", false, "Poll until the call reaches a terminal status")
	cmd.Flags().StringVar(&downloadPath, "download", "", "Download the stereo recording to an optional path")
	if flag := cmd.Flags().Lookup("download"); flag != nil {
		flag.NoOptDefVal = "."
	}
	cmd.Flags().BoolVar(&includeTranscript, "transcript", false, "Include full transcript text in output")
	cmd.Flags().DurationVar(&pollInterval, "poll-interval", 5*time.Second, "Polling interval for --watch")
	cmd.Flags().DurationVar(&watchTimeout, "watch-timeout", 10*time.Minute, "Maximum time to wait with --watch")
	return cmd
}

type junoClient interface {
	GetNoCache(ctx context.Context, path string, params map[string]string) (json.RawMessage, error)
	GetWithHeadersNoCache(ctx context.Context, path string, params map[string]string, headers map[string]string) (json.RawMessage, error)
}

func fetchJunoCall(cmd *cobra.Command, c junoClient, callID string) (map[string]any, error) {
	path := replacePathParam("/call/{id}", "id", callID)
	data, err := c.GetNoCache(cmd.Context(), path, map[string]string{})
	if err != nil {
		return nil, classifyAPIError(err, nil)
	}
	var call map[string]any
	if err := json.Unmarshal(data, &call); err != nil {
		return nil, fmt.Errorf("parsing call %s response: %w", callID, err)
	}
	return call, nil
}

func waitForJunoCallStatus(cmd *cobra.Command, c junoClient, callID string, watch bool, interval, timeout time.Duration) (map[string]any, error) {
	deadline := time.Now().Add(timeout)
	for {
		call, err := fetchJunoCall(cmd, c, callID)
		if err != nil {
			return nil, err
		}
		if !watch || isJunoTerminalStatus(stringValue(call["status"])) {
			return call, nil
		}
		if time.Now().After(deadline) {
			return call, fmt.Errorf("timed out waiting for call %s to end; last status: %s", callID, stringValue(call["status"]))
		}
		select {
		case <-cmd.Context().Done():
			return call, cmd.Context().Err()
		case <-time.After(interval):
		}
	}
}

func summarizeJunoCall(callID string, call map[string]any, includeTranscript bool) map[string]any {
	out := map[string]any{
		"id":     firstNonEmpty(stringValue(call["id"]), callID),
		"status": stringValue(call["status"]),
		"ended":  isJunoTerminalStatus(stringValue(call["status"])),
	}
	copySummaryField(out, call, "summary", "summary", "analysis.summary", "artifact.summary")
	copySummaryField(out, call, "outcome", "outcome", "analysis.successEvaluation", "analysis.structuredData.outcome")
	copySummaryField(out, call, "duration_seconds", "durationSeconds", "duration", "artifact.durationSeconds")
	copySummaryField(out, call, "cost", "cost", "costs.total")
	copySummaryField(out, call, "started_at", "startedAt")
	copySummaryField(out, call, "ended_at", "endedAt")
	if includeTranscript {
		copySummaryField(out, call, "transcript", "transcript", "artifact.transcript")
	} else if transcript := firstPathValue(call, "transcript", "artifact.transcript"); transcript != nil {
		out["has_transcript"] = true
	}
	recordings := map[string]any{}
	copySummaryField(recordings, call, "recording_url", "artifact.recordingUrl", "artifact.recording")
	copySummaryField(recordings, call, "stereo_recording_url", "artifact.stereoRecordingUrl")
	copySummaryField(recordings, call, "mono_recording_url", "artifact.monoRecordingUrl")
	if len(recordings) > 0 {
		out["recordings"] = recordings
	}
	return out
}

func downloadJunoRecording(cmd *cobra.Command, c junoClient, callID, requestedPath string) (map[string]any, error) {
	headers := map[string]string{client.BinaryResponseHeader: "true"}
	var data json.RawMessage
	var err error
	kind := "stereo"
	for _, candidate := range []struct {
		kind string
		path string
	}{
		{"stereo", "/call/{id}/stereo-recording"},
		{"mono", "/call/{id}/mono-recording"},
	} {
		path := replacePathParam(candidate.path, "id", callID)
		data, err = c.GetWithHeadersNoCache(cmd.Context(), path, map[string]string{}, headers)
		if err == nil {
			kind = candidate.kind
			break
		}
	}
	if err != nil {
		return nil, classifyAPIError(err, nil)
	}
	payload, contentType, err := decodePPBinary(data)
	if err != nil {
		return nil, err
	}
	target := resolveRecordingPath(callID, kind, requestedPath, contentType)
	if err := os.WriteFile(target, payload, 0o600); err != nil {
		return nil, fmt.Errorf("writing recording: %w", err)
	}
	abs, _ := filepath.Abs(target)
	return map[string]any{
		"path":         abs,
		"bytes":        len(payload),
		"content_type": contentType,
		"kind":         kind,
	}, nil
}

func decodePPBinary(data json.RawMessage) ([]byte, string, error) {
	var envelope struct {
		PPBinary    bool   `json:"_pp_binary"`
		ContentType string `json:"content_type"`
		Encoding    string `json:"encoding"`
		Data        string `json:"data"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, "", fmt.Errorf("recording response was not JSON-wrapped binary: %w", err)
	}
	if !envelope.PPBinary || envelope.Encoding != "base64" || envelope.Data == "" {
		return nil, "", fmt.Errorf("recording response did not contain a Printing Press binary envelope")
	}
	payload, err := base64.StdEncoding.DecodeString(envelope.Data)
	if err != nil {
		return nil, "", fmt.Errorf("decoding recording payload: %w", err)
	}
	return payload, envelope.ContentType, nil
}

func resolveRecordingPath(callID, kind, requestedPath, contentType string) string {
	if requestedPath == "" || requestedPath == "." {
		return fmt.Sprintf("vapi-call-%s-%s-recording%s", sanitizeFileStem(callID), kind, extensionForContentType(contentType))
	}
	if info, err := os.Stat(requestedPath); err == nil && info.IsDir() {
		return filepath.Join(requestedPath, fmt.Sprintf("vapi-call-%s-%s-recording%s", sanitizeFileStem(callID), kind, extensionForContentType(contentType)))
	}
	return requestedPath
}

func extensionForContentType(contentType string) string {
	ct := strings.ToLower(contentType)
	switch {
	case strings.Contains(ct, "mpeg"), strings.Contains(ct, "mp3"):
		return ".mp3"
	case strings.Contains(ct, "wav"), strings.Contains(ct, "wave"):
		return ".wav"
	case strings.Contains(ct, "ogg"):
		return ".ogg"
	default:
		return ".bin"
	}
}

func sanitizeFileStem(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func isJunoTerminalStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ended", "completed", "failed", "not-found", "deletion-failed", "canceled", "cancelled":
		return true
	default:
		return false
	}
}

func copyFirstPresent(dst, src map[string]any, keys ...string) {
	for _, key := range keys {
		if v, ok := src[key]; ok && v != nil {
			dst[key] = v
			return
		}
	}
}

func copySummaryField(dst, src map[string]any, outKey string, paths ...string) {
	if v := firstPathValue(src, paths...); v != nil {
		dst[outKey] = v
	}
}

func firstPathValue(src map[string]any, paths ...string) any {
	for _, path := range paths {
		if v, ok := anyAtPath(src, path); ok && v != nil && v != "" {
			return v
		}
	}
	return nil
}

func anyAtPath(src map[string]any, path string) (any, bool) {
	var current any = src
	for _, part := range strings.Split(path, ".") {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func mapFromAny(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	if m, ok := value.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func stringValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}
