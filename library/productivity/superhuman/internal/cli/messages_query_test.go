// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestMessagesQuery_BackendCallShape verifies the request body shape (query
// + scope + limit) and that the response surfaces via the envelope.
func TestMessagesQuery_BackendCallShape(t *testing.T) {
	var observed map[string]any
	var observedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observedPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &observed)
		_, _ = w.Write([]byte(`[{"threadId":"19abc","subject":"April invoice","score":0.92}]`))
	}))
	defer srv.Close()

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, srv.URL, "user@example.com")

	stdout, _, err := executeCmd(t, "--config", configPath, "--json", "messages", "query", "invoices from last month", "--limit", "5")
	if err != nil {
		t.Fatalf("messages query: %v", err)
	}
	if !strings.HasSuffix(observedPath, "/v3/ai.askAIProxy") {
		t.Fatalf("path = %q want /v3/ai.askAIProxy", observedPath)
	}
	if observed["query"] != "invoices from last month" {
		t.Fatalf("query = %v want invoices from last month", observed["query"])
	}
	if observed["scope"] != "email" {
		t.Fatalf("scope = %v want email", observed["scope"])
	}
	if observed["limit"].(float64) != 5 {
		t.Fatalf("limit = %v want 5", observed["limit"])
	}
	var env map[string]any
	_ = json.Unmarshal([]byte(stdout), &env)
	if env["action"] != "messages.query" {
		t.Fatalf("envelope action = %v want messages.query", env["action"])
	}
}

// TestMessagesQuery_EmptyArgument surfaces a usage error.
func TestMessagesQuery_EmptyArgument(t *testing.T) {
	configPath, _ := withConfigPath(t)
	_, _, err := executeCmd(t, "--config", configPath, "messages", "query", "")
	if err == nil {
		t.Fatalf("expected usage error for empty query")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("exit code = %d want 2", got)
	}
}

// TestMessagesQuery_MissingArg surfaces a usage error.
func TestMessagesQuery_MissingArg(t *testing.T) {
	configPath, _ := withConfigPath(t)
	_, _, err := executeCmd(t, "--config", configPath, "messages", "query")
	if err == nil {
		t.Fatalf("expected usage error without query text")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("exit code = %d want 2", got)
	}
}
