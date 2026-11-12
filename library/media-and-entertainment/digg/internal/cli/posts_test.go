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
	"slices"
	"strconv"
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
	// limitReached is what the stub reports as sourceLimitReached.
	limitReached bool
	mu           sync.Mutex
	calls        []map[string]any
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
		"sources": sources, "nextSourceOffset": next, "sourceLimitReached": s.limitReached,
	}
}

func (s *storyStub) requests() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]map[string]any(nil), s.calls...)
}

// serveStructured is a stand-in for Digg's MCP server that answers every
// tool call with the given structuredContent JSON.
func serveStructured(t *testing.T, structuredContent string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"structuredContent\":%s}}\n\n", structuredContent)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
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

// Digg lists a newer source ahead of older ones, and the oldest sit on a
// later page. --by time --limit must return the oldest of all of them.
func TestPostsByTimeLimitReturnsTheOldestSources(t *testing.T) {
	withTempHome(t)
	sources := xSources(45)
	for i := range sources {
		sources[i].publishedAt = fmt.Sprintf("2026-11-10T12:%02d:00.000Z", i)
	}
	sources[30].publishedAt = "2026-11-09T01:00:00.000Z"
	sources[44].publishedAt = "2026-11-09T02:00:00.000Z"
	out, stderr, err := runDiggCmd(t, sportsStub(sources).serve(t), "posts", sportsStoryID, "--agent", "--by", "time", "--limit", "3")
	if err != nil {
		t.Fatalf("posts: %v\n%s", err, stderr)
	}
	env := decodePosts(t, out)
	var got []any
	for _, row := range env.Results {
		got = append(got, row["posted_at"])
	}
	want := []any{"2026-11-09T01:00:00.000Z", "2026-11-09T02:00:00.000Z", "2026-11-10T12:00:00.000Z"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("posted_at = %v, want the three oldest %v", got, want)
	}
	if env.Meta["count"] != float64(3) {
		t.Errorf("meta.count = %v, want 3", env.Meta["count"])
	}
}

// Without --by time a limit still stops the fetch at that many sources.
func TestPostsLimitInDiggOrderFetchesOnlyThatMany(t *testing.T) {
	withTempHome(t)
	stub := sportsStub(xSources(45))
	out, stderr, err := runDiggCmd(t, stub.serve(t), "posts", sportsStoryID, "--agent", "--limit", "3")
	if err != nil {
		t.Fatalf("posts: %v\n%s", err, stderr)
	}
	rows := decodePosts(t, out).Results
	if len(rows) != 3 || rows[0]["body"] != "Take 0 on the game" || rows[2]["body"] != "Take 2 on the game" {
		t.Errorf("rows = %v, want Digg's first three", rows)
	}
	if calls := stub.requests(); len(calls) != 1 {
		t.Errorf("made %d requests for 3 sources, want 1", len(calls))
	}
}

func TestPostsReportsWhenDiggCutsTheSourceListShort(t *testing.T) {
	withTempHome(t)
	stub := sportsStub(xSources(5))
	stub.limitReached = true
	url := stub.serve(t)
	out, stderr, err := runDiggCmd(t, url, "posts", sportsStoryID, "--agent")
	if err != nil {
		t.Fatalf("posts: %v\n%s", err, stderr)
	}
	if got := decodePosts(t, out).Meta["sourcesTruncated"]; got != true {
		t.Errorf("meta.sourcesTruncated = %v, want true", got)
	}

	out, stderr, err = runDiggCmd(t, url, "posts", sportsStoryID)
	if err != nil {
		t.Fatalf("posts: %v\n%s", err, stderr)
	}
	if !strings.Contains(stderr, "warning") || !strings.Contains(stderr, "incomplete") {
		t.Errorf("stderr = %q, want a warning that the list is incomplete", stderr)
	}
	if strings.Contains(out, "incomplete") {
		t.Errorf("the warning belongs on stderr, got it on stdout:\n%s", out)
	}
}

func TestPostsCompleteSourceListIsNotMarkedTruncated(t *testing.T) {
	withTempHome(t)
	url := sportsStub(xSources(45)).serve(t)
	out, stderr, err := runDiggCmd(t, url, "posts", sportsStoryID, "--agent")
	if err != nil {
		t.Fatalf("posts: %v\n%s", err, stderr)
	}
	if got := decodePosts(t, out).Meta["sourcesTruncated"]; got != false {
		t.Errorf("meta.sourcesTruncated = %v, want false", got)
	}
	_, stderr, err = runDiggCmd(t, url, "posts", sportsStoryID)
	if err != nil {
		t.Fatalf("posts: %v\n%s", err, stderr)
	}
	if strings.Contains(stderr, "incomplete") {
		t.Errorf("stderr = %q, want no warning for a complete list", stderr)
	}
}

