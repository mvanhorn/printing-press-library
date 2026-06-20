// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// draftThreadListBody is a minimal /v3/userdata.getThreads response carrying
// one draft whose thread id and message id differ — the real Superhuman
// shape that the single-id userdata.read path could not resolve.
const draftThreadListBody = `{"data":{"threadList":[{
  "id":"draft007cf1fe328668c3",
  "thread":{"messages":{"draft00f9306577168708":{"draft":{
    "id":"draft00f9306577168708",
    "threadId":"draft007cf1fe328668c3",
    "action":"forward",
    "from":"user@example.com",
    "to":["alice@example.com"],
    "cc":[],
    "bcc":[],
    "subject":"Resolved subject",
    "body":"Resolved body",
    "labelIds":["DRAFT"],
    "fingerprint":{"from":"","to":"","cc":"","bcc":"","subject":"","body":"","attachments":""},
    "schemaVersion":3
  }}}}
}]}}`

func draftsGetServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/v3/userdata.getThreads") {
			http.Error(w, "wrong path: "+r.URL.Path, http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(draftThreadListBody))
	}))
}

// TestDraftsGet_ResolvesByThreadID confirms `drafts get <thread-id>` resolves
// the draft via getThreads even though thread id != message id.
func TestDraftsGet_ResolvesByThreadID(t *testing.T) {
	srv := draftsGetServer(t)
	defer srv.Close()

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, srv.URL, "user@example.com")

	stdout, _, err := executeCmd(t, "--config", configPath, "--json", "drafts", "get", "draft007cf1fe328668c3")
	if err != nil {
		t.Fatalf("drafts get by thread id: %v", err)
	}
	if !strings.Contains(stdout, "Resolved body") || !strings.Contains(stdout, "Resolved subject") {
		t.Fatalf("draft not resolved by thread id: %s", stdout)
	}
}

// TestDraftsGet_ResolvesByMessageID confirms the message id also resolves.
func TestDraftsGet_ResolvesByMessageID(t *testing.T) {
	srv := draftsGetServer(t)
	defer srv.Close()

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, srv.URL, "user@example.com")

	stdout, _, err := executeCmd(t, "--config", configPath, "--json", "drafts", "get", "draft00f9306577168708")
	if err != nil {
		t.Fatalf("drafts get by message id: %v", err)
	}
	if !strings.Contains(stdout, "draft00f9306577168708") {
		t.Fatalf("draft not resolved by message id: %s", stdout)
	}
}

// TestDraftsGet_NotFound surfaces a typed not-found error (exit code 3)
// when no draft in the list matches the requested id.
func TestDraftsGet_NotFound(t *testing.T) {
	srv := draftsGetServer(t)
	defer srv.Close()

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, srv.URL, "user@example.com")

	_, _, err := executeCmd(t, "--config", configPath, "--json", "drafts", "get", "draft00gone")
	if err == nil {
		t.Fatalf("expected not-found error for unmatched id")
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
