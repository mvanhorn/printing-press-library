// Tests for the `search` command. Search calls Digg's digg_search_posts
// tool and reads no local copy. Each test points the command at an
// httptest stub of Digg's MCP server via diggMCPURLOverride.

package cli

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/diggstore"
)

// stubPost is one story the stub server knows. score < 0 and firstSource
// "" stand for the nulls Digg sends when it has no score or no source date.
type stubPost struct {
	id, shortID, topic, title string
	score                     int
	age                       time.Duration
	firstSource               string
}

func (p stubPost) json() map[string]any {
	published := time.Now().Add(-p.age).UTC().Format(time.RFC3339)
	out := map[string]any{
		"id": p.id, "url": "https://digg.com/" + p.topic + "/" + p.shortID, "topic": p.topic,
		"title": p.title, "description": "About " + p.title, "tldr": "Short: " + p.title,
		"feedEntryAt": published, "publishedAt": published, "updatedAt": published,
		"diggScore": nil, "sourceCount": 14, "uniqueAuthors": 9, "firstSourceAt": nil,
	}
	if p.score >= 0 {
		out["diggScore"] = p.score
	}
	if p.firstSource != "" {
		out["firstSourceAt"] = p.firstSource
	}
	return out
}

// manyPosts returns n stories in topic with distinct ids and scores.
func manyPosts(n int, topic string) []stubPost {
	posts := make([]stubPost, n)
	for i := range posts {
		posts[i] = stubPost{
			id: fmt.Sprintf("00000000-0000-7000-8000-%012d", i), shortID: fmt.Sprintf("%s%04d", topic, i),
			topic: topic, title: fmt.Sprintf("Story %d", i), score: (i * 37) % 101, age: 2 * time.Hour,
		}
	}
	return posts
}

// searchStub is a stand-in for Digg's MCP server. It filters by topic,
// orders by score when asked, and pages by the requested limit using the
// offset as the cursor. fail makes every call a tool error.
type searchStub struct {
	posts []stubPost
	fail  bool
	mu    sync.Mutex
	calls []map[string]any
}

func (s *searchStub) serve(t *testing.T) string {
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
		if s.fail || req.Params.Name != "digg_search_posts" {
			result = map[string]any{"isError": true, "content": []map[string]any{{"type": "text", "text": "search is unavailable"}}}
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

func (s *searchStub) page(args map[string]any) map[string]any {
	var matched []stubPost
	topics, filtered := args["topic"].([]any)
	for _, p := range s.posts {
		keep := !filtered
		for _, tp := range topics {
			if tp == p.topic {
				keep = true
			}
		}
		if keep {
			matched = append(matched, p)
		}
	}
	if args["sort"] == "score" {
		sort.SliceStable(matched, func(i, j int) bool { return matched[i].score > matched[j].score })
	}
	offset := 0
	if c, ok := args["cursor"].(string); ok {
		offset, _ = strconv.Atoi(c)
	}
	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}
	end := min(offset+limit, len(matched))
	posts := []map[string]any{}
	for _, p := range matched[min(offset, end):end] {
		posts = append(posts, p.json())
	}
	var next any
	if end < len(matched) {
		next = strconv.Itoa(end)
	}
	return map[string]any{
		"posts": posts, "nextCursor": next, "totalMatches": len(matched),
		"window": map[string]string{"since": "2026-10-12T00:00:00.000Z", "until": "2026-11-11T00:00:00.000Z"},
		"sort":   "relevance",
	}
}

func (s *searchStub) requests() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]map[string]any(nil), s.calls...)
}