// Digg accepts source offsets up to 499. A story with more sources than
// that reaches cannot be read to the end, and posts must say so.
func TestPostsReportsWhenSourcesRunPastTheOffsetMaximum(t *testing.T) {
	withTempHome(t)
	stub := sportsStub(xSources(520))
	out, stderr, err := runDiggCmd(t, stub.serve(t), "posts", sportsStoryID, "--agent")
	if err != nil {
		t.Fatalf("posts: %v\n%s", err, stderr)
	}
	env := decodePosts(t, out)
	if len(env.Results) != 500 {
		t.Errorf("got %d rows, want the 500 Digg's offsets reach", len(env.Results))
	}
	if got := env.Meta["sourcesTruncated"]; got != true {
		t.Errorf("meta.sourcesTruncated = %v, want true", got)
	}
	for _, call := range stub.requests() {
		if o, _ := call["sourceOffset"].(float64); o > 499 {
			t.Errorf("asked Digg for source offset %v; its maximum is 499", o)
		}
	}
}

// A digg_get_post result without its story or sources is a changed
// response shape, not a story with no sources.
func TestPostsAndStoryRejectAResponseWithoutStoryOrSources(t *testing.T) {
	withTempHome(t)
	story := `{"id":"` + sportsStoryID + `","url":"https://digg.com/sports/` + sportsStoryShort + `","topic":"sports","title":"Game one"}`
	for name, content := range map[string]string{
		"empty object":    `{}`,
		"sources missing": `{"story":` + story + `}`,
		"sources null":    `{"story":` + story + `,"sources":null}`,
		"story missing":   `{"sources":[]}`,
		"story null":      `{"story":null,"sources":[]}`,
	} {
		for _, command := range []string{"posts", "story"} {
			out, _, err := runDiggCmd(t, serveStructured(t, content), command, sportsStoryID, "--agent")
			if err == nil || ExitCode(err) == 0 {
				t.Errorf("%s, %s: err = %v, want a non-zero exit\n%s", command, name, err, out)
				continue
			}
			if !strings.Contains(err.Error(), "expected shape") {
				t.Errorf("%s, %s: error %q should say the response did not have the expected shape", command, name, err)
			}
		}
	}
}

