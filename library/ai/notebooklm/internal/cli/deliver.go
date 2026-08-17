// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DeliverSink routes command output when --deliver is set.
type DeliverSink struct {
	Scheme string
	Target string
}

// ParseDeliverSink parses file:<path> or webhook:<url> sinks.
func ParseDeliverSink(spec string) (DeliverSink, error) {
	if spec == "" || spec == "stdout" {
		return DeliverSink{Scheme: "stdout"}, nil
	}
	idx := strings.Index(spec, ":")
	if idx == -1 {
		return DeliverSink{}, fmt.Errorf("unknown --deliver sink %q: expected scheme:target (supported: stdout, file:<path>, webhook:<url>)", spec)
	}
	scheme := spec[:idx]
	target := spec[idx+1:]
	switch scheme {
	case "file":
		if target == "" {
			return DeliverSink{}, fmt.Errorf("--deliver file:<path> requires a path")
		}
	case "webhook":
		if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
			return DeliverSink{}, fmt.Errorf("--deliver webhook:<url> requires http(s) URL")
		}
	default:
		return DeliverSink{}, fmt.Errorf("unknown --deliver scheme %q", scheme)
	}
	return DeliverSink{Scheme: scheme, Target: target}, nil
}

// Deliver writes captured output to the configured sink.
func Deliver(sink DeliverSink, body []byte) error {
	switch sink.Scheme {
	case "", "stdout":
		return nil
	case "file":
		dir := filepath.Dir(sink.Target)
		if dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return err
			}
		}
		tmp := sink.Target + ".tmp"
		if err := os.WriteFile(tmp, body, 0o600); err != nil {
			return err
		}
		return os.Rename(tmp, sink.Target)
	case "webhook":
		req, err := http.NewRequest(http.MethodPost, sink.Target, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return fmt.Errorf("webhook returned %d", resp.StatusCode)
		}
		return nil
	default:
		return fmt.Errorf("unsupported deliver sink %q", sink.Scheme)
	}
}
