// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/auth"
)

// fakeDraftFixture is the canonical user-edited draft used by the drafts
// send tests. The body intentionally differs from anything an agent
// would naively reconstruct from the CLI flags so the assertions can
// prove the wire actually carried the fetched draft, not a local copy.
const fakeEditedDraftBody = "Edited body — what the user actually wrote in the UI"
const fakeEditedDraftSubject = "Re: edits made in the web client"

// newSuperhumanForDraftsSend stands up a fake Superhuman backend that
// answers /v3/userdata.read with a draftValue carrying the edited body,
// and accepts /messages/send/log with a 200 + ok envelope.
func newSuperhumanForDraftsSend(t *testing.T, capture func(path string, body map[string]any)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]any
		_ = json.Unmarshal(body, &parsed)
		if capture != nil {
			capture(r.URL.Path, parsed)
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/v3/userdata.read"):
			_, _ = w.Write([]byte(`{"data":{"results":[{"path":"users/gid-001/threads/draft0099/messages/draft0099/draft","value":{
				"id":"draft0099","threadId":"draft0099","action":"draft_persist","name":null,
				"from":"User <user@example.com>","to":["alice@example.com"],"cc":[],"bcc":[],
				"subject":"` + fakeEditedDraftSubject + `","body":"` + fakeEditedDraftBody + `","snippet":"",
				"inReplyToRfc822Id":null,"labelIds":[],"clientCreatedAt":"2026-05-22T00:00:00.000Z",
				"date":"2026-05-22T00:00:00.000Z",
				"fingerprint":{"from":"","to":"","cc":"","bcc":"","subject":"","body":"","attachments":""},
				"lastSessionId":"s","quotedContent":"","quotedContentInlined":false,"references":[],
				"reminder":null,"rfc822Id":"<r@e>","scheduledFor":null,"scheduledReplyInterruptedAt":null,
				"schemaVersion":1,"totalComposeSeconds":0,"timeZone":"UTC"
			}}]}}`))
		case strings.HasSuffix(r.URL.Path, "/messages/send/log"):
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			http.Error(w, "unhandled path: "+r.URL.Path, 404)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestDraftsSend_HonorsServerSideBody is the headline regression test:
// the agent dispatches a draft id, the body that lands on the Gmail
// wire MUST equal the server-side body, NOT anything the agent could
// have composed from local memory.
func TestDraftsSend_HonorsServerSideBody(t *testing.T) {
	var observedRawBody string
	gmail := newCapturingGmail(t, &observedRawBody)
	withFakeGmail(t, gmail.URL+"/gmail/v1")
	withGmailRefreshFn(t, func(ctx context.Context, email, googleID string) (*auth.CookieAuthResult, error) {
		return nil, errors.New("refresh should not be called")
	})

	srv := newSuperhumanForDraftsSend(t, nil)

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, srv.URL, "user@example.com")

	stdout, _, err := executeCmd(t, "--config", configPath, "--json", "drafts", "send", "draft0099")
	if err != nil {
		t.Fatalf("drafts send: %v (stdout=%s)", err, stdout)
	}

	if !strings.Contains(stdout, "drafts.send") {
		t.Fatalf("envelope missing action: %s", stdout)
	}
	if !strings.Contains(observedRawBody, fakeEditedDraftBody) {
		t.Fatalf("Gmail wire body did not carry the server-side draft body.\nwant substring: %q\ngot: %q",
			fakeEditedDraftBody, observedRawBody)
	}
	if !strings.Contains(observedRawBody, fakeEditedDraftSubject) {
		t.Fatalf("Gmail wire missing edited subject. got: %q", observedRawBody)
	}
}

// TestDraftsSend_NotFound surfaces a typed not-found error (exit 3)
// when the draft is missing on the server.
func TestDraftsSend_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, srv.URL, "user@example.com")

	_, _, err := executeCmd(t, "--config", configPath, "--json", "drafts", "send", "draft00gone")
	if err == nil {
		t.Fatalf("expected not-found error for empty response")
	}
	if got := ExitCode(err); got != 3 {
		t.Fatalf("exit code = %d want 3", got)
	}
}

// TestDraftsSend_DryRun shows the resolved envelope and never fires Gmail.
func TestDraftsSend_DryRun(t *testing.T) {
	gmailHits := atomic.Int32{}
	gmail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gmailHits.Add(1)
	}))
	defer gmail.Close()
	withFakeGmail(t, gmail.URL+"/gmail/v1")

	srv := newSuperhumanForDraftsSend(t, nil)

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, srv.URL, "user@example.com")

	stdout, _, err := executeCmd(t, "--config", configPath, "--json", "--dry-run", "drafts", "send", "draft0099")
	if err != nil {
		t.Fatalf("drafts send --dry-run: %v", err)
	}
	if !strings.Contains(stdout, "dry_run") {
		t.Fatalf("dry-run envelope missing dry_run flag: %s", stdout)
	}
	if !strings.Contains(stdout, "draft0099") {
		t.Fatalf("dry-run envelope missing draft id: %s", stdout)
	}
	if got := gmailHits.Load(); got != 0 {
		t.Fatalf("Gmail fired %d times during dry-run, want 0", got)
	}
}

// TestDraftsSend_MissingArg surfaces a usage error.
func TestDraftsSend_MissingArg(t *testing.T) {
	configPath, _ := withConfigPath(t)
	_, _, err := executeCmd(t, "--config", configPath, "drafts", "send")
	if err == nil {
		t.Fatalf("expected usage error without draft id")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("exit code = %d want 2", got)
	}
}

// newCapturingGmail stands up a fake Gmail backend that decodes the raw
// base64url RFC822 payload and writes the decoded bytes into *captured
// for assertions. Returns 200 with a stub gmail id.
func newCapturingGmail(t *testing.T, captured *string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/users/me/messages/send") {
			http.Error(w, "wrong gmail path: "+r.URL.Path, 404)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var parsed struct {
			Raw string `json:"raw"`
		}
		_ = json.Unmarshal(body, &parsed)
		raw := parsed.Raw
		// Gmail accepts both base64url WITH and WITHOUT padding; trim and re-pad.
		switch n := len(raw) % 4; n {
		case 2:
			raw += "=="
		case 3:
			raw += "="
		}
		decoded, _ := base64.URLEncoding.DecodeString(raw)
		*captured = string(decoded)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"id":       "gmail-id-1",
			"threadId": "thread-id-1",
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}
