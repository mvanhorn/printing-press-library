// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Sink represents a destination for watch/alert events.
type Sink struct {
	Kind   string // stdout, file, exec, slack, webhook, macos
	Target string // path/url/command depending on kind
}

// ParseSink parses a "kind:target" string into a Sink.
//   - "stdout" or "stdout:" -> stdout
//   - "file:/path"          -> file
//   - "exec:/path/to/cmd"   -> exec
//   - "slack:<webhook url>" -> slack
//   - "webhook:<url>"       -> generic JSON POST
//   - "macos:" or "macos:<title>" -> macOS user notification
func ParseSink(s string) (Sink, error) {
	if s == "" || s == "-" {
		return Sink{Kind: "stdout"}, nil
	}
	idx := strings.Index(s, ":")
	if idx == -1 {
		return Sink{Kind: s}, nil
	}
	kind := strings.ToLower(s[:idx])
	target := s[idx+1:]
	switch kind {
	case "stdout", "file", "exec", "slack", "webhook", "macos":
		return Sink{Kind: kind, Target: target}, nil
	}
	return Sink{}, fmt.Errorf("unknown sink kind: %q (allowed: stdout, file, exec, slack, webhook, macos)", kind)
}

// Emit fires `event` to the sink and returns (outcome, error). When the
// sink is "stdout" Emit writes a single JSON-encoded line to stdout.
func (s Sink) Emit(ctx context.Context, event any) (string, error) {
	payload, err := json.Marshal(event)
	if err != nil {
		return "", err
	}
	switch s.Kind {
	case "stdout", "":
		fmt.Fprintln(os.Stdout, string(payload))
		return "stdout-write", nil
	case "file":
		f, err := os.OpenFile(s.Target, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return "", err
		}
		defer f.Close()
		if _, err := f.Write(append(payload, '\n')); err != nil {
			return "", err
		}
		return "file-append", nil
	case "exec":
		// Run the command and pipe the JSON payload to stdin.
		cmd := exec.CommandContext(ctx, s.Target)
		cmd.Stdin = bytes.NewReader(payload)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return "", fmt.Errorf("exec sink %q failed (exit %d): %s", s.Target, exitErr.ExitCode(), strings.TrimSpace(stderr.String()))
			}
			return "", fmt.Errorf("exec sink: %w", err)
		}
		return "exec-ok", nil
	case "slack":
		return postWebhook(ctx, s.Target, map[string]any{"text": summarizeEvent(event)})
	case "webhook":
		return postWebhook(ctx, s.Target, event)
	case "macos":
		title := s.Target
		if title == "" {
			title = "cmux-pp-cli alert"
		}
		body := summarizeEvent(event)
		script := fmt.Sprintf(`display notification %q with title %q`, body, title)
		cmd := exec.CommandContext(ctx, "osascript", "-e", script)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("osascript: %w (%s)", err, strings.TrimSpace(stderr.String()))
		}
		return "osascript-ok", nil
	}
	return "", fmt.Errorf("unknown sink: %s", s.Kind)
}

func postWebhook(ctx context.Context, url string, body any) (string, error) {
	if url == "" {
		return "", fmt.Errorf("webhook URL is empty")
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	httpClient := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("webhook %s returned HTTP %d: %s", url, resp.StatusCode, strings.TrimSpace(string(rb)))
	}
	return fmt.Sprintf("webhook-ok-%d", resp.StatusCode), nil
}

// summarizeEvent returns a one-line human summary of an event payload.
func summarizeEvent(event any) string {
	if m, ok := event.(map[string]any); ok {
		if t, _ := m["title"].(string); t != "" {
			if b, _ := m["body"].(string); b != "" {
				return fmt.Sprintf("%s — %s", t, truncate(b, 240))
			}
			return t
		}
		if ws, _ := m["workspace_title"].(string); ws != "" {
			from, _ := m["prev_value"].(string)
			to, _ := m["new_value"].(string)
			return fmt.Sprintf("%s: %s → %s", ws, from, to)
		}
	}
	raw, _ := json.Marshal(event)
	return truncate(string(raw), 240)
}
