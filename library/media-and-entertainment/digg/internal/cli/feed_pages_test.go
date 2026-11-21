// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// feedPagesStub is a fake digg_get_feed. pages maps a request's cursor ("" for
// the first page) to a reply: a structuredContent JSON string, or a
// func that writes the whole response.
type feedPagesStub struct {
	mu    sync.Mutex
	pages map[string]any
	args  []map[string]any
}

func (s *feedPagesStub) serve(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		if err := json.Unmarshal(body, &req); err != nil || req.Params.Name != diggGetFeedTool {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		s.args = append(s.args, req.Params.Arguments)
		cursor, _ := req.Params.Arguments["cursor"].(string)
		reply, ok := s.pages[cursor]
		s.mu.Unlock()
		if !ok {
			http.Error(w, "no page for cursor "+cursor, http.StatusBadRequest)
			return
		}
		if fn, ok := reply.(func(http.ResponseWriter)); ok {
			fn(w)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"structuredContent\":%s}}\n\n", reply.(string))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func (s *feedPagesStub) requests() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]map[string]any(nil), s.args...)
}

// feedPost renders one post the way digg_get_feed sends it, tldr and
// description included, so tests can check they are dropped.
func feedPagePostJSON(id, published string) string {
	return fmt.Sprintf(`{"id":%q,"url":"https://digg.com/ai/%s","topic":"ai","title":"Post %s",`+
		`"description":"secret description","tldr":"secret tldr",`+
		`"feedEntryAt":%q,"publishedAt":%q,"updatedAt":%q}`, id, id, id, published, published, published)
}

func feedPageJSON(next string, posts ...string) string {
	nc := "null"
	if next != "" {
		nc = fmt.Sprintf("%q", next)
	}
	return fmt.Sprintf(`{"topic":"ai","posts":[%s],"nextCursor":%s,"asOf":"2026-11-21T00:00:00Z","view":"newest","range":null,"unavailableCount":0}`,
		strings.Join(posts, ","), nc)
}

// decodeFeedLines splits stdout into post lines and the final cursor line.
func decodeFeedLines(t *testing.T, stdout string) ([]map[string]any, map[string]any) {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	var posts []map[string]any
	var end map[string]any
	for i, line := range lines {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("line %d is not JSON: %q", i, line)
		}
		switch m["kind"] {
		case "post":
			if end != nil {
				t.Fatalf("post line after the cursor line: %q", line)
			}
			posts = append(posts, m)
		case "cursor":
			end = m
		default:
			t.Fatalf("line %d has unknown kind: %q", i, line)
		}
	}
	if end == nil {
		t.Fatalf("no final cursor line in %q", stdout)
	}
	return posts, end
}

func postIDs(posts []map[string]any) []string {
	var ids []string
	for _, p := range posts {
		ids = append(ids, p["id"].(string))
	}
	return ids
}

func TestFeedResumesFromCursorAndPrintsNext(t *testing.T) {
	stub := &feedPagesStub{pages: map[string]any{
		"c1": feedPageJSON("c2", feedPagePostJSON("a", "2026-11-20T10:00:00Z")),
	}}
	out, _, err := runDiggCmd(t, stub.serve(t), "feed", "--topic", "ai", "--cursor", "c1", "--max-pages", "1")
	if err != nil {
		t.Fatalf("feed: %v", err)
	}
	reqs := stub.requests()
	if len(reqs) != 1 || reqs[0]["cursor"] != "c1" {
		t.Fatalf("first request should carry --cursor c1, got %v", reqs)
	}
	if reqs[0]["view"] != "newest" || reqs[0]["topic"] != "ai" || reqs[0]["limit"] != float64(feedPageSize) {
		t.Fatalf("unexpected request args %v", reqs[0])
	}
	posts, end := decodeFeedLines(t, out)
	if len(posts) != 1 {
		t.Fatalf("want 1 post, got %d", len(posts))
	}
	if end["nextCursor"] != "c2" || end["stop"] != "max-pages" || end["pages"] != float64(1) {
		t.Fatalf("final line should name c2 after max-pages, got %v", end)
	}
}

func TestFeedStopsPagingAtSince(t *testing.T) {
	stub := &feedPagesStub{pages: map[string]any{
		"":   feedPageJSON("p2", feedPagePostJSON("a", "2026-11-20T12:00:00Z"), feedPagePostJSON("b", "2026-11-20T11:00:00Z")),
		"p2": feedPageJSON("p3", feedPagePostJSON("c", "2026-11-20T08:00:00Z")),
		"p3": feedPageJSON("p4", feedPagePostJSON("d", "2026-11-20T02:00:00Z"), feedPagePostJSON("e", "2026-11-19T23:00:00Z")),
		"p4": feedPageJSON("p5", feedPagePostJSON("f", "2026-11-19T20:00:00Z")),
	}}
	out, _, err := runDiggCmd(t, stub.serve(t), "feed", "--topic", "ai", "--since", "2026-11-20T00:00:00Z", "--max-pages", "10")
	if err != nil {
		t.Fatalf("feed: %v", err)
	}
	if n := len(stub.requests()); n != 3 {
		t.Fatalf("paging should stop after the page that crosses --since: %d requests", n)
	}
	posts, end := decodeFeedLines(t, out)
	if got := strings.Join(postIDs(posts), ","); got != "a,b,c,d" {
		t.Fatalf("want only posts newer than --since (a,b,c,d), got %s", got)
	}
	if end["stop"] != "since" || end["nextCursor"] != "p4" || end["posts"] != float64(4) {
		t.Fatalf("unexpected final line %v", end)
	}
}

