// Copyright 2026 Paul Taramona and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/cliutil"
	"github.com/spf13/cobra"
	"sort"
	"strings"
	"text/tabwriter"
	"time"
)

// pp:data-source live
func newNovelImportsWatchCmd(flags *rootFlags) *cobra.Command {
	var verify bool
	var interval, watchTimeout time.Duration
	cmd := &cobra.Command{Use: "watch <jobId>", Short: "Blocks until an import job reaches a terminal state", Long: "Use this command to block until an import job completes. Do NOT use it for a finished job's per-row outcomes; use 'imports blame' instead.", Example: "  conduyt-crm-pp-cli imports watch 3b9e4c2d --verify --interval 10s", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:typed-exit-codes": "0,2,5"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "imports watch")
		}
		if len(args) == 0 && !cmd.Flags().Changed("verify") && !cmd.Flags().Changed("interval") && !cmd.Flags().Changed("timeout") {
			return cmd.Help()
		}
		if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
			_ = cmd.Usage()
			return usageErr(fmt.Errorf("jobId is required"))
		}
		if interval <= 0 || watchTimeout <= 0 {
			_ = cmd.Usage()
			return usageErr(fmt.Errorf("--interval and --timeout must be positive"))
		}
		base, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		ctx, stop := context.WithTimeout(base, watchTimeout)
		defer stop()
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		jobID := args[0]
		job := map[string]any{}
		monitor := map[string]any{}
		monitorFailure := ""
		var monitorErr error
		polls := 0
		for {
			polls++
			raw, e := c.GetNoCache(ctx, "/imports/"+jobID, nil)
			if e != nil {
				return classifyAPIErrorOnly(fmt.Errorf("fetching import job: %w", e))
			}
			if e = rejectResponseErrorEnvelope(raw); e != nil {
				view := watchView{JobID: jobID, Partial: true, Failures: []string{"decoding import job: " + e.Error()}}
				return outputWatchFailure(cmd, flags, view, e)
			}
			var env map[string]any
			if e = json.Unmarshal(raw, &env); e != nil {
				return apiErr(fmt.Errorf("parsing import job: %w", e))
			}
			job = unwrapMap(env)
			if strAny(job, "status") == "" {
				view := watchView{JobID: jobID, Partial: true, Failures: []string{"import job response missing status"}}
				return outputWatchFailure(cmd, flags, view, fmt.Errorf("import job response has an unexpected shape"))
			}
			if mr, e := c.GetNoCache(ctx, "/imports/"+jobID+"/launch-monitor", nil); e == nil {
				var me map[string]any
				if decodeErr := rejectResponseErrorEnvelope(mr); decodeErr != nil {
					monitorFailure = "decoding launch monitor: " + decodeErr.Error()
					monitorErr = decodeErr
				} else if decodeErr := json.Unmarshal(mr, &me); decodeErr == nil {
					monitor = unwrapMap(me)
					monitorFailure = ""
					monitorErr = nil
				} else {
					monitorFailure = "decoding launch monitor: " + decodeErr.Error()
				}
			} else {
				monitorFailure = "fetching launch monitor: " + e.Error()
				monitorErr = e
			}
			status := strings.ToLower(strAny(job, "status"))
			if status == "completed" || status == "failed" || status == "cancelled" || cliutil.IsDogfoodEnv() && polls >= 3 {
				break
			}
			timer := time.NewTimer(interval)
			select {
			case <-ctx.Done():
				timer.Stop()
				return apiErr(fmt.Errorf("watching import: %w", ctx.Err()))
			case <-timer.C:
			}
		}
		view := watchView{JobID: jobID, Status: strAny(job, "status"), Stage: strAny(monitor, "stage"), FailureType: strAny(monitor, "failureType", "failure_type"), Failures: []string{}}
		missingCounters := make([]string, 0, 4)
		for name, spec := range map[string][]string{
			"created": {"createdRows", "created_rows", "created"},
			"updated": {"updatedRows", "updated_rows", "updated"},
			"skipped": {"skippedRows", "skipped_rows", "skipped"},
			"errors":  {"errorRows", "error_rows", "errors"},
		} {
			parsed, counterErr := preferredLegacyCounter(job, spec[0], spec[1:]...)
			if parsed == nil {
				if counterErr != nil {
					missingCounters = append(missingCounters, counterErr.Error())
				} else {
					missingCounters = append(missingCounters, name)
				}
				continue
			}
			switch name {
			case "created":
				view.Created = *parsed
			case "updated":
				view.Updated = *parsed
			case "skipped":
				view.Skipped = *parsed
			case "errors":
				view.Errors = *parsed
			}
		}
		if len(missingCounters) > 0 {
			sort.Strings(missingCounters)
			view.Partial = true
			view.Failures = append(view.Failures, "terminal import response missing or invalid counters: "+strings.Join(missingCounters, ", "))
		}
		if monitorFailure != "" {
			view.Partial = true
			view.Failures = append(view.Failures, monitorFailure)
		}
		if verify {
			var verifyErr error
			view.Verification, view.FetchFailures, verifyErr = verifyImportJob(ctx, c, jobID)
			if monitorErr == nil {
				monitorErr = verifyErr
			}
			if view.FetchFailures > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of 2 verification fetches failed\n", view.FetchFailures)
			}
			if view.Verification == nil || !view.Verification.Verified {
				view.Partial = true
				reason := "verification was not completed"
				if view.Verification != nil && view.Verification.Reason != "" {
					reason = view.Verification.Reason
				}
				view.Failures = append(view.Failures, reason)
			}
		}
		if !wantsHumanTable(cmd.OutOrStdout(), flags) {
			if e := printJSONFiltered(cmd.OutOrStdout(), view, flags); e != nil {
				return e
			}
		} else {
			if view.Partial {
				fmt.Fprintln(cmd.OutOrStdout(), "WARNING: import result is partial: "+strings.Join(view.Failures, "; "))
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "JOB\tSTATUS\tSTAGE\tCREATED\tUPDATED\tSKIPPED\tERRORS")
			fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%d\t%d\t%d\n", view.JobID, view.Status, view.Stage, view.Created, view.Updated, view.Skipped, view.Errors)
			if e := tw.Flush(); e != nil {
				return e
			}
			if view.Verification != nil {
				verification := view.Verification
				fmt.Fprintf(cmd.OutOrStdout(), "VERIFIED: %t", verification.Verified)
				if verification.Reason != "" {
					fmt.Fprintf(cmd.OutOrStdout(), " (%s)", verification.Reason)
				}
				fmt.Fprintln(cmd.OutOrStdout())
				actionRequired := "unknown"
				if verification.ActionRequired != nil {
					actionRequired = fmt.Sprintf("%t", *verification.ActionRequired)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "ACTION REQUIRED: %s\n", actionRequired)
			}
		}
		if strings.EqualFold(view.Status, "failed") || strings.EqualFold(view.Status, "cancelled") {
			return apiErr(fmt.Errorf("import ended %s", view.Status))
		}
		if view.Partial {
			if monitorErr != nil {
				return classifyAPIErrorOnly(monitorErr)
			}
			return apiErr(fmt.Errorf("imports watch is incomplete: %s", strings.Join(view.Failures, "; ")))
		}
		return nil
	}}
	cmd.Flags().BoolVar(&verify, "verify", false, "Re-read the finished import and its job-scoped side effects")
	cmd.Flags().DurationVar(&interval, "interval", 10*time.Second, "Poll interval")
	cmd.Flags().DurationVar(&watchTimeout, "timeout", 30*time.Minute, "Maximum time to wait")
	return cmd
}

