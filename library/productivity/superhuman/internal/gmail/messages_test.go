// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package gmail

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/auth"
)

// newSimpleSrv returns an httptest.Server whose handler dispatches based on
// the suffix of r.URL.Path. Each entry's value is the response body bytes;
// status is always 200 unless a value-error is returned by the handler.
func newSimpleSrv(t *testing.T, byPath map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for suffix, body := range byPath {
			if strings.HasSuffix(r.URL.Path, suffix) {
				w.WriteHeader(200)
				_, _ = w.Write([]byte(body))
				return
			}
		}
		http.Error(w, "no fixture for "+r.URL.Path, 404)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// nonRefreshingClient is a Client whose refresh seam fails loudly — used
// when a test should never hit the refresh path. Catches regressions where
// the typed helpers accidentally trigger refresh on non-401 responses.
func nonRefreshingClient(t *testing.T) *Client {
	c := New(newStore(t), "user@example.com", "gid-001", "ya29.initial")
	c.Refresh = func(ctx context.Context, email, gid string) (*auth.CookieAuthResult, error) {
		t.Helper()
		return nil, errors.New("refresh should not be called by this helper test")
	}
	return c
}

// TestListInboxThreads_HappyPath exercises the canonical 25-result inbox.
func TestListInboxThreads_HappyPath(t *testing.T) {
	srv := newSimpleSrv(t, map[string]string{
		"/users/me/threads": `{
			"threads": [
				{"id": "t1", "historyId": "h1"},
				{"id": "t2", "historyId": "h2"},
				{"id": "t3", "historyId": "h3"}
			],
			"nextPageToken": "tok2",
			"resultSizeEstimate": 142
		}`,
	})
	withBaseURL(t, srv.URL)

	c := nonRefreshingClient(t)
	res, err := c.ListInboxThreads(context.Background(), 25, "")
	if err != nil {
		t.Fatalf("ListInboxThreads: %v", err)
	}
	if len(res.Threads) != 3 {
		t.Fatalf("Threads len = %d want 3", len(res.Threads))
	}
	if res.Threads[0].ID != "t1" {
		t.Fatalf("first thread id = %q want t1", res.Threads[0].ID)
	}
	if res.NextPageToken != "tok2" {
		t.Fatalf("NextPageToken = %q want tok2", res.NextPageToken)
	}
	if res.ResultSizeEstimate != 142 {
		t.Fatalf("ResultSizeEstimate = %d want 142", res.ResultSizeEstimate)
	}
}

// TestListInboxThreads_PageSizeClamped asserts the helper clamps pageSize
// to Gmail's hard cap (500) and translates non-positive values to a 25
// default.
func TestListInboxThreads_PageSizeClamped(t *testing.T) {
	var observedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observedQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"threads":[]}`))
	}))
	defer srv.Close()
	withBaseURL(t, srv.URL)

	c := nonRefreshingClient(t)
	_, err := c.ListInboxThreads(context.Background(), 9999, "")
	if err != nil {
		t.Fatalf("ListInboxThreads: %v", err)
	}
	if !strings.Contains(observedQuery, "maxResults=500") {
		t.Fatalf("9999 should clamp to 500, query was: %s", observedQuery)
	}

	_, err = c.ListInboxThreads(context.Background(), -1, "")
	if err != nil {
		t.Fatalf("ListInboxThreads: %v", err)
	}
	if !strings.Contains(observedQuery, "maxResults=25") {
		t.Fatalf("-1 should default to 25, query was: %s", observedQuery)
	}
}

func TestListMessages_LabelQueryAndPageToken(t *testing.T) {
	var observedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observedQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{
			"messages": [{"id":"m1","threadId":"t1"}],
			"nextPageToken": "next-1",
			"resultSizeEstimate": 10
		}`))
	}))
	defer srv.Close()
	withBaseURL(t, srv.URL)

	c := nonRefreshingClient(t)
	res, err := c.ListMessages(context.Background(), []string{"INBOX", "STARRED"}, "from:alice@example.com", 999, "page-1")
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	for _, want := range []string{"labelIds=INBOX", "labelIds=STARRED", "maxResults=500", "pageToken=page-1", "q=from%3Aalice%40example.com"} {
		if !strings.Contains(observedQuery, want) {
			t.Fatalf("query %q missing %q", observedQuery, want)
		}
	}
	if len(res.Messages) != 1 || res.Messages[0].ID != "m1" {
		t.Fatalf("messages = %+v", res.Messages)
	}
	if res.NextPageToken != "next-1" || res.ResultSizeEstimate != 10 {
		t.Fatalf("pagination fields wrong: %+v", res)
	}
}

