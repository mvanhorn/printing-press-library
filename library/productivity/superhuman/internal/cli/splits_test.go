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

// newSuperhumanBackendFake returns a tiny httptest server that handles the
// POST /v3/userdata.read shape used by splits + read-status. The handler
// records the requested path so tests can verify it without parsing the
// full body.
type backendFake struct {
	srv          *httptest.Server
	requestPaths []string
	respBody     string
}

func newSuperhumanBackendFake(t *testing.T, respBody string) *backendFake {
	t.Helper()
	f := &backendFake{respBody: respBody}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/v3/userdata.read") {
			http.Error(w, "wrong path: "+r.URL.Path, 404)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]any
		if jerr := json.Unmarshal(body, &parsed); jerr == nil {
			if reads, ok := parsed["reads"].([]any); ok && len(reads) > 0 {
				if first, ok := reads[0].(map[string]any); ok {
					if p, ok := first["path"].(string); ok {
						f.requestPaths = append(f.requestPaths, p)
					}
				}
			}
		}
		_, _ = w.Write([]byte(f.respBody))
	}))
	t.Cleanup(f.srv.Close)
	return f
}

// TestSplitsList_BackendCallShape verifies the request path is
// users/<gid>/splits and the response is surfaced in the JSON envelope.
func TestSplitsList_BackendCallShape(t *testing.T) {
	backend := newSuperhumanBackendFake(t, `[
		{"id":"split-1","name":"VIPs","filter":"from:ceo@*"},
		{"id":"split-2","name":"Newsletters","filter":"list:* OR has:list-unsubscribe"}
	]`)

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, backend.srv.URL, "user@example.com")

	stdout, _, err := executeCmd(t, "--config", configPath, "--json", "splits", "list")
	if err != nil {
		t.Fatalf("splits list: %v", err)
	}
	if len(backend.requestPaths) != 1 {
		t.Fatalf("expected 1 backend call, got %d", len(backend.requestPaths))
	}
	if backend.requestPaths[0] != "users/gid-001/splits" {
		t.Fatalf("path = %q want users/gid-001/splits", backend.requestPaths[0])
	}
	if !strings.Contains(stdout, "splits.list") {
		t.Fatalf("envelope missing action: %s", stdout)
	}
}

// TestSplitsList_EmptyResult_HumanFriendly asserts the human path prints a
// friendly message when no splits are configured.
func TestSplitsList_EmptyResult_HumanFriendly(t *testing.T) {
	backend := newSuperhumanBackendFake(t, `[]`)

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, backend.srv.URL, "user@example.com")

	stdout, _, err := executeCmd(t, "--config", configPath, "--plain", "splits", "list")
	if err != nil {
		t.Fatalf("splits list (empty): %v", err)
	}
	if !strings.Contains(stdout, "No splits") {
		t.Fatalf("empty splits should print 'No splits', got: %s", stdout)
	}
}

// TestSplitsList_NoActiveAccount surfaces a usable error.
func TestSplitsList_NoActiveAccount(t *testing.T) {
	configPath, _ := withConfigPath(t)
	writeConfigPointingAt(t, configPath, "http://unused", "")

	_, _, err := executeCmd(t, "--config", configPath, "splits", "list")
	if err == nil {
		t.Fatalf("expected error when no active account")
	}
}
