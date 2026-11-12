// Tests for the commands that act on one story: `posts`, `story`, `open`
// and `watch`. `posts` and `story` read Digg's digg_get_post tool; each
// test points them at an httptest stub of Digg's MCP server via
// diggMCPURLOverride.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	_ "modernc.org/sqlite"
)

const (
	sportsStoryID    = "11111111-2222-7333-8444-555555555555"
	sportsStoryShort = "ws7game1"
)

// stubSource is one source of a stub story. An empty handle makes a
// webpage source.
type stubSource struct {
	handle, author, text, title, url, publishedAt string
}

func (s stubSource) json(i int) map[string]any {
	if s.handle == "" {
		return map[string]any{
			"id": fmt.Sprintf("src-%d", i), "kind": "webpage", "url": s.url, "author": nil,
			"title": s.title, "text": s.text, "publishedAt": s.publishedAt,
		}
	}
	return map[string]any{
		"id": fmt.Sprintf("src-%d", i), "kind": "x",
		"url":    fmt.Sprintf("https://x.com/%s/status/%d", s.handle, 9000+i),
		"author": s.author, "title": nil, "text": s.text, "publishedAt": s.publishedAt,
	}
}

// xSources returns n X sources with distinct handles, authors and text.
func xSources(n int) []stubSource {
	out := make([]stubSource, n)
	for i := range out {
		out[i] = stubSource{
			handle: fmt.Sprintf("fan%02d", i), author: fmt.Sprintf("Fan Number %d", i),
			text:        fmt.Sprintf("Take %d on the game", i),
			publishedAt: fmt.Sprintf("2026-11-10T%02d:00:00.000Z", i%24),
		}
	}
	return out
}

// storyStub is a stand-in for Digg's MCP server that knows one story and
// serves its sources a page at a time, as digg_get_post does.
type storyStub struct {
	id, shortID, topic string
	sources            []stubSource
	mu                 sync.Mutex
	calls              []map[string]any
}

func sportsStub(sources []stubSource) *storyStub {
	return &storyStub{id: sportsStoryID, shortID: sportsStoryShort, topic: "sports", sources: sources}
}

func (s *storyStub) serve(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		args := req.Params.Arguments
		s.mu.Lock()
		s.calls = append(s.calls, args)
		s.mu.Unlock()

		var result map[string]any
		if req.Params.Name != "digg_get_post" || args["id"] != s.id {
			result = map[string]any{"isError": true, "content": []map[string]any{{"type": "text", "text": "Post not found"}}}
		} else {
			result = map[string]any{"structuredContent": s.page(args)}
		}
		reply, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result})
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "event: message\ndata: %s\n\n", reply)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func (s *storyStub) page(args map[string]any) map[string]any {
	offset, limit := 0, 10
	if v, ok := args["sourceOffset"].(float64); ok {
		offset = int(v)
	}
	if v, ok := args["sourceLimit"].(float64); ok {
		limit = int(v)
	}
	end := min(offset+limit, len(s.sources))
	sources := []map[string]any{}
	for i := min(offset, end); i < end; i++ {
		sources = append(sources, s.sources[i].json(i))
	}
	var next any
	if end < len(s.sources) {
		next = end
	}
	return map[string]any{
		"story": map[string]any{
			"id": s.id, "url": "https://digg.com/" + s.topic + "/" + s.shortID, "topic": s.topic,
			"title": "Game one goes to extra innings", "description": "A long night at the ballpark.",
			"tldr":        "The opener ran eleven innings.",
			"feedEntryAt": "2026-11-10T02:00:00.000Z", "publishedAt": "2026-11-10T02:05:00.000Z", "updatedAt": "2026-11-10T03:00:00.000Z",
		},
		"follows": nil, "newer": nil,
		"sources": sources, "nextSourceOffset": next, "sourceLimitReached": false,
	}
}

func (s *storyStub) requests() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]map[string]any(nil), s.calls...)
}

