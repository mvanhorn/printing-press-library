package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// feedStub stands in for Digg's /api/feed/continuous. Each topic has
// perTopic stories; story i of a topic has rank i+1 and a Digg Score that
// falls with rank and differs by topic, so a merge across topics has one
// right order. Like Digg, it serves 20 stories a page and answers an
// unknown view with HTTP 400.
type feedStub struct {
	perTopic int
	status   int // non-zero: every request fails with this status
	mu       sync.Mutex
	queries  []url.Values
}

var feedStubTopics = []string{"ai", "tech", "world-business", "sports", "science", "gaming", "entertainment"}

// feedStubScore is the Digg Score of the story at 0-based position i in
// the topic at index t of feedStubTopics.
func feedStubScore(t, i int) int { return 1000 - 10*i - t }

func (f *feedStub) serve(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/feed/continuous" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		f.mu.Lock()
		f.queries = append(f.queries, q)
		f.mu.Unlock()
		if f.status != 0 {
			w.WriteHeader(f.status)
			_, _ = w.Write([]byte(`{"error":"feed is down for maintenance"}`))
			return
		}
		topicIdx := -1
		for i, name := range feedStubTopics {
			if name == q.Get("topic") {
				topicIdx = i
			}
		}
		if topicIdx < 0 || (q.Get("view") != "top" && q.Get("view") != "trending") {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"Invalid query parameters"}`))
			return
		}
		start := 0
		if c := q.Get("cursor"); c != "" {
			start, _ = strconv.Atoi(strings.TrimPrefix(c, "o"))
		}
		end := min(start+20, f.perTopic)
		posts := make([]map[string]any, 0, 20)
		for i := start; i < end; i++ {
			posts = append(posts, map[string]any{
				"clusterId":    fmt.Sprintf("0190a1b2-c3d4-7e5f-8a6b-%04d%08d", topicIdx, i),
				"clusterUrlId": fmt.Sprintf("%s%03d", q.Get("topic")[:2], i),
				"type":         "cluster", "entryStatus": "ranked",
				"rank": i + 1, "peakRank": nil, "delta": nil,
				"diggScore": feedStubScore(topicIdx, i),
				"postCount": 13 + i, "sourcePostCount": 13 + i, "uniqueAuthors": 9 + i,
				"createdAt": "2026-11-11T18:30:25.760315+00:00",
				"summary":   map[string]string{"title": fmt.Sprintf("%s story %d", q.Get("topic"), i), "description": "What happened in story " + strconv.Itoa(i)},
			})
		}
		var next any
		if end < f.perTopic {
			next = "o" + strconv.Itoa(end)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"nextCursor": next, "nowMs": time.Now().UnixMilli(), "posts": posts, "view": q.Get("view"), "range": q.Get("range")})
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func (f *feedStub) requests() []url.Values {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]url.Values(nil), f.queries...)
}

// runTopCmd runs `top` with every Digg endpoint pointed at the stub (or
// at nothing), so no test reaches digg.com.
func runTopCmd(t *testing.T, srvURL string, args ...string) (string, string, error) {
	t.Helper()
	prevSite, prevFeed, prevMCP := diggSiteURL, feedURLOverride, diggMCPURLOverride
	diggSiteURL, feedURLOverride, diggMCPURLOverride = srvURL, srvURL+"/api/feed/continuous", "http://127.0.0.1:1"
	t.Cleanup(func() { diggSiteURL, feedURLOverride, diggMCPURLOverride = prevSite, prevFeed, prevMCP })

	var flags rootFlags
	root := newRootCmd(&flags)
	root.SetArgs(append([]string{"top"}, args...))
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

type topOutput struct {
	Meta    map[string]any   `json:"meta"`
	Results []map[string]any `json:"results"`
}

func decodeTop(t *testing.T, stdout string) topOutput {
	t.Helper()
	var env topOutput
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	return env
}

// With nothing in the local store, `top` for one topic returns that
// topic's ranked feed read live, and --agent keeps every field.
func TestTopOneTopicReadsLiveFeedWithEmptyStore(t *testing.T) {
	withTempHome(t)
	stub := &feedStub{perTopic: 20}
	stdout, _, err := runTopCmd(t, stub.serve(t), "--topic", "sports", "--limit", "10", "--agent")
	if err != nil {
		t.Fatalf("top: %v", err)
	}
	env := decodeTop(t, stdout)
	if len(env.Results) != 10 {
		t.Fatalf("got %d results, want 10: %s", len(env.Results), stdout)
	}
	want := map[string]any{
		"rank":          float64(1),
		"clusterId":     "0190a1b2-c3d4-7e5f-8a6b-000300000000",
		"clusterUrlId":  "sp000",
		"url":           "https://digg.com/sports/sp000",
		"topic":         "sports",
		"title":         "sports story 0",
		"description":   "What happened in story 0",
		"diggScore":     float64(997),
		"postCount":     float64(13),
		"uniqueAuthors": float64(9),
		"publishedAt":   "2026-11-11T18:30:25.760315+00:00",
	}
	for k, v := range want {
		if env.Results[0][k] != v {
			t.Errorf("results[0].%s = %v, want %v", k, env.Results[0][k], v)
		}
	}
	for i, r := range env.Results {
		if r["rank"] != float64(i+1) {
			t.Errorf("results[%d].rank = %v, want %d", i, r["rank"], i+1)
		}
		if u, _ := r["url"].(string); !strings.HasPrefix(u, "https://digg.com/sports/") {
			t.Errorf("results[%d].url = %q, want a /sports/ URL", i, u)
		}
	}
	reqs := stub.requests()
	if len(reqs) != 1 {
		t.Fatalf("made %d feed requests, want 1", len(reqs))
	}
	for k, v := range map[string]string{"topic": "sports", "view": "top", "range": "24h", "published": "1"} {
		if reqs[0].Get(k) != v {
			t.Errorf("feed request %s = %q, want %q", k, reqs[0].Get(k), v)
		}
	}
}

// Without --topic, `top` reads each of the seven topics once and merges
// the rows by Digg Score, each row tagged with its own topic.
func TestTopWithoutTopicMergesEveryTopicByScore(t *testing.T) {
	withTempHome(t)
	stub := &feedStub{perTopic: 20}
	stdout, _, err := runTopCmd(t, stub.serve(t), "--limit", "20", "--agent")
	if err != nil {
		t.Fatalf("top: %v", err)
	}
	env := decodeTop(t, stdout)
	if len(env.Results) != 20 {
		t.Fatalf("got %d results, want 20: %s", len(env.Results), stdout)
	}
	// The stub's scores are 1000-10*position-topicIndex, so the best 20
	// across topics are positions 0 and 1 of all seven topics, then
	// position 2 of the first six, in topic order within each position.
	for i, r := range env.Results {
		pos, topicIdx := i/7, i%7
		topic := feedStubTopics[topicIdx]
		if r["diggScore"] != float64(feedStubScore(topicIdx, pos)) || r["topic"] != topic {
			t.Errorf("results[%d] = topic %v score %v, want topic %s score %d", i, r["topic"], r["diggScore"], topic, feedStubScore(topicIdx, pos))
		}
		if r["rank"] != float64(pos+1) {
			t.Errorf("results[%d].rank = %v, want the story's rank in its own topic, %d", i, r["rank"], pos+1)
		}
		if u, _ := r["url"].(string); !strings.HasPrefix(u, "https://digg.com/"+topic+"/") {
			t.Errorf("results[%d].url = %q, want a URL under /%s/", i, u, topic)
		}
	}
	seen := map[string]int{}
	for _, q := range stub.requests() {
		seen[q.Get("topic")]++
	}
	if len(seen) != 7 {
		t.Fatalf("feed requests by topic = %v, want all seven topics", seen)
	}
	for topic, n := range seen {
		if n != 1 {
			t.Errorf("topic %s was read %d times, want once", topic, n)
		}
	}
}

// A --limit past one page of 20 follows the feed's cursor for that topic.
func TestTopLimitPastOnePageFollowsCursor(t *testing.T) {
	withTempHome(t)
	stub := &feedStub{perTopic: 45}
	stdout, _, err := runTopCmd(t, stub.serve(t), "--topic", "ai", "--limit", "30", "--json")
	if err != nil {
		t.Fatalf("top: %v", err)
	}
	env := decodeTop(t, stdout)
	if len(env.Results) != 30 {
		t.Fatalf("got %d results, want 30", len(env.Results))
	}
	if env.Results[29]["rank"] != float64(30) {
		t.Errorf("results[29].rank = %v, want 30", env.Results[29]["rank"])
	}
	reqs := stub.requests()
	if len(reqs) != 2 || reqs[1].Get("cursor") != "o20" {
		t.Fatalf("feed requests = %v, want two with the second carrying the first page's cursor", reqs)
	}
}

func TestTopFeedErrorExitsNonZeroWithoutRows(t *testing.T) {
	withTempHome(t)
	stub := &feedStub{perTopic: 20, status: http.StatusServiceUnavailable}
	stdout, _, err := runTopCmd(t, stub.serve(t), "--topic", "sports", "--agent")
	if err == nil {
		t.Fatalf("top succeeded against a failing feed:\n%s", stdout)
	}
	if !strings.Contains(err.Error(), "503") || !strings.Contains(err.Error(), "feed is down for maintenance") {
		t.Errorf("error %q should carry the endpoint's status and message", err)
	}
	if strings.Contains(stdout, "results") || strings.Contains(stdout, "[") {
		t.Errorf("a failed read printed rows: %q", stdout)
	}
}

func TestTopTrendingViewReturnsRows(t *testing.T) {
	withTempHome(t)
	stub := &feedStub{perTopic: 20}
	stdout, _, err := runTopCmd(t, stub.serve(t), "--topic", "tech", "--view", "trending", "--limit", "5", "--agent")
	if err != nil {
		t.Fatalf("top --view trending: %v", err)
	}
	if env := decodeTop(t, stdout); len(env.Results) != 5 {
		t.Fatalf("got %d results, want 5: %s", len(env.Results), stdout)
	}
	if reqs := stub.requests(); len(reqs) != 1 || reqs[0].Get("view") != "trending" {
		t.Fatalf("feed requests = %v, want one with view=trending", reqs)
	}
}

func TestTopRejectsUnknownViewBeforeAnyRequest(t *testing.T) {
	withTempHome(t)
	stub := &feedStub{perTopic: 20}
	_, _, err := runTopCmd(t, stub.serve(t), "--view", "rising")
	if err == nil || ExitCode(err) != 2 {
		t.Fatalf("err = %v (exit %d), want a usage error", err, ExitCode(err))
	}
	if !strings.Contains(err.Error(), "trending") {
		t.Errorf("error %q should list the valid views", err)
	}
	if n := len(stub.requests()); n != 0 {
		t.Errorf("made %d feed requests for an invalid view, want 0", n)
	}
}

// Feed rows carry no topic, so the cross-topic "all" feed cannot give a
// story its URL; `top` takes the seven real topics only.
func TestTopRejectsAllAndUnknownTopics(t *testing.T) {
	withTempHome(t)
	stub := &feedStub{perTopic: 20}
	for _, topic := range []string{"all", "technology"} {
		_, _, err := runTopCmd(t, stub.serve(t), "--topic", topic)
		if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "world-business") {
			t.Errorf("--topic %s: err = %v, want a usage error listing the topics", topic, err)
		}
	}
	if n := len(stub.requests()); n != 0 {
		t.Errorf("made %d feed requests for invalid topics, want 0", n)
	}
}
