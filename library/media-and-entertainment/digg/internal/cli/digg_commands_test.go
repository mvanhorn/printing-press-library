// Tests for the `search` command. Search reads the local copy of Digg's
// story feed and tops it up from /api/feed/continuous when a requested
// topic has no recent completed sync. Each test seeds a temp store and
// points the feed at an httptest stub via feedURLOverride.

package cli

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/diggstore"
)

// searchEnvelopeForTest mirrors the JSON shape printed by `search`.
type searchEnvelopeForTest struct {
	Meta    map[string]any `json:"meta"`
	Results []struct {
		ClusterID     string `json:"clusterId"`
		ClusterURLID  string `json:"clusterUrlId"`
		Title         string `json:"title,omitempty"`
		TLDR          string `json:"tldr,omitempty"`
		Rank          int    `json:"rank,omitempty"`
		PostCount     int    `json:"postCount,omitempty"`
		UniqueAuthors int    `json:"uniqueAuthors,omitempty"`
		FirstPostAge  string `json:"firstPostAge,omitempty"`
	} `json:"results"`
}

type testStory struct {
	id, title, desc string
	age             time.Duration
}

func (s testStory) json() map[string]any {
	return map[string]any{
		"clusterId": s.id, "clusterUrlId": "u-" + s.id, "rank": 3, "diggScore": 50,
		"createdAt": time.Now().Add(-s.age).UTC().Format(time.RFC3339),
		"postCount": 14, "uniqueAuthors": 9,
		"summary": map[string]string{"title": s.title, "description": s.desc},
	}
}

func (s testStory) feedStory(t *testing.T) client.FeedStory {
	t.Helper()
	b, _ := json.Marshal(s.json())
	var fs client.FeedStory
	if err := json.Unmarshal(b, &fs); err != nil {
		t.Fatal(err)
	}
	return fs
}