func runSearchCmd(t *testing.T, mcpURL string, args ...string) (string, string, error) {
	t.Helper()
	prev := diggMCPURLOverride
	diggMCPURLOverride = mcpURL
	t.Cleanup(func() { diggMCPURLOverride = prev })

	var flags rootFlags
	root := newRootCmd(&flags)
	root.SetArgs(append([]string{"search"}, args...))
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

// searchOutput is the printed envelope, decoded loosely so tests can
// assert which keys are present.
type searchOutput struct {
	Meta    map[string]any   `json:"meta"`
	Results []map[string]any `json:"results"`
}

func decodeSearch(t *testing.T, stdout string) searchOutput {
	t.Helper()
	var env searchOutput
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	return env
}

// The exact call last30days 3.36.1 makes. This pins the envelope older
// skill versions read plus the fields added for Digg's engagement numbers.
func TestSearchLast30daysCallShape(t *testing.T) {
	withTempHome(t)
	stub := &searchStub{posts: []stubPost{{
		id: "0190a1b2-c3d4-7e5f-8a6b-000000000001", shortID: "ab12cd34", topic: "ai",
		title: "Gemini 3 ships", score: 87, age: 3 * 24 * time.Hour, firstSource: "2026-11-08T09:30:00.000Z",
	}}}
	stdout, _, err := runSearchCmd(t, stub.serve(t), "gemini", "--topic", "ai,tech", "--since", "30d", "--agent", "--limit", "20")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	env := decodeSearch(t, stdout)
	if len(env.Results) != 1 {
		t.Fatalf("got %d results, want 1: %s", len(env.Results), stdout)
	}
	r := env.Results[0]
	want := map[string]any{
		// Keys older skill versions read.
		"clusterId":     "0190a1b2-c3d4-7e5f-8a6b-000000000001",
		"clusterUrlId":  "ab12cd34",
		"title":         "Gemini 3 ships",
		"tldr":          "Short: Gemini 3 ships",
		"postCount":     float64(14),
		"uniqueAuthors": float64(9),
		"firstPostAge":  "3d",
		// Keys added with live search.
		"url":           "https://digg.com/ai/ab12cd34",
		"topic":         "ai",
		"diggScore":     float64(87),
		"sourceCount":   float64(14),
		"firstSourceAt": "2026-11-08T09:30:00.000Z",
		"description":   "About Gemini 3 ships",
	}
	for k, v := range want {
		if r[k] != v {
			t.Errorf("results[0].%s = %v, want %v", k, r[k], v)
		}
	}
	if at, err := time.Parse(time.RFC3339, fmt.Sprint(r["publishedAt"])); err != nil || time.Since(at) < 71*time.Hour || time.Since(at) > 73*time.Hour {
		t.Errorf("results[0].publishedAt = %v, want the story's publish time 3 days ago", r["publishedAt"])
	}
	if _, has := r["rank"]; has {
		t.Errorf("results carry no rank, got %v", r["rank"])
	}
	if env.Meta["source"] != "live" || env.Meta["stale"] != false {
		t.Errorf("meta = %v, want source=live stale=false", env.Meta)
	}
	if env.Meta["totalMatches"] != float64(1) || env.Meta["sort"] != "relevance" {
		t.Errorf("meta = %v, want totalMatches=1 sort=relevance", env.Meta)
	}
	window, _ := env.Meta["window"].(map[string]any)
	if window["since"] != "2026-10-12T00:00:00.000Z" || window["until"] != "2026-11-11T00:00:00.000Z" {
		t.Errorf("meta.window = %v, want the window Digg resolved", env.Meta["window"])
	}

	reqs := stub.requests()
	if len(reqs) != 1 {
		t.Fatalf("made %d requests, want 1", len(reqs))
	}
	a := reqs[0]
	if a["query"] != "gemini" || a["range"] != "30d" || a["limit"] != float64(20) || fmt.Sprint(a["topic"]) != "[ai tech]" {
		t.Errorf("request arguments = %v", a)
	}
	if _, has := a["since"]; has {
		t.Errorf("--since 30d should travel as range, got since=%v", a["since"])
	}
}

func TestSearchFollowsCursorPastOnePage(t *testing.T) {
	withTempHome(t)
	stub := &searchStub{posts: manyPosts(120, "ai")}
	stdout, _, err := runSearchCmd(t, stub.serve(t), "story", "--limit", "120", "--json")
	if err != nil {
		t.Fatal(err)
	}
	env := decodeSearch(t, stdout)
	seen := map[any]bool{}
	for _, r := range env.Results {
		seen[r["clusterId"]] = true
	}
	if len(env.Results) != 120 || len(seen) != 120 {
		t.Fatalf("got %d results with %d distinct clusterIds, want 120 and 120", len(env.Results), len(seen))
	}
	var limits []any
	for _, a := range stub.requests() {
		limits = append(limits, a["limit"])
	}
	if fmt.Sprint(limits) != "[50 50 20]" {
		t.Fatalf("page limits = %v, want [50 50 20]", limits)
	}
}

func TestSearchStopsWhenMatchesRunOut(t *testing.T) {
	withTempHome(t)
	stub := &searchStub{posts: manyPosts(70, "ai")}
	stdout, _, err := runSearchCmd(t, stub.serve(t), "story", "--limit", "120", "--json")
	if err != nil {
		t.Fatalf("fewer matches than --limit is not an error: %v", err)
	}
	env := decodeSearch(t, stdout)
	if len(env.Results) != 70 || env.Meta["count"] != float64(70) {
		t.Fatalf("got %d results (meta.count %v), want 70", len(env.Results), env.Meta["count"])
	}
}

func TestSearchTopicFilter(t *testing.T) {
	withTempHome(t)
	stub := &searchStub{posts: append(manyPosts(2, "ai"), manyPosts(2, "sports")...)}
	url := stub.serve(t)

	stdout, _, err := runSearchCmd(t, url, "story", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if _, has := stub.requests()[0]["topic"]; has {
		t.Fatalf("no --topic should send no topic filter, sent %v", stub.requests()[0]["topic"])
	}
	if n := len(decodeSearch(t, stdout).Results); n != 4 {
		t.Fatalf("got %d results across every topic, want 4", n)
	}

	stdout, _, err = runSearchCmd(t, url, "story", "--topic", "sports", "--json")
	if err != nil {
		t.Fatal(err)
	}
	env := decodeSearch(t, stdout)
	if len(env.Results) != 2 {
		t.Fatalf("got %d results, want the 2 sports stories", len(env.Results))
	}
	for _, r := range env.Results {
		if r["topic"] != "sports" {
			t.Fatalf("--topic sports returned a %v story", r["topic"])
		}
	}
}

func TestSearchSinceWindowArguments(t *testing.T) {
	withTempHome(t)
	stub := &searchStub{posts: manyPosts(1, "ai")}
	url := stub.serve(t)

	if _, _, err := runSearchCmd(t, url, "story", "--since", "45d", "--json"); err != nil {
		t.Fatal(err)
	}
	a := stub.requests()[0]
	since, err := time.Parse(time.RFC3339, fmt.Sprint(a["since"]))
	if err != nil {
		t.Fatalf("--since 45d sent since=%v, want an ISO 8601 datetime", a["since"])
	}
	if off := time.Since(since) - 45*24*time.Hour; off < 0 || off > time.Minute {
		t.Fatalf("since = %s, want 45 days ago", since)
	}
	if _, has := a["range"]; has {
		t.Fatalf("--since 45d must not send range, sent %v", a["range"])
	}

	if _, _, err := runSearchCmd(t, url, "story", "--since", "30d", "--json"); err != nil {
		t.Fatal(err)
	}
	a = stub.requests()[1]
	if _, has := a["since"]; a["range"] != "30d" || has {
		t.Fatalf("--since 30d sent %v, want range=30d and no since", a)
	}
}

func TestSearchUntilSendsAbsoluteWindow(t *testing.T) {
	withTempHome(t)
	stub := &searchStub{posts: manyPosts(1, "ai")}
	until := time.Now().Add(-10 * 24 * time.Hour).UTC().Format("2006-01-02")
	if _, _, err := runSearchCmd(t, stub.serve(t), "story", "--since", "30d", "--until", until, "--json"); err != nil {
		t.Fatal(err)
	}
	a := stub.requests()[0]
	if _, has := a["range"]; has {
		t.Fatalf("--until must never travel with range, sent %v", a)
	}
	since, err := time.Parse(time.RFC3339, fmt.Sprint(a["since"]))
	if err != nil {
		t.Fatalf("since = %v, want an ISO 8601 datetime", a["since"])
	}
	if off := time.Since(since) - 30*24*time.Hour; off < 0 || off > time.Minute {
		t.Fatalf("since = %s, want 30 days ago", since)
	}
	if a["until"] != until+"T00:00:00Z" {
		t.Fatalf("until = %v, want %sT00:00:00Z", a["until"], until)
	}
}

func TestSearchSortScore(t *testing.T) {
	withTempHome(t)
	stub := &searchStub{posts: manyPosts(30, "ai")}
	stdout, _, err := runSearchCmd(t, stub.serve(t), "story", "--sort", "score", "--limit", "30", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if got := stub.requests()[0]["sort"]; got != "score" {
		t.Fatalf("request sort = %v, want score", got)
	}
	env := decodeSearch(t, stdout)
	if len(env.Results) != 30 {
		t.Fatalf("got %d results, want 30", len(env.Results))
	}
	for i := 1; i < len(env.Results); i++ {
		prev, cur := env.Results[i-1]["diggScore"].(float64), env.Results[i]["diggScore"].(float64)
		if cur > prev {
			t.Fatalf("results[%d].diggScore %v follows %v; want descending order", i, cur, prev)
		}
	}
}

func TestSearchOmitsNullScoreAndFirstSource(t *testing.T) {
	withTempHome(t)
	stub := &searchStub{posts: []stubPost{{id: "u-1", shortID: "nulls001", topic: "tech", title: "Unscored", score: -1, age: time.Hour}}}
	stdout, _, err := runSearchCmd(t, stub.serve(t), "unscored", "--json")
	if err != nil {
		t.Fatal(err)
	}
	r := decodeSearch(t, stdout).Results[0]
	for _, k := range []string{"diggScore", "firstSourceAt"} {
		if v, has := r[k]; has {
			t.Errorf("results[0].%s = %v, want the key left out when Digg sends null", k, v)
		}
	}
	if r["clusterUrlId"] != "nulls001" {
		t.Errorf("clusterUrlId = %v", r["clusterUrlId"])
	}
}

func TestSearchFirstPostAgeTokens(t *testing.T) {
	withTempHome(t)
	stub := &searchStub{posts: []stubPost{
		{id: "u-1", shortID: "age00001", topic: "ai", title: "Minutes old", age: 45 * time.Minute},
		{id: "u-2", shortID: "age00002", topic: "ai", title: "Days old", age: 3 * 24 * time.Hour},
		{id: "u-3", shortID: "age00003", topic: "ai", title: "Weeks old", age: 40 * 24 * time.Hour},
	}}
	stdout, _, err := runSearchCmd(t, stub.serve(t), "old", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var ages []string
	for _, r := range decodeSearch(t, stdout).Results {
		ages = append(ages, fmt.Sprint(r["firstPostAge"]))
	}
	if strings.Join(ages, ",") != "1h,3d,40d" {
		t.Fatalf("firstPostAge = %v, want [1h 3d 40d]", ages)
	}
}

func TestSearchRejectsBadQueryBeforeAnyRequest(t *testing.T) {
	withTempHome(t)
	stub := &searchStub{posts: manyPosts(1, "ai")}
	url := stub.serve(t)
	for _, q := range []string{"ai", strings.Repeat("x", 121)} {
		_, _, err := runSearchCmd(t, url, q, "--json")
		if err == nil || ExitCode(err) != 2 {
			t.Fatalf("query of %d chars: err = %v (exit %d), want a usage error", len(q), err, ExitCode(err))
		}
	}
	if n := len(stub.requests()); n != 0 {
		t.Fatalf("made %d requests for rejected queries, want 0", n)
	}
}

func TestSearchRejectsWindowOlderThan90Days(t *testing.T) {
	withTempHome(t)
	stub := &searchStub{posts: manyPosts(1, "ai")}
	url := stub.serve(t)
	old := time.Now().Add(-100 * 24 * time.Hour).UTC().Format("2006-01-02")
	for _, since := range []string{"91d", "4m", old} {
		_, _, err := runSearchCmd(t, url, "story", "--since", since, "--json")
		if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "90 days") {
			t.Fatalf("--since %s: err = %v, want a usage error naming the 90 day limit", since, err)
		}
	}
	if n := len(stub.requests()); n != 0 {
		t.Fatalf("made %d requests for rejected windows, want 0", n)
	}
}

func TestSearchServerErrorIsNotAnEmptyResult(t *testing.T) {
	withTempHome(t)
	stub := &searchStub{fail: true}
	stdout, _, err := runSearchCmd(t, stub.serve(t), "story", "--json")
	if err == nil || ExitCode(err) == 0 {
		t.Fatalf("err = %v, want a non-zero exit", err)
	}
	env := decodeSearch(t, stdout)
	if msg, _ := env.Meta["error"].(string); !strings.Contains(msg, "search is unavailable") {
		t.Fatalf("meta.error = %v, want the server's error", env.Meta["error"])
	}
}

func TestSearchSelectFiltersResultsEnvelope(t *testing.T) {
	withTempHome(t)
	stub := &searchStub{posts: manyPosts(3, "ai")}
	stdout, _, err := runSearchCmd(t, stub.serve(t), "story", "--json", "--select", "title,diggScore")
	if err != nil {
		t.Fatalf("search --select: %v", err)
	}
	env := decodeSearch(t, stdout)
	if len(env.Results) != 3 {
		t.Fatalf("got %d results, want 3: %s", len(env.Results), stdout)
	}
	for _, r := range env.Results {
		_, hasTitle := r["title"]
		_, hasScore := r["diggScore"]
		if !hasTitle || !hasScore || len(r) != 2 {
			t.Fatalf("selected result = %v, want only title and diggScore", r)
		}
	}
}

func TestSearchRecordsStoryIDs(t *testing.T) {
	withTempHome(t)
	posts := append(manyPosts(3, "ai"), manyPosts(2, "gaming")...)
	if _, _, err := runSearchCmd(t, (&searchStub{posts: posts}).serve(t), "story", "--json"); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", defaultDBPath("digg-pp-cli"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, p := range posts {
		ref, ok, err := diggstore.LookupPostRef(db, p.shortID)
		if err != nil || !ok {
			t.Fatalf("short id %s: ok=%v err=%v, want a record", p.shortID, ok, err)
		}
		if ref.ID != p.id || ref.Topic != p.topic {
			t.Fatalf("short id %s recorded as %+v, want id %s topic %s", p.shortID, ref, p.id, p.topic)
		}
	}
}

func TestSearchUnwritableStoreStillReturnsResults(t *testing.T) {
	// A file where the store's directory tree should start makes the
	// store impossible to create, whoever runs the test.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := os.WriteFile(filepath.Join(home, ".local"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	stub := &searchStub{posts: manyPosts(3, "ai")}
	stdout, stderr, err := runSearchCmd(t, stub.serve(t), "story", "--json")
	if err != nil {
		t.Fatalf("a store that cannot be written must not fail search: %v", err)
	}
	if n := len(decodeSearch(t, stdout).Results); n != 3 {
		t.Fatalf("got %d results, want 3", n)
	}
	if !strings.Contains(stderr, "warn") || !strings.Contains(stderr, "story ids") {
		t.Fatalf("stderr %q should warn that story ids were not recorded", stderr)
	}
}

// TestSearchHelperProcess is not a test: it is the body of the child
// processes TestSearchConcurrentProcessesShareOneStore starts.
func TestSearchHelperProcess(t *testing.T) {
	if os.Getenv("DIGG_SEARCH_HELPER") != "1" {
		return
	}
	diggMCPURLOverride = os.Getenv("DIGG_SEARCH_HELPER_URL")
	var flags rootFlags
	root := newRootCmd(&flags)
	root.SetArgs([]string{"search", "story", "--limit", "50", "--json"})
	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(ExitCode(err))
	}
	os.Exit(0)
}

func TestSearchConcurrentProcessesShareOneStore(t *testing.T) {
	home := t.TempDir()
	posts := manyPosts(50, "ai")
	url := (&searchStub{posts: posts}).serve(t)

	type outcome struct {
		stdout, stderr string
		err            error
	}
	outcomes := make([]outcome, 4)
	var wg sync.WaitGroup
	for i := range outcomes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command(os.Args[0], "-test.run=^TestSearchHelperProcess$")
			cmd.Env = append(os.Environ(), "DIGG_SEARCH_HELPER=1", "DIGG_SEARCH_HELPER_URL="+url, "HOME="+home, "USERPROFILE="+home)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			outcomes[i] = outcome{stdout.String(), stderr.String(), err}
		}()
	}
	wg.Wait()
	for i, o := range outcomes {
		if o.err != nil {
			t.Fatalf("process %d: %v\nstderr: %s", i, o.err, o.stderr)
		}
		if n := len(decodeSearch(t, o.stdout).Results); n != 50 {
			t.Fatalf("process %d returned %d results, want 50\nstderr: %s", i, n, o.stderr)
		}
		if strings.Contains(o.stderr, "warn") {
			t.Fatalf("process %d could not record story ids: %s", i, o.stderr)
		}
	}
	db, err := sql.Open("sqlite", filepath.Join(home, ".local", "share", "digg-pp-cli", "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, ok, err := diggstore.LookupPostRef(db, posts[49].shortID); err != nil || !ok {
		t.Fatalf("shared store is missing a recorded story: ok=%v err=%v", ok, err)
	}
}

