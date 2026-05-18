// Copyright 2026 twidtwid. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/pushover/internal/client"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/pushover/internal/config"
)

// TestMCPBodyIsJSONObjectNotBase64 locks in the contract makeAPIHandler now
// depends on: client.Post/Put/Patch accept `body any` and json-marshal it
// themselves. Passing a map[string]any sends a JSON object on the wire.
// Pre-marshaling to []byte (the bug from PR #511 Greptile review) makes Go
// re-encode the bytes as a base64-quoted JSON string, which Pushover and
// every other strict API will reject. A regression that re-introduces the
// pre-marshal at the makeAPIHandler call sites would also be visible here:
// every API client in this codebase shares the same Post/Put/Patch shape.
func TestMCPBodyIsJSONObjectNotBase64(t *testing.T) {
	var captured []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		captured = b
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":1}`))
	}))
	defer srv.Close()

	cfg := &config.Config{BaseURL: srv.URL}
	c := client.New(cfg, 5*time.Second, 0)
	c.NoCache = true

	bodyArgs := map[string]any{
		"token":   "app-tok",
		"user":    "user-key",
		"message": "hello",
	}
	if _, _, err := c.Post("/1/messages.json", bodyArgs); err != nil {
		t.Fatalf("c.Post: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(captured, &decoded); err != nil {
		t.Fatalf("captured body is not a JSON object: %v\nbody=%q", err, string(captured))
	}
	for _, k := range []string{"token", "user", "message"} {
		if _, ok := decoded[k]; !ok {
			t.Fatalf("body missing key %q; decoded=%v\nraw=%q", k, decoded, string(captured))
		}
	}
}
