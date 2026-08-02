// Copyright 2026 Rahul Bansal and contributors. Licensed under Apache-2.0. See LICENSE.
package gmailmail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/textproto"
	"strings"
	"testing"
)

// fakeSender answers batch POSTs with a canned multipart response echoing one
// message per requested id.
type fakeSender struct {
	calls     int
	lastPath  string
	lastCT    string
	fail      bool
	perIDFail map[string]bool
	// throttleUntilAttempt[id] = N means the id 429s until the Nth call.
	throttleUntilAttempt map[string]int
}

func (f *fakeSender) SendRaw(_ context.Context, method, path string, _ map[string]string, body []byte, contentType string, _ map[string]string) (json.RawMessage, int, error) {
	f.calls++
	f.lastPath = path
	f.lastCT = contentType
	if f.fail {
		return nil, 500, fmt.Errorf("boom")
	}
	// Recover requested ids from the outbound multipart body.
	var ids []string
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "GET /gmail/v1/users/me/messages/") {
			rest := strings.TrimPrefix(line, "GET /gmail/v1/users/me/messages/")
			id, _, _ := strings.Cut(rest, "?")
			ids = append(ids, id)
		}
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for i, id := range ids {
		ph := textproto.MIMEHeader{}
		ph.Set("Content-Type", "application/http")
		part, _ := w.CreatePart(ph)
		if f.perIDFail[id] {
			fmt.Fprintf(part, "HTTP/1.1 404 Not Found\r\nContent-Type: application/json\r\n\r\n{\"error\":{\"code\":404}}")
			continue
		}
		if until, ok := f.throttleUntilAttempt[id]; ok && f.calls < until {
			fmt.Fprintf(part, "HTTP/1.1 429 Too Many Requests\r\nContent-Type: application/json\r\n\r\n{\"error\":{\"code\":429}}")
			continue
		}
		fmt.Fprintf(part, "HTTP/1.1 200 OK\r\nContent-Type: application/json; charset=UTF-8\r\n\r\n{\"id\":%q,\"threadId\":\"t%d\",\"sizeEstimate\":100}", id, i)
	}
	w.Close()
	return json.RawMessage(buf.Bytes()), 200, nil
}

func TestBatchGetMessagesRoundTrip(t *testing.T) {
	f := &fakeSender{}
	msgs, skipped, err := BatchGetMessages(context.Background(), f, []string{"a", "b", "c"}, "metadata", DefaultMetadataHeaders)
	if err != nil {
		t.Fatalf("BatchGetMessages error = %v", err)
	}
	if skipped != 0 {
		t.Fatalf("skipped = %d, want 0", skipped)
	}
	if len(msgs) != 3 {
		t.Fatalf("len(msgs) = %d, want 3", len(msgs))
	}
	if f.lastPath != "/batch/gmail/v1" {
		t.Fatalf("path = %q", f.lastPath)
	}
	if !strings.HasPrefix(f.lastCT, "multipart/mixed; boundary=") {
		t.Fatalf("content type = %q", f.lastCT)
	}
	got := map[string]bool{}
	for _, m := range msgs {
		got[m.ID] = true
	}
	for _, id := range []string{"a", "b", "c"} {
		if !got[id] {
			t.Fatalf("missing message %q in %v", id, got)
		}
	}
}

func TestBatchGetMessagesChunks(t *testing.T) {
	f := &fakeSender{}
	ids := make([]string, 95)
	for i := range ids {
		ids[i] = fmt.Sprintf("id%02d", i)
	}
	msgs, _, err := BatchGetMessages(context.Background(), f, ids, "metadata", nil)
	if err != nil {
		t.Fatalf("BatchGetMessages error = %v", err)
	}
	if len(msgs) != 95 {
		t.Fatalf("len(msgs) = %d, want 95", len(msgs))
	}
	wantCalls := (95 + batchChunkSize - 1) / batchChunkSize
	if f.calls != wantCalls {
		t.Fatalf("calls = %d, want %d chunks of %d", f.calls, wantCalls, batchChunkSize)
	}
}

func TestBatchGetMessagesSkipsPerItemFailures(t *testing.T) {
	f := &fakeSender{perIDFail: map[string]bool{"bad": true}}
	msgs, skipped, err := BatchGetMessages(context.Background(), f, []string{"good", "bad"}, "metadata", nil)
	if err != nil {
		t.Fatalf("BatchGetMessages error = %v", err)
	}
	// "good" may be returned once per retry round; the id set is what matters.
	seen := map[string]bool{}
	for _, m := range msgs {
		seen[m.ID] = true
	}
	if !seen["good"] || seen["bad"] {
		t.Fatalf("msgs = %+v, want good present and bad absent", msgs)
	}
	if skipped != 1 {
		t.Fatalf("skipped = %d, want 1", skipped)
	}
}

// The batch endpoint runs every sub-request at once against the per-user
// quota, so a chunk can throttle itself. Throttled ids must be retried, never
// silently reported as missing data.
func TestBatchGetMessagesRetriesThrottledParts(t *testing.T) {
	f := &fakeSender{throttleUntilAttempt: map[string]int{"slow": 2}}
	msgs, skipped, err := BatchGetMessages(context.Background(), f, []string{"fast", "slow"}, "metadata", nil)
	if err != nil {
		t.Fatalf("BatchGetMessages error = %v", err)
	}
	if skipped != 0 {
		t.Fatalf("skipped = %d, want 0 after retries", skipped)
	}
	seen := map[string]bool{}
	for _, m := range msgs {
		seen[m.ID] = true
	}
	if !seen["slow"] {
		t.Fatalf("throttled id was never recovered: %+v", msgs)
	}
}

func TestIsRetryableStatus(t *testing.T) {
	for _, s := range []int{429, 403, 500, 503} {
		if !isRetryableStatus(s) {
			t.Fatalf("status %d should be retryable", s)
		}
	}
	for _, s := range []int{200, 400, 404} {
		if isRetryableStatus(s) {
			t.Fatalf("status %d should not be retryable", s)
		}
	}
}

func TestBatchGetMessagesTransportError(t *testing.T) {
	f := &fakeSender{fail: true}
	if _, _, err := BatchGetMessages(context.Background(), f, []string{"a"}, "metadata", nil); err == nil {
		t.Fatal("expected transport error")
	}
}

func TestParseBatchResponseRejectsGarbage(t *testing.T) {
	if _, _, err := ParseBatchResponse([]byte("not multipart at all")); err == nil {
		t.Fatal("expected error for non-multipart body")
	}
}
