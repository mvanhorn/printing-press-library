// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/gmail"
)

// TestThreadsTrash_AddsTrashRemovesInbox covers the trash verb end-to-end.
func TestThreadsTrash_AddsTrashRemovesInbox(t *testing.T) {
	var payload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &payload)
		_, _ = w.Write([]byte(`{"messages":[{"labelIds":["TRASH"]}]}`))
	}))
	defer srv.Close()
	withGmailBaseURL(t, srv.URL)

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, "http://unused", "user@example.com")

	_, _, err := executeCmd(t, "--config", configPath, "--json", "threads", "trash", "19abc", "--yes")
	if err != nil {
		t.Fatalf("threads trash: %v", err)
	}
	add, _ := payload["addLabelIds"].([]any)
	rm, _ := payload["removeLabelIds"].([]any)
	if len(add) != 1 || add[0] != "TRASH" {
		t.Fatalf("trash should addLabelIds=[TRASH], got %v", add)
	}
	if len(rm) != 1 || rm[0] != "INBOX" {
		t.Fatalf("trash should removeLabelIds=[INBOX], got %v", rm)
	}
}

// TestThreadsMarkSpam_AddsSpamRemovesInbox covers mark-spam end-to-end.
func TestThreadsMarkSpam_AddsSpamRemovesInbox(t *testing.T) {
	var payload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &payload)
		_, _ = w.Write([]byte(`{"messages":[{"labelIds":["SPAM"]}]}`))
	}))
	defer srv.Close()
	withGmailBaseURL(t, srv.URL)

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, "http://unused", "user@example.com")

	_, _, err := executeCmd(t, "--config", configPath, "--json", "threads", "mark-spam", "19abc", "--yes")
	if err != nil {
		t.Fatalf("threads mark-spam: %v", err)
	}
	add, _ := payload["addLabelIds"].([]any)
	if len(add) != 1 || add[0] != "SPAM" {
		t.Fatalf("mark-spam should addLabelIds=[SPAM], got %v", add)
	}
}

// TestThreadsTerminal_RequiresYesOnNonTTY ensures destructive commands
// refuse to fire when stdin is piped without --yes.
func TestThreadsTerminal_RequiresYesOnNonTTY(t *testing.T) {
	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, "http://unused", "user@example.com")

	// executeCmd's stdin is a *bytes.Buffer (non-TTY). Without --yes (and
	// without --json which auto-confirms) we expect a usage error.
	_, _, err := executeCmd(t, "--config", configPath, "threads", "trash", "19abc")
	if err == nil {
		t.Fatalf("expected usage error when no --yes and stdin not a terminal")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("exit code = %d want 2", got)
	}
}

// TestPickUnsubscribeTargets_RFC8058 exercises the header parser at unit
// level. RFC 8058 One-Click requires BOTH headers; mailto-only forms are
// surfaced for human follow-up.
func TestPickUnsubscribeTargets_RFC8058(t *testing.T) {
	cases := []struct {
		name        string
		headers     []gmail.Header
		wantURL     string
		wantMailto  string
	}{
		{
			name: "one-click with both headers and url + mailto",
			headers: []gmail.Header{
				{Name: "List-Unsubscribe", Value: "<https://example.com/u?id=42>, <mailto:unsub@example.com>"},
				{Name: "List-Unsubscribe-Post", Value: "List-Unsubscribe=One-Click"},
			},
			wantURL:    "https://example.com/u?id=42",
			wantMailto: "unsub@example.com",
		},
		{
			name: "mailto only (no one-click post header)",
			headers: []gmail.Header{
				{Name: "List-Unsubscribe", Value: "<mailto:unsub@example.com>"},
			},
			wantURL:    "",
			wantMailto: "unsub@example.com",
		},
		{
			name: "url present but no one-click post header — not auto-fired",
			headers: []gmail.Header{
				{Name: "List-Unsubscribe", Value: "<https://example.com/u?id=42>"},
			},
			wantURL:    "", // gated on Post: One-Click
			wantMailto: "",
		},
		{
			name:       "no list-unsubscribe header at all",
			headers:    []gmail.Header{{Name: "From", Value: "x@y"}},
			wantURL:    "",
			wantMailto: "",
		},
		{
			name: "case-insensitive header name match",
			headers: []gmail.Header{
				{Name: "LIST-UNSUBSCRIBE", Value: "<https://example.com/u>"},
				{Name: "LIST-UNSUBSCRIBE-POST", Value: "List-Unsubscribe=One-Click"},
			},
			wantURL:    "https://example.com/u",
			wantMailto: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotURL, gotMailto := pickUnsubscribeTargets(tc.headers)
			if gotURL != tc.wantURL {
				t.Errorf("url = %q want %q", gotURL, tc.wantURL)
			}
			if gotMailto != tc.wantMailto {
				t.Errorf("mailto = %q want %q", gotMailto, tc.wantMailto)
			}
		})
	}
}