func outputWatchFailure(cmd *cobra.Command, flags *rootFlags, view watchView, cause error) error {
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "WARNING: import result is partial: %v\n", cause)
	}
	return classifyAPIErrorOnly(cause)
}

type watchView struct {
	JobID         string             `json:"job_id"`
	Status        string             `json:"status"`
	Stage         string             `json:"stage,omitempty"`
	FailureType   string             `json:"failure_type,omitempty"`
	Created       int                `json:"created"`
	Updated       int                `json:"updated"`
	Skipped       int                `json:"skipped"`
	Errors        int                `json:"errors"`
	Verification  *watchVerification `json:"verification,omitempty"`
	FetchFailures int                `json:"fetch_failures"`
	Partial       bool               `json:"partial"`
	Failures      []string           `json:"failures,omitempty"`
}

type watchVerification struct {
	Verified       bool             `json:"verified"`
	Reason         string           `json:"reason,omitempty"`
	Status         string           `json:"status,omitempty"`
	Total          *int             `json:"total,omitempty"`
	Created        *int             `json:"created,omitempty"`
	Updated        *int             `json:"updated,omitempty"`
	Skipped        *int             `json:"skipped,omitempty"`
	Errors         *int             `json:"errors,omitempty"`
	Processed      *int             `json:"processed,omitempty"`
	SideEffects    watchSideEffects `json:"side_effects"`
	ActionRequired *bool            `json:"action_required"`
}

