// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
//
// `report submit --wait` poll helper. After a submit, polls /OpenReport until
// the report's status leaves SUBMITTED/PROCESSING (approved, rejected, etc.) or
// the timeout expires. Recorded in .printing-press-patches.json as `novel-layer`.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// waitForSubmitExit polls /OpenReport every 30s until the report's status is
// something other than SUBMITTED/PROCESSING, or the timeout expires.
func waitForSubmitExit(ctx context.Context, c interface {
	Post(ctx context.Context, path string, body any) (json.RawMessage, int, error)
}, reportID string, timeout time.Duration, w io.Writer) (string, error) {
	deadline := time.Now().Add(timeout)
	var last string
	for {
		if time.Now().After(deadline) {
			return last, fmt.Errorf("timed out waiting for report %s to leave SUBMITTED (last status: %s)", reportID, last)
		}
		data, status, err := c.Post(ctx, "/OpenReport", map[string]any{"reportID": reportID})
		if err != nil {
			return last, apiErr(err)
		}
		if status < 200 || status >= 300 {
			return last, apiErr(fmt.Errorf("OpenReport returned HTTP %d", status))
		}
		curr := extractReportStatus(data)
		last = curr
		upper := strings.ToUpper(curr)
		if upper != "" && upper != "SUBMITTED" && upper != "PROCESSING" {
			return curr, nil
		}
		fmt.Fprintf(w, "report %s status: %s — polling again in 30s...\n", reportID, curr)
		select {
		case <-ctx.Done():
			return last, ctx.Err()
		case <-time.After(30 * time.Second):
		}
	}
}

func extractReportStatus(data json.RawMessage) string {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	for _, k := range []string{"status", "state", "stateNum", "statusNum"} {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
		if v, ok := m[k].(float64); ok && v != 0 {
			return fmt.Sprintf("%d", int64(v))
		}
	}
	if r, ok := m["report"].(map[string]any); ok {
		for _, k := range []string{"status", "state", "stateNum"} {
			if v, ok := r[k].(string); ok && v != "" {
				return v
			}
		}
	}
	return ""
}