func TestListThreads_LabelQueryAndPageToken(t *testing.T) {
	var observedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observedQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{
			"threads": [{"id":"t1","historyId":"h1"}],
			"nextPageToken": "next-1",
			"resultSizeEstimate": 5
		}`))
	}))
	defer srv.Close()
	withBaseURL(t, srv.URL)

	c := nonRefreshingClient(t)
	res, err := c.ListThreads(context.Background(), []string{"SENT"}, "in:anywhere -label:inbox", 999, "page-1")
	if err != nil {
		t.Fatalf("ListThreads: %v", err)
	}
	for _, want := range []string{"labelIds=SENT", "maxResults=500", "pageToken=page-1", "q=in%3Aanywhere+-label%3Ainbox"} {
		if !strings.Contains(observedQuery, want) {
			t.Fatalf("query %q missing %q", observedQuery, want)
		}
	}
	if len(res.Threads) != 1 || res.Threads[0].ID != "t1" || res.NextPageToken != "next-1" || res.ResultSizeEstimate != 5 {
		t.Fatalf("threads result wrong: %+v", res)
	}
}

// TestGetMessage_DecodesBodyAndAttachments exercises the full message-tree
// walk: text/plain, text/html, and one attachment with id/size.
func TestGetMessage_DecodesBodyAndAttachments(t *testing.T) {
	plain := base64.URLEncoding.EncodeToString([]byte("Hello, plain world!"))
	plain = strings.TrimRight(plain, "=")
	html := base64.URLEncoding.EncodeToString([]byte("<p>Hello, <b>html</b> world!</p>"))
	html = strings.TrimRight(html, "=")

	fixture := map[string]any{
		"id":           "msg-id-1",
		"threadId":     "thread-id-1",
		"labelIds":     []string{"INBOX", "UNREAD"},
		"snippet":      "Hello, plain world!",
		"historyId":    "h1",
		"internalDate": "1700000000000",
		"payload": map[string]any{
			"partId":   "",
			"mimeType": "multipart/mixed",
			"headers": []map[string]string{
				{"name": "From", "value": "alice@example.com"},
				{"name": "Subject", "value": "Test"},
			},
			"parts": []map[string]any{
				{
					"partId":   "0",
					"mimeType": "text/plain",
					"body":     map[string]any{"size": 19, "data": plain},
				},
				{
					"partId":   "1",
					"mimeType": "text/html",
					"body":     map[string]any{"size": 31, "data": html},
				},
				{
					"partId":   "2",
					"mimeType": "image/png",
					"filename": "logo.png",
					"body":     map[string]any{"attachmentId": "att-1", "size": 4096},
				},
			},
		},
	}
	body, _ := json.Marshal(fixture)
	srv := newSimpleSrv(t, map[string]string{
		"/users/me/messages/msg-id-1": string(body),
	})
	withBaseURL(t, srv.URL)

	c := nonRefreshingClient(t)
	msg, err := c.GetMessage(context.Background(), "msg-id-1", "full")
	if err != nil {
		t.Fatalf("GetMessage: %v", err)
	}
	if msg.ID != "msg-id-1" || msg.ThreadID != "thread-id-1" {
		t.Fatalf("IDs wrong: %+v", msg)
	}
	if msg.Body != "Hello, plain world!" {
		t.Fatalf("Body = %q", msg.Body)
	}
	if msg.HTMLBody != "<p>Hello, <b>html</b> world!</p>" {
		t.Fatalf("HTMLBody = %q", msg.HTMLBody)
	}
	if len(msg.Attachments) != 1 {
		t.Fatalf("Attachments len = %d want 1", len(msg.Attachments))
	}
	att := msg.Attachments[0]
	if att.AttachmentID != "att-1" || att.Filename != "logo.png" || att.MimeType != "image/png" || att.Size != 4096 {
		t.Fatalf("attachment metadata wrong: %+v", att)
	}
	// Headers should have both entries.
	if len(msg.Headers) != 2 {
		t.Fatalf("Headers len = %d want 2", len(msg.Headers))
	}
	if msg.InternalDate != 1700000000000 {
		t.Fatalf("InternalDate parsed wrong: %d", msg.InternalDate)
	}
}

// TestGetMessage_IDRequired surfaces the validation gate.
func TestGetMessage_IDRequired(t *testing.T) {
	c := nonRefreshingClient(t)
	_, err := c.GetMessage(context.Background(), "", "full")
	if err == nil {
		t.Fatalf("expected error for empty id, got nil")
	}
	if !strings.Contains(err.Error(), "id is required") {
		t.Fatalf("error %q missing 'id is required'", err.Error())
	}
}

// TestGetAttachment_SizeMismatchReturnsError catches truncated responses
// that would otherwise silently write a partial file.
func TestGetAttachment_SizeMismatchReturnsError(t *testing.T) {
	encoded := base64.URLEncoding.EncodeToString([]byte("only 13 bytes"))
	encoded = strings.TrimRight(encoded, "=")
	fixture := map[string]any{
		"size": 99999,
		"data": encoded,
	}
	body, _ := json.Marshal(fixture)
	srv := newSimpleSrv(t, map[string]string{
		"/attachments/att-1": string(body),
	})
	withBaseURL(t, srv.URL)

	c := nonRefreshingClient(t)
	_, err := c.GetAttachment(context.Background(), "msg-id-1", "att-1")
	if err == nil {
		t.Fatalf("expected size-mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "size mismatch") {
		t.Fatalf("error %q missing 'size mismatch'", err.Error())
	}
}

// TestGetAttachment_HappyPath round-trips a small payload.
func TestGetAttachment_HappyPath(t *testing.T) {
	want := []byte("hello attachment world")
	encoded := base64.URLEncoding.EncodeToString(want)
	encoded = strings.TrimRight(encoded, "=")
	fixture := map[string]any{"size": len(want), "data": encoded}
	body, _ := json.Marshal(fixture)
	srv := newSimpleSrv(t, map[string]string{
		"/attachments/att-1": string(body),
	})
	withBaseURL(t, srv.URL)

	c := nonRefreshingClient(t)
	att, err := c.GetAttachment(context.Background(), "msg-id-1", "att-1")
	if err != nil {
		t.Fatalf("GetAttachment: %v", err)
	}
	if string(att.Data) != string(want) {
		t.Fatalf("Data = %q want %q", att.Data, want)
	}
	if att.Size != len(want) {
		t.Fatalf("Size = %d want %d", att.Size, len(want))
	}
}

// TestModifyMessageLabels_HappyPath verifies the POST body shape and the
// returned label-set.
func TestModifyMessageLabels_HappyPath(t *testing.T) {
	var received string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		received = string(buf[:n])
		_, _ = w.Write([]byte(`{"labelIds":["UNREAD"]}`))
	}))
	defer srv.Close()
	withBaseURL(t, srv.URL)

	c := nonRefreshingClient(t)
	out, err := c.ModifyMessageLabels(context.Background(), "msg-id-1", nil, []string{"INBOX"})
	if err != nil {
		t.Fatalf("ModifyMessageLabels: %v", err)
	}
	if len(out) != 1 || out[0] != "UNREAD" {
		t.Fatalf("returned labels = %v want [UNREAD]", out)
	}
	if !strings.Contains(received, `"removeLabelIds":["INBOX"]`) {
		t.Fatalf("body missing removeLabelIds: %s", received)
	}
}

// TestModifyMessageLabels_RequiresAtLeastOneSide gates the empty-call edge.
func TestModifyMessageLabels_RequiresAtLeastOneSide(t *testing.T) {
	c := nonRefreshingClient(t)
	_, err := c.ModifyMessageLabels(context.Background(), "msg-id-1", nil, nil)
	if err == nil {
		t.Fatalf("expected error for empty add+remove")
	}
}

// TestListLabels_SortsSystemFirstThenUserAlphabetical asserts the stable
// ordering contract. Gmail's underlying order varies; the helper enforces
// system-first + alphabetical user labels.
func TestListLabels_SortsSystemFirstThenUserAlphabetical(t *testing.T) {
	// Intentionally shuffled to confirm the sort actually runs.
	fixture := `{
		"labels": [
			{"id":"Label_1","name":"Travel","type":"user"},
			{"id":"INBOX","name":"INBOX","type":"system"},
			{"id":"Label_2","name":"Newsletters","type":"user"},
			{"id":"SPAM","name":"SPAM","type":"system"},
			{"id":"Label_3","name":"Archive","type":"user"}
		]
	}`
	srv := newSimpleSrv(t, map[string]string{"/users/me/labels": fixture})
	withBaseURL(t, srv.URL)

	c := nonRefreshingClient(t)
	labels, err := c.ListLabels(context.Background())
	if err != nil {
		t.Fatalf("ListLabels: %v", err)
	}
	if len(labels) != 5 {
		t.Fatalf("labels len = %d want 5", len(labels))
	}
	// Index 0-1: system (Gmail order preserved).
	if labels[0].Type != "system" || labels[1].Type != "system" {
		t.Fatalf("first two should be system, got %v %v", labels[0], labels[1])
	}
	// Index 2-4: user, alphabetical: Archive, Newsletters, Travel.
	if labels[2].Name != "Archive" || labels[3].Name != "Newsletters" || labels[4].Name != "Travel" {
		t.Fatalf("user labels not alphabetical: %v", labels[2:])
	}
}