func TestSearchRejectsUnknownTopic(t *testing.T) {
	withTempHome(t)
	_, _, err := runSearchCmd(t, "http://127.0.0.1:1", "claude", "--topic", "news")
	if err == nil || !strings.Contains(err.Error(), "valid topics") {
		t.Fatalf("err = %v, want a usage error listing valid topics", err)
	}
}

func TestSearchRejectsMalformedSince(t *testing.T) {
	withTempHome(t)
	_, _, err := runSearchCmd(t, "http://127.0.0.1:1", "claude", "--since", "soon")
	if err == nil || !strings.Contains(err.Error(), "--since") {
		t.Fatalf("err = %v, want a --since usage error", err)
	}
}

// Digg rejects a window that starts 90 days back or more, so the widest
// relative --since is sent a minute inside that limit.
func TestSearchSinceAtNinetyDayLimitStaysInsideIt(t *testing.T) {
	withTempHome(t)
	stub := &searchStub{posts: manyPosts(1, "ai")}
	url := stub.serve(t)
	for i, since := range []string{"90d", "3m"} {
		if _, _, err := runSearchCmd(t, url, "story", "--since", since, "--json"); err != nil {
			t.Fatalf("--since %s: %v", since, err)
		}
		a := stub.requests()[i]
		sent, err := time.Parse(time.RFC3339, fmt.Sprint(a["since"]))
		if err != nil {
			t.Fatalf("--since %s sent since=%v, want an ISO 8601 datetime", since, a["since"])
		}
		if inside := 90*24*time.Hour - time.Since(sent); inside < 30*time.Second || inside > 61*time.Second {
			t.Fatalf("--since %s sent a start %s inside the 90 day limit, want about one minute", since, inside)
		}
	}
}