type watchSideEffects struct {
	Queued         *int  `json:"queued"`
	Retryable      *int  `json:"retryable"`
	Exhausted      *int  `json:"exhausted"`
	Total          *int  `json:"total"`
	Completed      *int  `json:"completed"`
	Failed         *int  `json:"failed"`
	Held           *int  `json:"held"`
	ActionRequired *bool `json:"action_required"`
	IsComplete     bool  `json:"is_complete"`
}

type importGetter interface {
	GetNoCache(context.Context, string, map[string]string) (json.RawMessage, error)
}

func verifyImportJob(ctx context.Context, c importGetter, jobID string) (*watchVerification, int, error) {
	verification := &watchVerification{}
	missing := make([]string, 0)
	failures := 0
	var fetchErr error
	jobRaw, err := c.GetNoCache(ctx, "/imports/"+jobID, nil)
	if err != nil {
		fetchErr = err
		failures++
		missing = append(missing, "finished import job")
	} else {
		var env map[string]any
		if rejectResponseErrorEnvelope(jobRaw) != nil || json.Unmarshal(jobRaw, &env) != nil {
			failures++
			missing = append(missing, "parseable import job")
		} else {
			job := unwrapMap(env)
			verification.Status = strAny(job, "status")
			rowCounts := []struct {
				target    **int
				preferred string
				legacy    []string
			}{
				{&verification.Total, "totalRows", []string{"total_rows", "total"}},
				{&verification.Created, "createdRows", []string{"created_rows", "created"}},
				{&verification.Updated, "updatedRows", []string{"updated_rows", "updated"}},
				{&verification.Skipped, "skippedRows", []string{"skipped_rows", "skipped"}},
				{&verification.Errors, "errorRows", []string{"error_rows", "errors"}},
				{&verification.Processed, "processedRows", []string{"processed_rows", "processed"}},
			}
			for _, spec := range rowCounts {
				value, counterErr := preferredLegacyCounter(job, spec.preferred, spec.legacy...)
				if counterErr != nil {
					missing = append(missing, counterErr.Error())
					continue
				}
				*spec.target = value
			}
			if verification.Status == "" {
				missing = append(missing, "job status")
			} else if !terminalImportStatus(verification.Status) {
				missing = append(missing, "terminal job status")
			}
			if verification.Total == nil && verification.Created == nil && verification.Updated == nil && verification.Skipped == nil && verification.Errors == nil && verification.Processed == nil {
				missing = append(missing, "job row counts")
			}
		}
	}
	sideRaw, err := c.GetNoCache(ctx, "/imports/"+jobID+"/side-effects", nil)
	if err != nil {
		if fetchErr == nil {
			fetchErr = err
		}
		failures++
		missing = append(missing, "job side effects")
	} else {
		var env map[string]any
		if rejectResponseErrorEnvelope(sideRaw) != nil || json.Unmarshal(sideRaw, &env) != nil {
			failures++
			missing = append(missing, "parseable job side effects")
		} else {
			side := unwrapMap(env)
			counters := []struct {
				name      string
				preferred string
				legacy    []string
				target    **int
			}{
				{"queued", "queued", nil, &verification.SideEffects.Queued},
				{"retryableFailed", "retryableFailed", []string{"retryable"}, &verification.SideEffects.Retryable},
				{"exhaustedFailed", "exhaustedFailed", []string{"exhausted"}, &verification.SideEffects.Exhausted},
				{"total", "total", nil, &verification.SideEffects.Total},
				{"completed", "completed", nil, &verification.SideEffects.Completed},
				{"failed", "failed", nil, &verification.SideEffects.Failed},
				{"held", "held", nil, &verification.SideEffects.Held},
			}
			for _, spec := range counters {
				value, counterErr := preferredLegacyCounter(side, spec.preferred, spec.legacy...)
				if value == nil {
					if counterErr != nil {
						missing = append(missing, counterErr.Error())
					} else {
						missing = append(missing, spec.name)
					}
					continue
				}
				*spec.target = value
			}
			actionRequired, actionPresent, actionErr := preferredLegacyBool(side, "actionRequired", "action_required")
			isComplete, isCompleteErr := requiredBool(side, "isComplete")
			if actionErr != nil {
				missing = append(missing, actionErr.Error())
			}
			if isCompleteErr != nil {
				missing = append(missing, strings.TrimPrefix(isCompleteErr.Error(), "missing "))
			} else {
				verification.SideEffects.IsComplete = isComplete
				if !isComplete {
					missing = append(missing, "side effects are not complete")
				}
			}
			if actionErr == nil && actionPresent {
				verification.SideEffects.ActionRequired = &actionRequired
				verification.ActionRequired = &actionRequired
			}
		}
	}
	verification.Verified = len(missing) == 0
	if !verification.Verified {
		verification.Reason = "missing " + strings.Join(missing, ", ")
	}
	return verification, failures, fetchErr
}