// runDiggCmd runs the CLI with Digg's MCP endpoint pointed at mcpURL.
func runDiggCmd(t *testing.T, mcpURL string, args ...string) (string, string, error) {
	t.Helper()
	prev := diggMCPURLOverride
	diggMCPURLOverride = mcpURL
	t.Cleanup(func() { diggMCPURLOverride = prev })

	var flags rootFlags
	root := newRootCmd(&flags)
	root.SetArgs(args)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

// postsOutput is the printed `posts` envelope. Rows decode loosely so
// tests can assert which keys are present.
type postsOutput struct {
	Meta    map[string]any   `json:"meta"`
	Results []map[string]any `json:"results"`
}

func decodePosts(t *testing.T, stdout string) postsOutput {
	t.Helper()
	var env postsOutput
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	return env
}

// recordSportsStory runs a search that returns the sports story, which
// records its short id, UUID and topic in the local store.
func recordSportsStory(t *testing.T) {
	t.Helper()
	search := &searchStub{posts: []stubPost{{id: sportsStoryID, shortID: sportsStoryShort, topic: "sports", title: "Game one", score: 80}}}
	if _, stderr, err := runSearchCmd(t, search.serve(t), "world series", "--json"); err != nil {
		t.Fatalf("search: %v\n%s", err, stderr)
	}
}

// The call last30days 3.36.1 makes, given a UUID. That version keeps a
// row only when body, author.username and xUrl are all present.
func TestPostsByUUIDReturnsSourceText(t *testing.T) {
	withTempHome(t)
	stub := sportsStub(xSources(8))
	out, stderr, err := runDiggCmd(t, stub.serve(t), "posts", sportsStoryID, "--agent", "--by", "rank", "--limit", "5")
	if err != nil {
		t.Fatalf("posts: %v\n%s", err, stderr)
	}
	env := decodePosts(t, out)
	if len(env.Results) != 5 {
		t.Fatalf("got %d rows, want 5", len(env.Results))
	}
	for i, row := range env.Results {
		author, _ := row["author"].(map[string]any)
		wantHandle := fmt.Sprintf("fan%02d", i)
		if row["body"] != fmt.Sprintf("Take %d on the game", i) {
			t.Errorf("row %d body = %v", i, row["body"])
		}
		if author["username"] != wantHandle {
			t.Errorf("row %d author.username = %v, want %s", i, author["username"], wantHandle)
		}
		if author["display_name"] != fmt.Sprintf("Fan Number %d", i) {
			t.Errorf("row %d author.display_name = %v", i, author["display_name"])
		}
		if row["xUrl"] != fmt.Sprintf("https://x.com/%s/status/%d", wantHandle, 9000+i) {
			t.Errorf("row %d xUrl = %v", i, row["xUrl"])
		}
		if row["posted_at"] != fmt.Sprintf("2026-11-10T%02d:00:00.000Z", i) {
			t.Errorf("row %d posted_at = %v", i, row["posted_at"])
		}
		// Digg no longer reports these, so they are absent, not zero.
		for _, gone := range []string{"post_type"} {
			if _, ok := row[gone]; ok {
				t.Errorf("row %d still carries %s", i, gone)
			}
		}
		for _, gone := range []string{"rank", "category"} {
			if _, ok := author[gone]; ok {
				t.Errorf("row %d author still carries %s", i, gone)
			}
		}
	}
	if env.Meta["topic"] != "sports" || env.Meta["url"] != "https://digg.com/sports/"+sportsStoryShort {
		t.Errorf("meta = %v, want the sports topic and URL", env.Meta)
	}
}

func TestPostsByShortIDAfterSearch(t *testing.T) {
	withTempHome(t)
	recordSportsStory(t)
	stub := sportsStub(xSources(3))
	url := stub.serve(t)
	byShort, stderr, err := runDiggCmd(t, url, "posts", sportsStoryShort, "--agent")
	if err != nil {
		t.Fatalf("posts by short id: %v\n%s", err, stderr)
	}
	byUUID, _, err := runDiggCmd(t, url, "posts", sportsStoryID, "--agent")
	if err != nil {
		t.Fatal(err)
	}
	short, full := decodePosts(t, byShort), decodePosts(t, byUUID)
	if len(short.Results) != 3 {
		t.Fatalf("got %d rows by short id, want 3", len(short.Results))
	}
	a, _ := json.Marshal(short.Results)
	b, _ := json.Marshal(full.Results)
	if !bytes.Equal(a, b) {
		t.Errorf("rows differ by id form:\nshort id: %s\nUUID:     %s", a, b)
	}
}

func TestPostsByStoryURLAfterSearch(t *testing.T) {
	withTempHome(t)
	recordSportsStory(t)
	out, stderr, err := runDiggCmd(t, sportsStub(xSources(2)).serve(t), "posts", "https://digg.com/sports/"+sportsStoryShort, "--agent")
	if err != nil {
		t.Fatalf("posts by URL: %v\n%s", err, stderr)
	}
	if got := len(decodePosts(t, out).Results); got != 2 {
		t.Fatalf("got %d rows, want 2", got)
	}
}

func TestPostsUnknownShortIDNamesSearch(t *testing.T) {
	withTempHome(t)
	stub := sportsStub(xSources(3))
	out, _, err := runDiggCmd(t, stub.serve(t), "posts", "neverseen", "--agent")
	if err == nil {
		t.Fatalf("posts with an unrecorded short id succeeded:\n%s", out)
	}
	if !strings.Contains(err.Error(), "search") || !strings.Contains(err.Error(), "neverseen") {
		t.Errorf("error %q should name the id and the search command", err)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("stdout should be empty on failure, got:\n%s", out)
	}
	if n := len(stub.requests()); n != 0 {
		t.Errorf("made %d Digg requests for an id that cannot be resolved", n)
	}
}

func TestPostsWebpageSourceHasURLAndTitleButNoUsername(t *testing.T) {
	withTempHome(t)
	sources := []stubSource{
		{url: "https://example.com/recap", title: "Eleven innings, one hero", text: "The recap text.", publishedAt: "2026-11-10T05:00:00.000Z"},
		xSources(1)[0],
	}
	out, stderr, err := runDiggCmd(t, sportsStub(sources).serve(t), "posts", sportsStoryID, "--agent")
	if err != nil {
		t.Fatalf("posts: %v\n%s", err, stderr)
	}
	env := decodePosts(t, out)
	if len(env.Results) != 2 {
		t.Fatalf("got %d rows, want 2", len(env.Results))
	}
	page := env.Results[0]
	if page["xUrl"] != "https://example.com/recap" || page["title"] != "Eleven innings, one hero" || page["body"] != "The recap text." {
		t.Errorf("webpage row = %v", page)
	}
	if page["kind"] != "webpage" {
		t.Errorf("webpage row kind = %v", page["kind"])
	}
	author, _ := page["author"].(map[string]any)
	if _, ok := author["username"]; ok {
		t.Errorf("webpage row has author.username %v", author["username"])
	}
	if env.Results[1]["kind"] != "x" {
		t.Errorf("X row kind = %v", env.Results[1]["kind"])
	}
}

func TestPostsFollowsSourcePagesUpToLimit(t *testing.T) {
	withTempHome(t)
	stub := sportsStub(xSources(45))
	out, stderr, err := runDiggCmd(t, stub.serve(t), "posts", sportsStoryID, "--agent", "--limit", "33")
	if err != nil {
		t.Fatalf("posts: %v\n%s", err, stderr)
	}
	env := decodePosts(t, out)
	if len(env.Results) != 33 {
		t.Fatalf("got %d rows, want 33", len(env.Results))
	}
	seen := map[any]bool{}
	for _, row := range env.Results {
		seen[row["xUrl"]] = true
	}
	if len(seen) != 33 {
		t.Errorf("got %d distinct sources in 33 rows", len(seen))
	}
	for _, call := range stub.requests() {
		if l, _ := call["sourceLimit"].(float64); l > 20 {
			t.Errorf("asked Digg for %v sources in one page; its maximum is 20", l)
		}
	}
}

func TestPostsWithoutLimitReturnsEverySource(t *testing.T) {
	withTempHome(t)
	out, stderr, err := runDiggCmd(t, sportsStub(xSources(45)).serve(t), "posts", sportsStoryID, "--agent")
	if err != nil {
		t.Fatalf("posts: %v\n%s", err, stderr)
	}
	if got := len(decodePosts(t, out).Results); got != 45 {
		t.Fatalf("got %d rows, want all 45", got)
	}
}

func TestPostsByTimeOrdersChronologically(t *testing.T) {
	withTempHome(t)
	sources := xSources(3)
	sources[0].publishedAt, sources[1].publishedAt, sources[2].publishedAt = "2026-11-10T09:00:00.000Z", "2026-11-10T07:00:00.000Z", "2026-11-10T08:00:00.000Z"
	out, stderr, err := runDiggCmd(t, sportsStub(sources).serve(t), "posts", sportsStoryID, "--agent", "--by", "time")
	if err != nil {
		t.Fatalf("posts: %v\n%s", err, stderr)
	}
	var got []any
	for _, row := range decodePosts(t, out).Results {
		got = append(got, row["posted_at"])
	}
	want := []any{"2026-11-10T07:00:00.000Z", "2026-11-10T08:00:00.000Z", "2026-11-10T09:00:00.000Z"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}

func TestPostsDiggErrorIsNotAnEmptyResult(t *testing.T) {
	withTempHome(t)
	out, _, err := runDiggCmd(t, sportsStub(nil).serve(t), "posts", "99999999-2222-7333-8444-555555555555", "--agent")
	if err == nil {
		t.Fatalf("posts for a story Digg does not have succeeded:\n%s", out)
	}
	if !strings.Contains(err.Error(), "Post not found") {
		t.Errorf("error %q should carry Digg's message", err)
	}
}

func TestStoryReadsDetailAndSourcesFromDigg(t *testing.T) {
	withTempHome(t)
	recordSportsStory(t)
	out, stderr, err := runDiggCmd(t, sportsStub(xSources(4)).serve(t), "story", sportsStoryShort, "--json")
	if err != nil {
		t.Fatalf("story: %v\n%s", err, stderr)
	}
	var env struct {
		Story map[string]any   `json:"story"`
		Posts []map[string]any `json:"posts"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	want := map[string]any{
		"id": sportsStoryID, "clusterUrlId": sportsStoryShort, "topic": "sports",
		"url":   "https://digg.com/sports/" + sportsStoryShort,
		"title": "Game one goes to extra innings", "tldr": "The opener ran eleven innings.",
		"publishedAt": "2026-11-10T02:05:00.000Z",
	}
	for k, v := range want {
		if env.Story[k] != v {
			t.Errorf("story.%s = %v, want %v", k, env.Story[k], v)
		}
	}
	if len(env.Posts) != 4 {
		t.Fatalf("got %d posts, want 4", len(env.Posts))
	}
	if env.Posts[0]["body"] != "Take 0 on the game" {
		t.Errorf("first post body = %v", env.Posts[0]["body"])
	}
}

func TestOpenPrintsStoredTopicURL(t *testing.T) {
	withTempHome(t)
	recordSportsStory(t)
	// The endpoint refuses connections: a recorded short id needs no request.
	out, stderr, err := runDiggCmd(t, "http://127.0.0.1:1", "open", sportsStoryShort)
	if err != nil {
		t.Fatalf("open: %v\n%s", err, stderr)
	}
	if !strings.Contains(out, "https://digg.com/sports/"+sportsStoryShort) {
		t.Errorf("open printed %q, want the /sports/ URL", out)
	}
	if strings.Contains(out, "/ai/") {
		t.Errorf("open printed an /ai/ URL for a sports story: %q", out)
	}
}

func TestOpenByUUIDUsesDiggCanonicalURL(t *testing.T) {
	withTempHome(t)
	out, stderr, err := runDiggCmd(t, sportsStub(xSources(1)).serve(t), "open", sportsStoryID)
	if err != nil {
		t.Fatalf("open: %v\n%s", err, stderr)
	}
	if !strings.Contains(out, "https://digg.com/sports/"+sportsStoryShort) {
		t.Errorf("open printed %q, want the /sports/ URL", out)
	}
}

func TestOpenUnknownShortIDNamesSearch(t *testing.T) {
	withTempHome(t)
	out, _, err := runDiggCmd(t, "http://127.0.0.1:1", "open", "neverseen")
	if err == nil {
		t.Fatalf("open with an unrecorded short id succeeded:\n%s", out)
	}
	if !strings.Contains(err.Error(), "search") {
		t.Errorf("error %q should name the search command", err)
	}
}

func TestWatchPollsTheRequestedTopicPage(t *testing.T) {
	withTempHome(t)
	var mu sync.Mutex
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		_, _ = w.Write([]byte("<html></html>"))
	}))
	t.Cleanup(srv.Close)
	prev := diggSiteURL
	diggSiteURL = srv.URL
	t.Cleanup(func() { diggSiteURL = prev })

	if _, stderr, err := runDiggCmd(t, "http://127.0.0.1:1", "watch", "--topic", "sports", "--iterations", "1"); err != nil {
		t.Fatalf("watch: %v\n%s", err, stderr)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(paths) != 1 || paths[0] != "/sports" {
		t.Errorf("watch requested %v, want one request for /sports", paths)
	}
}

func TestPostsHelpDocumentsFlags(t *testing.T) {
	out, _, err := runDiggCmd(t, "http://127.0.0.1:1", "posts", "--help")
	if err != nil {
		t.Fatalf("posts --help: %v", err)
	}
	for _, want := range []string{"--by", "--type", "--limit"} {
		if !strings.Contains(out, want) {
			t.Errorf("posts --help missing %q; got:\n%s", want, out)
		}
	}
}