func TestFeedTopViewSinceFiltersWithoutStopping(t *testing.T) {
	stub := &feedPagesStub{pages: map[string]any{
		"":   feedPageJSON("p2", feedPagePostJSON("old", "2026-10-01T00:00:00Z"), feedPagePostJSON("new", "2026-11-20T00:00:00Z")),
		"p2": feedPageJSON("", feedPagePostJSON("new2", "2026-11-19T00:00:00Z")),
	}}
	out, _, err := runDiggCmd(t, stub.serve(t), "feed", "--topic", "ai", "--view", "top", "--range", "30d", "--since", "2026-11-01")
	if err != nil {
		t.Fatalf("feed: %v", err)
	}
	reqs := stub.requests()
	if len(reqs) != 2 || reqs[0]["range"] != "30d" || reqs[0]["view"] != "top" {
		t.Fatalf("top view should page on past older posts with range 30d: %v", reqs)
	}
	posts, end := decodeFeedLines(t, out)
	if got := strings.Join(postIDs(posts), ","); got != "new,new2" {
		t.Fatalf("want new,new2, got %s", got)
	}
	if end["stop"] != "end" || end["nextCursor"] != nil {
		t.Fatalf("feed end should give a null cursor: %v", end)
	}
}

func TestFeedDedupesByIDAcrossPages(t *testing.T) {
	stub := &feedPagesStub{pages: map[string]any{
		"":   feedPageJSON("p2", feedPagePostJSON("a", "2026-11-20T12:00:00Z"), feedPagePostJSON("b", "2026-11-20T11:00:00Z")),
		"p2": feedPageJSON("", feedPagePostJSON("b", "2026-11-20T11:00:00Z"), feedPagePostJSON("c", "2026-11-20T10:00:00Z")),
	}}
	out, _, err := runDiggCmd(t, stub.serve(t), "feed", "--topic", "ai")
	if err != nil {
		t.Fatalf("feed: %v", err)
	}
	posts, end := decodeFeedLines(t, out)
	if got := strings.Join(postIDs(posts), ","); got != "a,b,c" {
		t.Fatalf("a post repeated across pages must print once: %s", got)
	}
	if end["posts"] != float64(3) {
		t.Fatalf("posts count should match printed posts: %v", end)
	}
}

