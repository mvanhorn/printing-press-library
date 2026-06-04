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

// TestDraftsGet_HappyPath_BareDraftValueShape covers the most common
// response shape: /v3/userdata.read returns a bare draftValue object.
func TestDraftsGet_HappyPath_BareDraftValueShape(t *testing.T) {
	var observedPath string
	var observedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/v3/userdata.read") {
			http.Error(w, "wrong path: "+r.URL.Path, 404)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &observedBody)
		if reads, ok := observedBody["reads"].([]any); ok && len(reads) > 0 {
			if first, ok := reads[0].(map[string]any); ok {
				if p, ok := first["path"].(string); ok {
					observedPath = p
				}
			}
		}
		// Bare draftValue response.
		_, _ = w.Write([]byte(`{
			"id":"draft0012ab34cd56ef",
			"threadId":"draft0012ab34cd56ef",
			"action":"draft_persist",
			"name":null,
			"from":"user@example.com",
			"to":["alice@example.com"],
			"cc":[],
			"bcc":[],
			"subject":"Edited subject",
			"body":"Edited body",
			"snippet":"",
			"inReplyToRfc822Id":null,
			"labelIds":[],
			"clientCreatedAt":"2026-05-22T00:00:00.000Z",
			"date":"2026-05-22T00:00:00.000Z",
			"fingerprint":{"from":"","to":"","cc":"","bcc":"","subject":"","body":"","attachments":""},
			"lastSessionId":"sess",
			"quotedContent":"",
			"quotedContentInlined":false,
			"references":[],
			"reminder":null,
			"rfc822Id":"<rfc@example.com>",
			"scheduledFor":null,
			"scheduledReplyInterruptedAt":null,
			"schemaVersion":1,
			"totalComposeSeconds":0,
			"timeZone":"UTC"
		}`))
	}))
	defer srv.Close()

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, srv.URL, "user@example.com")

	stdout, _, err := executeCmd(t, "--config", configPath, "--json", "drafts", "get", "draft0012ab34cd56ef")
	if err != nil {
		t.Fatalf("drafts get --json: %v", err)
	}
	wantPath := "users/gid-001/threads/draft0012ab34cd56ef/messages/draft0012ab34cd56ef/draft"
	if observedPath != wantPath {
		t.Fatalf("path = %q want %q", observedPath, wantPath)
	}
	if !strings.Contains(stdout, "drafts.get") {
		t.Fatalf("envelope missing action: %s", stdout)
	}
	if !strings.Contains(stdout, "Edited body") {
		t.Fatalf("body not in envelope: %s", stdout)
	}
	if !strings.Contains(stdout, "Edited subject") {
		t.Fatalf("subject not in envelope: %s", stdout)
	}
}

// TestDraftsGet_DataWrapperShape covers the {data: draftValue} wrapper
// shape that mirrors threads.get.
func TestDraftsGet_DataWrapperShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{
			"id":"draft00wrapped",
			"threadId":"draft00wrapped",
			"action":"draft_persist",
			"name":null,
			"from":"user@example.com",
			"to":["alice@example.com"],
			"cc":[],
			"bcc":[],
			"subject":"wrap",
			"body":"wb",
			"snippet":"",
			"inReplyToRfc822Id":null,
			"labelIds":[],
			"clientCreatedAt":"2026-05-22T00:00:00.000Z",
			"date":"2026-05-22T00:00:00.000Z",
			"fingerprint":{"from":"","to":"","cc":"","bcc":"","subject":"","body":"","attachments":""},
			"lastSessionId":"sess",
			"quotedContent":"",
			"quotedContentInlined":false,
			"references":[],
			"reminder":null,
			"rfc822Id":"<r@e>",
			"scheduledFor":null,
			"scheduledReplyInterruptedAt":null,
			"schemaVersion":1,
			"totalComposeSeconds":0,
			"timeZone":"UTC"
		}}`))
	}))
	defer srv.Close()

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, srv.URL, "user@example.com")

	stdout, _, err := executeCmd(t, "--config", configPath, "--json", "drafts", "get", "draft00wrapped")
	if err != nil {
		t.Fatalf("drafts get --json: %v", err)
	}
	if !strings.Contains(stdout, "draft00wrapped") {
		t.Fatalf("data-wrapper shape not unmarshaled: %s", stdout)
	}
}

// TestDraftsGet_ReadsWrapperShape covers the {reads:[{value: …}]} shape.
func TestDraftsGet_ReadsWrapperShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"reads":[{"value":{
			"id":"draft00reads",
			"threadId":"draft00reads",
			"action":"draft_persist",
			"name":null,
			"from":"user@example.com",
			"to":[],
			"cc":[],
			"bcc":[],
			"subject":"r",
			"body":"rb",
			"snippet":"",
			"inReplyToRfc822Id":null,
			"labelIds":[],
			"clientCreatedAt":"2026-05-22T00:00:00.000Z",
			"date":"2026-05-22T00:00:00.000Z",
			"fingerprint":{"from":"","to":"","cc":"","bcc":"","subject":"","body":"","attachments":""},
			"lastSessionId":"s",
			"quotedContent":"",
			"quotedContentInlined":false,
			"references":[],
			"reminder":null,
			"rfc822Id":"<r@e>",
			"scheduledFor":null,
			"scheduledReplyInterruptedAt":null,
			"schemaVersion":1,
			"totalComposeSeconds":0,
			"timeZone":"UTC"
		}}]}`))
	}))
	defer srv.Close()

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, srv.URL, "user@example.com")

	stdout, _, err := executeCmd(t, "--config", configPath, "--json", "drafts", "get", "draft00reads")
	if err != nil {
		t.Fatalf("drafts get --json: %v", err)
	}
	if !strings.Contains(stdout, "draft00reads") {
		t.Fatalf("reads-wrapper shape not unmarshaled: %s", stdout)
	}
}

// TestDraftsGet_NotFound surfaces a typed not-found error (exit code 3)
// when the response is empty or carries no draft body.
func TestDraftsGet_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, srv.URL, "user@example.com")

	_, _, err := executeCmd(t, "--config", configPath, "--json", "drafts", "get", "draft00gone")
	if err == nil {
		t.Fatalf("expected not-found error for empty response")
	}
	if got := ExitCode(err); got != 3 {
		t.Fatalf("exit code = %d want 3", got)
	}
}

// TestDraftsGet_MissingArg surfaces a usage error.
func TestDraftsGet_MissingArg(t *testing.T) {
	configPath, _ := withConfigPath(t)
	_, _, err := executeCmd(t, "--config", configPath, "drafts", "get")
	if err == nil {
		t.Fatalf("expected usage error without draft id")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("exit code = %d want 2", got)
	}
}
