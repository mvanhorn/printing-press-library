// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package deepline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// executeSubprocess runs `deepline tools execute <toolID> --payload <json>`
// and parses stdout as JSON. The DEEPLINE_API_KEY env var is inherited from
// the parent process; we never pass the key on the command line.
func (c *Client) executeSubprocess(ctx context.Context, toolID string, payload map[string]any) (json.RawMessage, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("deepline subprocess: marshaling payload: %w", err)
	}

	cmd := exec.CommandContext(ctx, c.cliPath, "tools", "execute", toolID, "--payload", string(payloadJSON))

	// Inherit env so DEEPLINE_API_KEY reaches the subprocess. We never echo
	// the key value in logs or error messages.
	// cmd.Env == nil -> child inherits parent env.

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Include stderr tail in the error so the user sees what upstream
		// said, but do not include the payload (may contain PII) or echo any
		// key material.
		stderrMsg := stderr.String()
		if len(stderrMsg) > 400 {
			stderrMsg = stderrMsg[:400] + "..."
		}
		return nil, fmt.Errorf("deepline subprocess %q failed: %w (stderr: %s)", toolID, err, stderrMsg)
	}

	out := bytes.TrimSpace(stdout.Bytes())
	if len(out) == 0 {
		return nil, fmt.Errorf("deepline subprocess %q returned empty stdout", toolID)
	}

	// Validate JSON before returning so callers can rely on RawMessage being
	// well-formed.
	if !json.Valid(out) {
		return nil, fmt.Errorf("deepline subprocess %q returned non-JSON stdout", toolID)
	}

	return json.RawMessage(out), nil
}