// orderedWriter appends its name to a shared log on every write, so a
// test can tell which stream was written first.
type orderedWriter struct {
	name string
	log  *[]string
	buf  bytes.Buffer
}

func (w *orderedWriter) Write(p []byte) (int, error) {
	*w.log = append(*w.log, w.name)
	return w.buf.Write(p)
}

// The id record is optional, so results are printed before it is tried:
// a slow or failing store can then never hold them back.
func TestSearchPrintsResultsBeforeRecordingStoryIDs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := os.WriteFile(filepath.Join(home, ".local"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	prev := diggMCPURLOverride
	diggMCPURLOverride = (&searchStub{posts: manyPosts(3, "ai")}).serve(t)
	t.Cleanup(func() { diggMCPURLOverride = prev })

	var order []string
	stdout := &orderedWriter{name: "stdout", log: &order}
	stderr := &orderedWriter{name: "stderr", log: &order}
	var flags rootFlags
	root := newRootCmd(&flags)
	root.SetArgs([]string{"search", "story", "--json"})
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if n := len(decodeSearch(t, stdout.buf.String()).Results); n != 3 {
		t.Fatalf("got %d results, want 3", n)
	}
	if !strings.Contains(stderr.buf.String(), "story ids") {
		t.Fatalf("stderr %q should warn that story ids were not recorded", stderr.buf.String())
	}
	if len(order) == 0 || order[0] != "stdout" {
		t.Fatalf("streams were written in order %v, want the results on stdout first", order)
	}
}