// seedTopic stores stories under topic. fresh records a completed 30d
// sync just now, so search treats the topic as current.
func seedTopic(t *testing.T, topic string, fresh bool, stories ...testStory) {
	t.Helper()
	db, err := sql.Open("sqlite", defaultDBPath("digg-pp-cli"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := diggstore.EnsureSchema(db); err != nil {
		t.Fatal(err)
	}
	var page []client.FeedStory
	for _, s := range stories {
		page = append(page, s.feedStory(t))
	}
	if err := diggstore.UpsertFeedPage(db, page, topic, "30d", time.Now()); err != nil {
		t.Fatal(err)
	}
	if fresh {
		if err := diggstore.MarkFeedSynced(db, topic, "30d", len(stories), time.Now()); err != nil {
			t.Fatal(err)
		}
	}
}

// feedStub serves the same stories on every feed page and never ends the
// cursor, so a walk only stops at its page budget. fail makes every
// request return HTTP 500.
type feedStub struct {
	stories []testStory
	fail    bool
	mu      sync.Mutex
	queries []string
}

func (f *feedStub) serve(t *testing.T) string {
	t.Helper()
	page := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.queries = append(f.queries, r.URL.RawQuery)
		page++
		n := page
		f.mu.Unlock()
		if f.fail {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		posts := []map[string]any{}
		for _, s := range f.stories {
			posts = append(posts, s.json())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"nextCursor": fmt.Sprintf("c%d", n), "posts": posts})
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func (f *feedStub) hits() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.queries)
}

func runSearchCmd(t *testing.T, feedURL string, args ...string) (string, string, error) {
	t.Helper()
	prev := feedURLOverride
	feedURLOverride = feedURL
	t.Cleanup(func() { feedURLOverride = prev })

	var flags rootFlags
	root := newRootCmd(&flags)
	root.SetArgs(append([]string{"search"}, args...))
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

func decodeSearch(t *testing.T, stdout string) searchEnvelopeForTest {
	t.Helper()
	var env searchEnvelopeForTest
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	return env
}

func resultIDs(env searchEnvelopeForTest) []string {
	ids := []string{}
	for _, r := range env.Results {
		ids = append(ids, r.ClusterID)
	}
	return ids
}

// The exact call last30days makes, asserting every field lib/digg.py reads.
func TestSearchLast30daysCallShape(t *testing.T) {
	withTempHome(t)
	seedTopic(t, "ai", true, testStory{"c1", "Claude Code ships plugins", "Anthropic adds a plugin system", 3 * 24 * time.Hour})
	stub := &feedStub{}
	stdout, _, err := runSearchCmd(t, stub.serve(t), "claude", "--since", "30d", "--agent", "--limit", "5")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	env := decodeSearch(t, stdout)
	if len(env.Results) != 1 {
		t.Fatalf("got %d results, want 1: %s", len(env.Results), stdout)
	}
	r := env.Results[0]
	if r.ClusterURLID != "u-c1" || r.Title != "Claude Code ships plugins" || r.TLDR != "Anthropic adds a plugin system" ||
		r.Rank != 3 || r.PostCount != 14 || r.UniqueAuthors != 9 || r.FirstPostAge != "3d" {
		t.Fatalf("result fields wrong: %+v", r)
	}
	if env.Meta["source"] != "local" || env.Meta["stale"] != false {
		t.Fatalf("meta = %v, want source=local stale=false", env.Meta)
	}
	if stub.hits() != 0 {
		t.Fatalf("fresh store made %d feed requests, want 0", stub.hits())
	}
}

func TestSearchStaleStoreTopsUpFromFeed(t *testing.T) {
	withTempHome(t)
	seedTopic(t, "ai", false, testStory{"stored", "Claude stored story", "", 24 * time.Hour})
	stub := &feedStub{stories: []testStory{{"live", "Claude live story", "", time.Hour}}}
	stdout, _, err := runSearchCmd(t, stub.serve(t), "claude", "--json")
	if err != nil {
		t.Fatal(err)
	}
	env := decodeSearch(t, stdout)
	ids := strings.Join(resultIDs(env), ",")
	if !strings.Contains(ids, "stored") || !strings.Contains(ids, "live") {
		t.Fatalf("results %s, want both the stored and the topped-up story", ids)
	}
	if env.Meta["source"] != "local+live" {
		t.Fatalf("meta.source = %v, want local+live", env.Meta["source"])
	}
	if env.Meta["stale"] != true {
		t.Fatalf("a top-up does not replace a sync; meta.stale = %v, want true", env.Meta["stale"])
	}
}

func TestSearchStaleStoreWithFailingFeedServesStoredRows(t *testing.T) {
	withTempHome(t)
	seedTopic(t, "ai", false, testStory{"stored", "Claude stored story", "", 24 * time.Hour})
	stub := &feedStub{fail: true}
	stdout, stderr, err := runSearchCmd(t, stub.serve(t), "claude", "--json")
	if err != nil {
		t.Fatalf("search should still answer from the store: %v", err)
	}
	env := decodeSearch(t, stdout)
	if len(env.Results) != 1 || env.Meta["stale"] != true || env.Meta["source"] != "local" {
		t.Fatalf("results=%v meta=%v, want the stored row, stale=true, source=local", resultIDs(env), env.Meta)
	}
	if !strings.Contains(stderr, "sync") {
		t.Fatalf("stderr %q should warn that the copy is stale and suggest sync", stderr)
	}
}

func TestSearchEmptyStoreWithFailingFeedErrors(t *testing.T) {
	withTempHome(t)
	stub := &feedStub{fail: true}
	_, _, err := runSearchCmd(t, stub.serve(t), "claude", "--json")
	if err == nil || !strings.Contains(err.Error(), "sync") {
		t.Fatalf("err = %v, want a failure telling the user to run sync", err)
	}
}

func TestSearchTopicScopesResults(t *testing.T) {
	withTempHome(t)
	seedTopic(t, "ai", true, testStory{"ai-1", "Claude news", "", time.Hour})
	seedTopic(t, "sports", true, testStory{"sp-1", "Claude scores a goal", "", time.Hour})
	stdout, _, err := runSearchCmd(t, (&feedStub{}).serve(t), "claude", "--topic", "sports", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if ids := resultIDs(decodeSearch(t, stdout)); len(ids) != 1 || ids[0] != "sp-1" {
		t.Fatalf("results %v, want only the sports story", ids)
	}
}

func TestSearchSinceFiltersOnCreatedAt(t *testing.T) {
	withTempHome(t)
	seedTopic(t, "ai", true,
		testStory{"new", "Claude new", "", 24 * time.Hour},
		testStory{"old", "Claude old", "", 3 * 24 * time.Hour})
	stdout, _, err := runSearchCmd(t, (&feedStub{}).serve(t), "claude", "--since", "2d", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if ids := resultIDs(decodeSearch(t, stdout)); len(ids) != 1 || ids[0] != "new" {
		t.Fatalf("results %v, want only the 1-day-old story", ids)
	}
}

func TestSearchLongSinceTopsUpWithAllRange(t *testing.T) {
	withTempHome(t)
	stub := &feedStub{stories: []testStory{{"live", "Claude live", "", time.Hour}}}
	if _, _, err := runSearchCmd(t, stub.serve(t), "claude", "--since", "45d", "--json"); err != nil {
		t.Fatal(err)
	}
	if stub.hits() == 0 || !strings.Contains(stub.queries[0], "range=all") {
		t.Fatalf("queries %v, want the top-up to request range=all", stub.queries)
	}
}

func TestSearchAgeComesFromWallClock(t *testing.T) {
	withTempHome(t)
	// Stored long ago when the story was younger; age must be measured now.
	seedTopic(t, "ai", true, testStory{"aged", "Claude aged story", "", 35 * 24 * time.Hour})
	stub := &feedStub{}
	stdout, _, err := runSearchCmd(t, stub.serve(t), "claude", "--json")
	if err != nil {
		t.Fatal(err)
	}
	env := decodeSearch(t, stdout)
	if len(env.Results) != 1 || env.Results[0].FirstPostAge != "35d" {
		t.Fatalf("results %+v, want one story aged 35d", env.Results)
	}
	stdout, _, err = runSearchCmd(t, stub.serve(t), "claude", "--since", "30d", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if ids := resultIDs(decodeSearch(t, stdout)); len(ids) != 0 {
		t.Fatalf("results %v, want the 35-day-old story excluded by --since 30d", ids)
	}
}

func TestSearchTopUpPageBudget(t *testing.T) {
	withTempHome(t)
	stub := &feedStub{stories: []testStory{{"live", "Claude live", "", time.Hour}}}
	if _, _, err := runSearchCmd(t, stub.serve(t), "claude", "--topic", "ai,tech", "--json"); err != nil {
		t.Fatal(err)
	}
	if h := stub.hits(); h == 0 || h > 8 {
		t.Fatalf("top-up made %d feed requests, want 1 to 8", h)
	}
}

func TestSearchDataSourceLocalSkipsTopUp(t *testing.T) {
	withTempHome(t)
	seedTopic(t, "ai", false, testStory{"stored", "Claude stored", "", time.Hour})
	stub := &feedStub{}
	if _, _, err := runSearchCmd(t, stub.serve(t), "claude", "--data-source", "local", "--json"); err != nil {
		t.Fatal(err)
	}
	if stub.hits() != 0 {
		t.Fatalf("--data-source local made %d feed requests", stub.hits())
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

func TestSearchSelectFiltersResultsEnvelope(t *testing.T) {
	withTempHome(t)
	seedTopic(t, "ai", true, testStory{"c1", "Agents everywhere", "", 2 * 24 * time.Hour})
	stdout, _, err := runSearchCmd(t, (&feedStub{}).serve(t), "agents", "--json",
		"--select", "clusterUrlId,title,rank,postCount,uniqueAuthors,firstPostAge")
	if err != nil {
		t.Fatalf("search --select: %v", err)
	}
	env := decodeSearch(t, stdout)
	if len(env.Results) == 0 {
		t.Fatalf("expected selected results, got: %s", stdout)
	}
	first := env.Results[0]
	if first.ClusterURLID == "" || first.Title == "" || first.Rank == 0 || first.PostCount == 0 || first.UniqueAuthors == 0 || first.FirstPostAge == "" {
		t.Fatalf("selected result missing requested fields: %+v", first)
	}
	if first.ClusterID != "" {
		t.Fatalf("unrequested clusterId should be omitted, got %q", first.ClusterID)
	}
}

func TestSearchDataSourceLocalEmptyStoreErrors(t *testing.T) {
	withTempHome(t)
	_, _, err := runSearchCmd(t, (&feedStub{}).serve(t), "claude", "--data-source", "local", "--json")
	if err == nil || !strings.Contains(err.Error(), "sync") {
		t.Fatalf("err = %v, want a failure telling the user to run sync", err)
	}
}

func TestSearchStaleStoreOutsideWindowWithFailingFeedErrors(t *testing.T) {
	withTempHome(t)
	seedTopic(t, "ai", false, testStory{"old", "Claude old", "", 10 * 24 * time.Hour})
	_, _, err := runSearchCmd(t, (&feedStub{fail: true}).serve(t), "claude", "--since", "2d", "--json")
	if err == nil || !strings.Contains(err.Error(), "sync") {
		t.Fatalf("err = %v, want a failure telling the user to run sync", err)
	}
}
