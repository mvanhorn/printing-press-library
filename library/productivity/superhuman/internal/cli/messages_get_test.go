// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// gmailMessageFixture builds a Gmail messages.get response body containing
// the supplied text/plain body and one attachment.
func gmailMessageFixture(t *testing.T, id, threadID, plainBody, attachmentID, filename string, attSize int) string {
	t.Helper()
	encoded := strings.TrimRight(base64.URLEncoding.EncodeToString([]byte(plainBody)), "=")
	fixture := map[string]any{
		"id":        id,
		"threadId":  threadID,
		"labelIds":  []string{"INBOX", "UNREAD"},
		"snippet":   plainBody,
		"historyId": "h-001",
		"payload": map[string]any{
			"headers": []map[string]string{
				{"name": "From", "value": "alice@example.com"},
				{"name": "To", "value": "bob@example.com"},
				{"name": "Subject", "value": "U4 happy path"},
				{"name": "X-Internal", "value": "should-not-print"},
			},
			"parts": []map[string]any{
				{
					"mimeType": "text/plain",
					"body":     map[string]any{"size": len(plainBody), "data": encoded},
				},
				{
					"mimeType": "image/png",
					"filename": filename,
					"body":     map[string]any{"attachmentId": attachmentID, "size": attSize},
				},
			},
		},
	}
	b, _ := json.Marshal(fixture)
	return string(b)
}

// TestMessagesGet_JSON_EnvelopeIncludesBodyHeadersAttachments asserts the
// --json envelope contains the decoded body, the full headers, the
// attachment metadata, and the standard CLI envelope fields.
func TestMessagesGet_JSON_EnvelopeIncludesBodyHeadersAttachments(t *testing.T) {
	body := gmailMessageFixture(t, "msg-1", "th-1", "hello from U4", "att-1", "logo.png", 4096)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/users/me/messages/msg-1") {
			http.Error(w, "wrong path", 404)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	withGmailBaseURL(t, srv.URL)

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, "http://unused", "user@example.com")

	stdout, _, err := executeCmd(t, "--config", configPath, "--json", "messages", "get", "msg-1")
	if err != nil {
		t.Fatalf("messages get msg-1 --json: %v", err)
	}
	var env map[string]any
	if jerr := json.Unmarshal([]byte(stdout), &env); jerr != nil {
		t.Fatalf("parse envelope: %v\n%s", jerr, stdout)
	}
	if env["action"] != "messages.get" {
		t.Fatalf("action = %v want messages.get", env["action"])
	}
	data, _ := env["data"].(map[string]any)
	if data["body"] != "hello from U4" {
		t.Fatalf("body = %v want 'hello from U4'", data["body"])
	}
	if data["threadId"] != "th-1" {
		t.Fatalf("threadId = %v want th-1", data["threadId"])
	}
	atts, _ := data["attachments"].([]any)
	if len(atts) != 1 {
		t.Fatalf("attachments = %v want 1-entry", atts)
	}
}

// TestMessagesGet_MissingID_UsageError asserts the arg validator surfaces
// usage error (exit code 2).
func TestMessagesGet_MissingID_UsageError(t *testing.T) {
	configPath, _ := withConfigPath(t)
	_, _, err := executeCmd(t, "--config", configPath, "messages", "get")
	if err == nil {
		t.Fatalf("expected usage error when id missing, got nil")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("exit code = %d want 2", got)
	}
}

// TestMessagesGet_NotFound_TypedExit3 maps a Gmail 404 to the not-found
// exit code (3).
func TestMessagesGet_NotFound_TypedExit3(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()
	withGmailBaseURL(t, srv.URL)

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, "http://unused", "user@example.com")

	_, _, err := executeCmd(t, "--config", configPath, "--json", "messages", "get", "missing")
	if err == nil {
		t.Fatalf("expected error for 404, got nil")
	}
	if got := ExitCode(err); got != 3 {
		t.Fatalf("exit code = %d want 3 (not-found)", got)
	}
}

// TestMessagesGet_NoActiveAccount surfaces a usable error before the
// network call fires.
func TestMessagesGet_NoActiveAccount(t *testing.T) {
	configPath, _ := withConfigPath(t)
	writeConfigPointingAt(t, configPath, "http://unused", "")

	_, _, err := executeCmd(t, "--config", configPath, "messages", "get", "msg-1")
	if err == nil {
		t.Fatalf("expected error when no active account")
	}
	if !strings.Contains(err.Error(), "active account") && !strings.Contains(err.Error(), "auth login") {
		t.Fatalf("error %q missing remediation hint", err.Error())
	}
}