// TestThreadsUnsubscribe_OneClickFiresAndArchives covers the happy
// One-Click path: a thread with proper RFC 8058 headers triggers an
// HTTP POST to the listed URL and is then archived.
func TestThreadsUnsubscribe_OneClickFiresAndArchives(t *testing.T) {
	var oneClickPostHits atomic.Int32
	oneClickSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			oneClickPostHits.Add(1)
			w.WriteHeader(200)
			return
		}
		http.Error(w, "wrong method", 405)
	}))
	defer oneClickSrv.Close()

	// Gmail fake: serves the messages.list (for fetchLatestThreadMessage),
	// the messages.get (for headers), and the threads.modify (for archive).
	plainBody := strings.TrimRight(base64.URLEncoding.EncodeToString([]byte("body")), "=")
	gmailSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/users/me/messages") && strings.Contains(r.URL.RawQuery, "threadId"):
			_, _ = w.Write([]byte(`{"messages":[{"id":"msg-1"}]}`))
		case strings.HasSuffix(r.URL.Path, "/users/me/messages/msg-1"):
			fixture := map[string]any{
				"id":       "msg-1",
				"threadId": "19abc",
				"payload": map[string]any{
					"headers": []map[string]string{
						{"name": "List-Unsubscribe", "value": "<" + oneClickSrv.URL + ">"},
						{"name": "List-Unsubscribe-Post", "value": "List-Unsubscribe=One-Click"},
					},
					"parts": []map[string]any{
						{"mimeType": "text/plain", "body": map[string]any{"size": 4, "data": plainBody}},
					},
				},
			}
			out, _ := json.Marshal(fixture)
			_, _ = w.Write(out)
		case strings.HasSuffix(r.URL.Path, "/threads/19abc/modify"):
			_, _ = w.Write([]byte(`{"messages":[{"labelIds":["UNREAD"]}]}`))
		default:
			http.Error(w, "unhandled path: "+r.URL.Path, 404)
		}
	}))
	defer gmailSrv.Close()
	withGmailBaseURL(t, gmailSrv.URL)

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, "http://unused", "user@example.com")

	stdout, _, err := executeCmd(t, "--config", configPath, "--json", "threads", "unsubscribe", "19abc", "--yes")
	if err != nil {
		t.Fatalf("threads unsubscribe: %v", err)
	}
	if got := oneClickPostHits.Load(); got != 1 {
		t.Fatalf("expected exactly 1 One-Click POST, got %d", got)
	}
	var env map[string]any
	_ = json.Unmarshal([]byte(stdout), &env)
	if env["archived"] != true {
		t.Fatalf("envelope archived = %v want true", env["archived"])
	}
	unsub, _ := env["unsubscribe"].(string)
	if !strings.Contains(unsub, "One-Click POST sent") {
		t.Fatalf("envelope unsubscribe = %q want 'One-Click POST sent'", unsub)
	}
}

// TestThreadsUnsubscribe_NoListUnsubscribeHeader still archives but
// reports "no List-Unsubscribe header".
func TestThreadsUnsubscribe_NoListUnsubscribeHeader(t *testing.T) {
	gmailSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/users/me/messages") && strings.Contains(r.URL.RawQuery, "threadId"):
			_, _ = w.Write([]byte(`{"messages":[{"id":"msg-1"}]}`))
		case strings.HasSuffix(r.URL.Path, "/users/me/messages/msg-1"):
			_, _ = w.Write([]byte(`{"id":"msg-1","payload":{"headers":[{"name":"From","value":"x@y"}]}}`))
		case strings.HasSuffix(r.URL.Path, "/threads/19abc/modify"):
			_, _ = w.Write([]byte(`{"messages":[{"labelIds":["UNREAD"]}]}`))
		default:
			http.Error(w, "unhandled: "+r.URL.Path, 404)
		}
	}))
	defer gmailSrv.Close()
	withGmailBaseURL(t, gmailSrv.URL)

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, "http://unused", "user@example.com")

	stdout, _, err := executeCmd(t, "--config", configPath, "--json", "threads", "unsubscribe", "19abc", "--yes")
	if err != nil {
		t.Fatalf("threads unsubscribe: %v", err)
	}
	var env map[string]any
	_ = json.Unmarshal([]byte(stdout), &env)
	if env["archived"] != true {
		t.Fatalf("envelope archived = %v want true", env["archived"])
	}
	unsub, _ := env["unsubscribe"].(string)
	if !strings.Contains(unsub, "no List-Unsubscribe") {
		t.Fatalf("envelope unsubscribe = %q want 'no List-Unsubscribe'", unsub)
	}
}