func TestPostsStoryWithNoSourcesIsAValidEmptyResult(t *testing.T) {
	withTempHome(t)
	out, stderr, err := runDiggCmd(t, sportsStub(nil).serve(t), "posts", sportsStoryID, "--agent")
	if err != nil {
		t.Fatalf("posts: %v\n%s", err, stderr)
	}
	if env := decodePosts(t, out); len(env.Results) != 0 || env.Meta["count"] != float64(0) {
		t.Errorf("got %d rows and count %v, want an empty result", len(env.Results), env.Meta["count"])
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

// watchSectionStub serves topic pages with a ranked list per topic, the
// way sectionStub does, and calls beforePage (when set) ahead of each
// response without holding its lock, so a test can run another watcher
// between two polls.
type watchSectionStub struct {
	mu         sync.Mutex
	ranks      map[string][]string
	hits       map[string]int
	beforePage func(topic string, hit int)
}

func (s *watchSectionStub) setRanks(topic string, ids ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ranks[topic] = ids
}

func (s *watchSectionStub) serve(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		topic := strings.TrimPrefix(r.URL.Path, "/")
		s.mu.Lock()
		if s.hits == nil {
			s.hits = map[string]int{}
		}
		s.hits[topic]++
		hit, hook := s.hits[topic], s.beforePage
		s.mu.Unlock()
		if hook != nil {
			hook(topic, hit)
		}
		s.mu.Lock()
		ids := s.ranks[topic]
		s.mu.Unlock()
		var objs []string
		for i, id := range ids {
			objs = append(objs, fmt.Sprintf(`{"clusterId":"uuid-%s","clusterUrlId":"%s","rank":%d}`, id, id, i+1))
		}
		payload := `{"posts":[` + strings.Join(objs, ",") + `]}`
		fmt.Fprintf(w, `<html><script>self.__next_f.push([1,%s])</script></html>`, strconv.Quote(payload))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// Two watchers on different topics share one store, each run as its own
// process would be. Each one's next poll must still see its own movers.
func TestWatchTopicsSharingAStoreKeepTheirOwnBaseline(t *testing.T) {
	withTempHome(t)
	stub := &watchSectionStub{ranks: map[string][]string{
		"ai":     {"aaa11111", "aaa22222", "aaa33333"},
		"sports": {"sss11111", "sss22222", "sss33333"},
	}}
	url := stub.serve(t)
	poll := func(topic string) string {
		t.Helper()
		out, stderr, err := runCLIAgainst(t, url, "watch", "--topic", topic, "--iterations", "1", "--min-delta", "2")
		if err != nil {
			t.Fatalf("watch %s: %v\n%s", topic, err, stderr)
		}
		return out
	}
	poll("ai")
	poll("sports")

	stub.setRanks("ai", "aaa33333", "aaa11111", "aaa22222")
	if out := poll("ai"); !strings.Contains(out, "+2") || !strings.Contains(out, "[aaa33333]") {
		t.Errorf("second ai poll did not alert on the story that climbed two ranks:\n%s", out)
	}
	stub.setRanks("sports", "sss22222", "sss33333", "sss11111")
	if out := poll("sports"); !strings.Contains(out, "-2") || !strings.Contains(out, "[sss11111]") {
		t.Errorf("second sports poll did not alert on the story that fell two ranks:\n%s", out)
	}
}

// One long-running watcher polls ai twice. Between its two polls another
// watcher polls sports into the same store.
func TestWatchKeepsItsBaselineWhileAnotherTopicIsPolled(t *testing.T) {
	withTempHome(t)
	stub := &watchSectionStub{ranks: map[string][]string{
		"ai":     {"aaa11111", "aaa22222", "aaa33333"},
		"sports": {"sss11111", "sss22222", "sss33333"},
	}}
	url := stub.serve(t)
	stub.beforePage = func(topic string, hit int) {
		if topic != "ai" || hit != 2 {
			return
		}
		if _, stderr, err := runCLIAgainst(t, url, "watch", "--topic", "sports", "--iterations", "1"); err != nil {
			t.Errorf("watch sports: %v\n%s", err, stderr)
		}
		stub.setRanks("ai", "aaa33333", "aaa11111", "aaa22222")
	}
	out, stderr, err := runCLIAgainst(t, url, "watch", "--topic", "ai", "--iterations", "2", "--interval", "1ms", "--min-delta", "2")
	if err != nil {
		t.Fatalf("watch ai: %v\n%s", err, stderr)
	}
	if !strings.Contains(out, "+2") || !strings.Contains(out, "[aaa33333]") {
		t.Errorf("second ai poll did not alert on the story that climbed two ranks:\n%s", out)
	}
}

// watchOnce runs one `watch` poll of topic as its own command invocation,
// the way a restarted watcher process would, and returns its output.
func watchOnce(t *testing.T, url, topic string) string {
	t.Helper()
	out, stderr, err := runCLIAgainst(t, url, "watch", "--topic", topic, "--iterations", "1", "--min-delta", "2")
	if err != nil {
		t.Fatalf("watch %s: %v\n%s", topic, err, stderr)
	}
	return out
}

// watchAlertLines returns each alert in out as "<delta> [<short id>]".
func watchAlertLines(out string) []string {
	var alerts []string
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if !strings.HasPrefix(line, "  ") || len(f) < 2 {
			continue
		}
		alerts = append(alerts, f[0]+" "+f[len(f)-1])
	}
	return alerts
}

func wantWatchAlerts(t *testing.T, what, out string, want ...string) {
	t.Helper()
	if got := watchAlertLines(out); !slices.Equal(got, want) {
		t.Errorf("%s: alerts %v, want %v; output:\n%s", what, got, want, out)
	}
}

// Two topics are polled alternately against one store, every poll its own
// invocation. Each topic's later polls alert on exactly its own movers.
func TestWatchRestartedWatchersAlertOnExactlyTheirOwnMovers(t *testing.T) {
	withTempHome(t)
	stub := &watchSectionStub{ranks: map[string][]string{
		"ai":     {"aaa11111", "aaa22222", "aaa33333"},
		"sports": {"sss11111", "sss22222", "sss33333"},
	}}
	url := stub.serve(t)
	wantWatchAlerts(t, "first ai poll", watchOnce(t, url, "ai"))
	wantWatchAlerts(t, "first sports poll", watchOnce(t, url, "sports"))

	stub.setRanks("ai", "aaa33333", "aaa11111", "aaa22222")
	wantWatchAlerts(t, "second ai poll", watchOnce(t, url, "ai"), "+2 [aaa33333]")
	stub.setRanks("sports", "sss22222", "sss33333", "sss11111")
	wantWatchAlerts(t, "second sports poll", watchOnce(t, url, "sports"), "-2 [sss11111]")

	// Nothing moved since each topic's own last poll.
	wantWatchAlerts(t, "third ai poll", watchOnce(t, url, "ai"))
	wantWatchAlerts(t, "third sports poll", watchOnce(t, url, "sports"))
}

// A sync of every topic lands between a topic's two polls, after the
// topic's ranks changed. The second poll still compares against the first
// poll, not against what the sync saved.
func TestWatchBaselineSurvivesASyncOfAllTopics(t *testing.T) {
	withTempHome(t)
	stub := &watchSectionStub{ranks: map[string][]string{
		"ai": {"aaa11111", "aaa22222", "aaa33333"},
		// The cross-topic mix lists ai's stories at other ranks.
		"": {"sss11111", "ttt11111", "aaa33333", "aaa22222", "aaa11111"},
	}}
	for _, topic := range []string{"tech", "world-business", "sports", "science", "gaming", "entertainment"} {
		stub.setRanks(topic, topic[:3]+"11111", topic[:3]+"22222")
	}
	url := stub.serve(t)
	wantWatchAlerts(t, "first ai poll", watchOnce(t, url, "ai"))

	stub.setRanks("ai", "aaa33333", "aaa11111", "aaa22222")
	if out, stderr, err := runCLIAgainst(t, url, "sync", "--topics", strings.Join(validTopicSlugs(), ",")); err != nil || stderr != "" {
		t.Fatalf("sync: %v\n%s\n%s", err, out, stderr)
	}
	wantWatchAlerts(t, "ai poll after the sync", watchOnce(t, url, "ai"), "+2 [aaa33333]")
}

// One story is listed in two topics at different ranks. Each topic's
// baseline holds that story's rank in that topic only.
func TestWatchStoryInTwoTopicsKeepsARankPerTopic(t *testing.T) {
	withTempHome(t)
	stub := &watchSectionStub{ranks: map[string][]string{
		"ai":     {"xxx11111", "aaa22222", "aaa33333"},
		"sports": {"sss11111", "sss22222", "xxx11111"},
	}}
	url := stub.serve(t)
	watchOnce(t, url, "ai")
	watchOnce(t, url, "sports")

	// The shared story is third in sports and still first in ai.
	wantWatchAlerts(t, "second ai poll, nothing moved", watchOnce(t, url, "ai"))
	wantWatchAlerts(t, "second sports poll, nothing moved", watchOnce(t, url, "sports"))

	stub.setRanks("ai", "aaa22222", "aaa33333", "xxx11111")
	wantWatchAlerts(t, "third ai poll", watchOnce(t, url, "ai"), "-2 [xxx11111]")
	stub.setRanks("sports", "xxx11111", "sss11111", "sss22222")
	wantWatchAlerts(t, "third sports poll", watchOnce(t, url, "sports"), "+2 [xxx11111]")
}

// With nothing stored for the topic, the first poll reports no movers,
// says it is the baseline, and stores the ranks the next poll compares to.
func TestWatchFirstPollOfATopicIsTheBaseline(t *testing.T) {
	withTempHome(t)
	stub := &watchSectionStub{ranks: map[string][]string{
		"ai":     {"aaa11111", "aaa22222", "aaa33333"},
		"sports": {"aaa33333", "aaa22222", "aaa11111"},
	}}
	url := stub.serve(t)
	// Another topic's poll of the same stories is not ai's baseline.
	watchOnce(t, url, "sports")

	out := watchOnce(t, url, "ai")
	wantWatchAlerts(t, "first ai poll", out)
	if !strings.Contains(out, "baseline") || strings.Contains(out, "alerts") {
		t.Errorf("first ai poll should report a baseline poll and no alerts, got:\n%s", out)
	}

	stub.setRanks("ai", "aaa33333", "aaa11111", "aaa22222")
	out = watchOnce(t, url, "ai")
	wantWatchAlerts(t, "second ai poll", out, "+2 [aaa33333]")
	if strings.Contains(out, "baseline") {
		t.Errorf("second ai poll should not be a baseline poll, got:\n%s", out)
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