func terminalImportStatus(status string) bool {
	return strings.EqualFold(status, "completed") || strings.EqualFold(status, "failed") || strings.EqualFold(status, "cancelled")
}

func optionalIntAny(m map[string]any, keys ...string) *int {
	for _, key := range keys {
		if value, ok := m[key]; ok {
			if n, ok := numericInt(value); ok {
				return &n
			}
		}
	}
	return nil
}

func preferredLegacyCounter(m map[string]any, preferred string, legacy ...string) (*int, error) {
	_, preferredPresent := m[preferred]
	if preferredPresent {
		preferredInt, err := nonNegativeCounter(m, preferred)
		if err != nil {
			return nil, err
		}
		for _, legacyKey := range legacy {
			if _, present := m[legacyKey]; !present {
				continue
			}
			legacyInt, err := nonNegativeCounter(m, legacyKey)
			if err != nil {
				return nil, err
			}
			if *preferredInt != *legacyInt {
				return nil, fmt.Errorf("conflicting %s=%d and %s=%d", preferred, *preferredInt, legacyKey, *legacyInt)
			}
		}
		return preferredInt, nil
	}
	var selected *int
	selectedKey := ""
	for _, legacyKey := range legacy {
		if _, present := m[legacyKey]; !present {
			continue
		}
		legacyInt, err := nonNegativeCounter(m, legacyKey)
		if err != nil {
			return nil, err
		}
		if selected != nil && *selected != *legacyInt {
			return nil, fmt.Errorf("conflicting %s=%d and %s=%d", selectedKey, *selected, legacyKey, *legacyInt)
		}
		selected, selectedKey = legacyInt, legacyKey
	}
	return selected, nil
}

func nonNegativeCounter(m map[string]any, key string) (*int, error) {
	value, present := m[key]
	if !present {
		return nil, nil
	}
	n, ok := numericInt(value)
	if !ok || n < 0 {
		return nil, fmt.Errorf("invalid %s", key)
	}
	return &n, nil
}

func requiredBool(m map[string]any, key string) (bool, error) {
	value, ok := m[key].(bool)
	if !ok {
		return false, fmt.Errorf("missing or invalid %s", key)
	}
	return value, nil
}

func preferredLegacyBool(m map[string]any, preferred, legacy string) (bool, bool, error) {
	preferredValue, preferredPresent := m[preferred]
	legacyValue, legacyPresent := m[legacy]
	if preferredPresent {
		preferredBool, ok := preferredValue.(bool)
		if !ok {
			return false, true, fmt.Errorf("invalid %s", preferred)
		}
		if legacyPresent {
			legacyBool, ok := legacyValue.(bool)
			if !ok {
				return false, true, fmt.Errorf("invalid %s", legacy)
			}
			if preferredBool != legacyBool {
				return false, true, fmt.Errorf("conflicting %s=%t and %s=%t", preferred, preferredBool, legacy, legacyBool)
			}
		}
		return preferredBool, true, nil
	}
	if !legacyPresent {
		return false, false, nil
	}
	legacyBool, ok := legacyValue.(bool)
	if !ok {
		return false, true, fmt.Errorf("invalid %s", legacy)
	}
	return legacyBool, true, nil
}