func TestFeed429ExitsWithResetAndLastGoodCursorWithoutRetry(t *testing.T) {
	stub := &feedPagesStub{pages: map[string]any{
		"": feedPageJSON("p2", feedPagePostJSON("a", "2026-11-20T12:00:00Z")),
		"p2": func(w http.ResponseWriter) {
			w.Header().Set("Retry-After", "Sat, 21 Nov 2026 20:15:00 GMT")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"Too many requests"}}`)
		},
	}}
	out, stderr, err := runDiggCmd(t, stub.serve(t), "feed", "--topic", "ai")
	if err == nil {
		t.Fatal("a 429 must fail the command")
	}
	if code := ExitCode(err); code != 7 {
		t.Fatalf("a 429 must exit with the rate-limit code 7, got %d (%v)", code, err)
	}
	if n := len(stub.requests()); n != 2 {
		t.Fatalf("a 429 must not be retried: %d requests", n)
	}
	if !strings.Contains(err.Error(), "2026-11-21T20:15:00Z") {
		t.Fatalf("error should name the reset time: %v (stderr %q)", err, stderr)
	}
	posts, end := decodeFeedLines(t, out)
	if len(posts) != 1 {
		t.Fatalf("the page before the refusal should still print: %d posts", len(posts))
	}
	if end["stop"] != "rate-limited" || end["nextCursor"] != "p2" || end["resetAt"] != "2026-11-21T20:15:00Z" {
		t.Fatalf("final line should carry the refused page's cursor and reset time: %v", end)
	}
}

func TestFeedToolRefusalNamesResetTime(t *testing.T) {
	stub := &feedPagesStub{pages: map[string]any{
		"c9": func(w http.ResponseWriter) {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, `event: message`+"\n"+`data: {"jsonrpc":"2.0","id":1,"result":{"isError":true,"content":[{"type":"text","text":"Digg MCP is temporarily unavailable. Retry after 2026-11-21T20:00:00.750Z"}]}}`+"\n\n")
		},
	}}
	out, _, err := runDiggCmd(t, stub.serve(t), "feed", "--topic", "ai", "--cursor", "c9")
	if code := ExitCode(err); err == nil || code != 7 {
		t.Fatalf("a tool refusal must exit 7, got %d (%v)", code, err)
	}
	if n := len(stub.requests()); n != 1 {
		t.Fatalf("a refusal must not be retried: %d requests", n)
	}
	_, end := decodeFeedLines(t, out)
	// Sub-second precision is kept so a collector never retries early.
	if end["resetAt"] != "2026-11-21T20:00:00.75Z" || end["nextCursor"] != "c9" || end["stop"] != "rate-limited" {
		t.Fatalf("unexpected final line %v", end)
	}
}

func TestFeedNeverPrintsTldrOrDescription(t *testing.T) {
	stub := &feedPagesStub{pages: map[string]any{
		"": feedPageJSON("", feedPagePostJSON("a", "2026-11-20T12:00:00Z")),
	}}
	out, _, err := runDiggCmd(t, stub.serve(t), "feed", "--topic", "ai", "--agent")
	if err != nil {
		t.Fatalf("feed: %v", err)
	}
	if strings.Contains(out, "tldr") || strings.Contains(out, "description") || strings.Contains(out, "secret") {
		t.Fatalf("output must not carry tldr or description: %s", out)
	}
	posts, _ := decodeFeedLines(t, out)
	for _, key := range []string{"id", "url", "topic", "title", "publishedAt", "updatedAt", "diggScore", "sourceCount", "uniqueAuthors", "firstSourceAt"} {
		if _, ok := posts[0][key]; !ok {
			t.Fatalf("post line is missing %q: %v", key, posts[0])
		}
	}
}

func TestFeedRejectsBadFlagsBeforeAnyRequest(t *testing.T) {
	stub := &feedPagesStub{pages: map[string]any{}}
	url := stub.serve(t)
	cases := [][]string{
		{"feed", "--range", "7d"},
		{"feed", "--view", "trending"},
		{"feed", "--topic", "nope"},
		{"feed", "--max-pages", "0"},
		{"feed", "--since", "yesterday"},
	}
	for _, args := range cases {
		_, _, err := runDiggCmd(t, url, args...)
		if code := ExitCode(err); err == nil || code != 2 {
			t.Errorf("%v: want usage exit 2, got %d (%v)", args, code, err)
		}
	}
	if n := len(stub.requests()); n != 0 {
		t.Fatalf("bad flags must not reach Digg: %d requests", n)
	}
}

func TestFeedPageMissingFieldsIsAnErrorNotTheEnd(t *testing.T) {
	stub := &feedPagesStub{pages: map[string]any{
		"":   feedPageJSON("p2", feedPagePostJSON("a", "2026-11-20T12:00:00Z")),
		"p2": `{"topic":"ai"}`,
	}}
	out, _, err := runDiggCmd(t, stub.serve(t), "feed", "--topic", "ai")
	if code := ExitCode(err); err == nil || code != 5 {
		t.Fatalf("a page without posts or nextCursor must fail with exit 5, got %d (%v)", code, err)
	}
	posts, end := decodeFeedLines(t, out)
	if len(posts) != 1 || end["stop"] != "error" || end["nextCursor"] != "p2" {
		t.Fatalf("want the first page's post and stop error at cursor p2, got %d posts, %v", len(posts), end)
	}
}

func TestFeedDeliversToFileEvenWhenRefused(t *testing.T) {
	stub := &feedPagesStub{pages: map[string]any{
		"": feedPageJSON("p2", feedPagePostJSON("a", "2026-11-20T12:00:00Z")),
		"p2": func(w http.ResponseWriter) {
			w.Header().Set("Retry-After", "120")
			w.WriteHeader(http.StatusTooManyRequests)
		},
	}}
	dest := filepath.Join(t.TempDir(), "feed.jsonl")
	_, _, err := runDiggCmd(t, stub.serve(t), "feed", "--topic", "ai", "--deliver", "file:"+dest)
	if code := ExitCode(err); err == nil || code != 7 {
		t.Fatalf("want exit 7, got %d (%v)", code, err)
	}
	data, rerr := os.ReadFile(dest)
	if rerr != nil {
		t.Fatalf("--deliver file was not written on a refusal: %v", rerr)
	}
	posts, end := decodeFeedLines(t, string(data))
	if len(posts) != 1 || end["stop"] != "rate-limited" || end["nextCursor"] != "p2" {
		t.Fatalf("delivered file should hold the post and the resume line: %s", data)
	}
}

func TestFeedWebhookDeliveryIsNDJSON(t *testing.T) {
	var mu sync.Mutex
	var contentType string
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		mu.Lock()
		contentType = r.Header.Get("Content-Type")
		mu.Unlock()
	}))
	t.Cleanup(hook.Close)
	stub := &feedPagesStub{pages: map[string]any{
		"": feedPageJSON("", feedPagePostJSON("a", "2026-11-20T12:00:00Z")),
	}}
	if _, _, err := runDiggCmd(t, stub.serve(t), "feed", "--topic", "ai", "--deliver", "webhook:"+hook.URL); err != nil {
		t.Fatalf("feed: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if contentType != "application/x-ndjson" {
		t.Fatalf("feed webhook body is JSON lines; want application/x-ndjson, got %q", contentType)
	}
}