func preferredLegacyString(m map[string]any, preferred, legacy string) (string, error) {
	preferredValue, preferredPresent := m[preferred]
	legacyValue, legacyPresent := m[legacy]
	if preferredPresent {
		preferredString, ok := preferredValue.(string)
		if !ok || strings.TrimSpace(preferredString) == "" {
			return "", fmt.Errorf("invalid %s", preferred)
		}
		if legacyPresent {
			legacyString, ok := legacyValue.(string)
			if !ok || strings.TrimSpace(legacyString) == "" {
				return "", fmt.Errorf("invalid %s", legacy)
			}
			if preferredString != legacyString {
				return "", fmt.Errorf("conflicting %s=%q and %s=%q", preferred, preferredString, legacy, legacyString)
			}
		}
		return preferredString, nil
	}
	if !legacyPresent {
		return "", fmt.Errorf("missing %s", preferred)
	}
	legacyString, ok := legacyValue.(string)
	if !ok || strings.TrimSpace(legacyString) == "" {
		return "", fmt.Errorf("invalid %s", legacy)
	}
	return legacyString, nil
}

func decodeDNC(raw json.RawMessage) map[string]bool {
	out := map[string]bool{}
	items, _ := objectItems(raw, false)
	for _, m := range items {
		if b, ok := m["dnc"].(bool); ok && !b {
			continue
		}
		id := strAny(m, "contactId", "contact_id", "id")
		if id != "" {
			out[id] = true
		}
		p, ok := normalizePhoneIdentity(strAny(m, "phone", "normalizedPhone"))
		if ok {
			out[p] = true
		}
	}
	var env map[string]any
	// decodeDNCResponse validates the same bytes before this shared helper is called.
	if json.Unmarshal(raw, &env) == nil {
		if d, ok := env["data"].(map[string]any); ok {
			for k, v := range d {
				if b, ok := v.(bool); ok && b {
					out[k] = true
					if phone, phoneOK := normalizePhoneIdentity(k); phoneOK {
						out[phone] = true
					}
				}
			}
		}
	}
	return out
}

func objectItems(raw json.RawMessage, allowSingleObject bool) ([]map[string]any, error) {
	if err := rejectResponseErrorEnvelope(raw); err != nil {
		return nil, err
	}
	if hoisted, ok := hoistPaginatedEnvelope(raw); ok {
		raw = hoisted
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	if m, ok := v.(map[string]any); ok {
		found := false
		for _, k := range []string{"data", "contacts", "rows", "items", "entries"} {
			if x, exists := m[k]; exists {
				v, found = x, true
				break
			}
		}
		if !found {
			if allowSingleObject {
				if _, bad := m["error"]; bad || m["message"] != nil {
					return nil, fmt.Errorf("unexpected single-object response envelope")
				}
				return []map[string]any{m}, nil
			}
			return nil, fmt.Errorf("unexpected collection response envelope")
		}
		if nested, ok := v.(map[string]any); ok && !allowSingleObject {
			found = false
			for _, k := range []string{"contacts", "rows", "items", "entries"} {
				if x, exists := nested[k]; exists {
					v, found = x, true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("collection response data is not an array")
			}
		}
		if single, ok := v.(map[string]any); ok && allowSingleObject {
			if _, bad := single["error"]; bad || single["message"] != nil {
				return nil, fmt.Errorf("unexpected single-object response data")
			}
			return []map[string]any{single}, nil
		}
	}
	a, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("collection response is not an array")
	}
	out := make([]map[string]any, 0, len(a))
	for i, x := range a {
		m, ok := x.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("collection item %d is not an object", i)
		}
		out = append(out, m)
	}
	return out, nil
}

func strAny(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok {
			return s
		}
	}
	return ""
}
func anyStrings(v any) []string {
	out := make([]string, 0)
	switch x := v.(type) {
	case []any:
		for _, e := range x {
			if s, ok := e.(string); ok {
				out = append(out, s)
			} else if m, ok := e.(map[string]any); ok {
				out = append(out, strAny(m, "id", "name"))
			}
		}
	case []string:
		out = append(out, x...)
	}
	return out
}
