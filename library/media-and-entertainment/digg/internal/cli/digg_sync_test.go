package cli

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// mockFeed serves /api/feed/continuous pages per topic. pages[topic] is
// the number of 20-story pages the topic has; failPage[topic] makes that
// 1-based page return HTTP 500. Every non-feed path (the HTML section
// pages) returns 404, so sync's scrape step warns and moves on.
type mockFeed struct {
	pages    map[string]int
	failPage map[string]int
	mu       sync.Mutex
	hits     map[string]int
}

func (m *mockFeed) serve(t *testing.T) *httptest.Server {
	t.Helper()
	m.hits = map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/feed/continuous" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		topic := q.Get("topic")
		page := 1
		if c := q.Get("cursor"); c != "" {
			page, _ = strconv.Atoi(strings.TrimPrefix(c, "p"))
		}
		m.mu.Lock()
		m.hits[topic]++
		m.mu.Unlock()
		if m.failPage[topic] == page {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		total, ok := m.pages[topic]
		if !ok || page > total {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"Invalid query parameters"}`))
			return
		}
		posts := make([]map[string]any, 0, 20)
		for i := 0; i < 20; i++ {
			id := fmt.Sprintf("%s-%d-%d", topic, page, i)
			posts = append(posts, map[string]any{
				"clusterId": id, "clusterUrlId": "u" + id, "rank": (page-1)*20 + i + 1,
				"createdAt": time.Now().Add(-time.Duration(page) * 24 * time.Hour).UTC().Format(time.RFC3339),
				"postCount": 5, "uniqueAuthors": 3,
				"summary": map[string]string{"title": "Story " + id, "description": "about " + topic},
			})
		}
		var next any
		if page < total {
			next = fmt.Sprintf("p%d", page+1)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"nextCursor": next, "nowMs": time.Now().UnixMilli(), "posts": posts})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func runSyncCmd(t *testing.T, srvURL string, args ...string) (string, string, error) {
	t.Helper()
	prevSite, prevFeed := diggSiteURL, feedURLOverride
	diggSiteURL, feedURLOverride = srvURL, srvURL+"/api/feed/continuous"
	t.Cleanup(func() { diggSiteURL, feedURLOverride = prevSite, prevFeed })

	var flags rootFlags
	root := newRootCmd(&flags)
	root.SetArgs(append([]string{"sync"}, args...))
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

func openSyncedDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(defaultDBPath("digg-pp-cli")))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func topicCount(t *testing.T, db *sql.DB, topic string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM digg_cluster_topics WHERE topic = ?`, topic).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func syncKeyExists(t *testing.T, db *sql.DB, key string) bool {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sync_state WHERE resource_type = ?`, key).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n > 0
}

func TestSyncStoresEachTopicFromFeed(t *testing.T) {
	withTempHome(t)
	m := &mockFeed{pages: map[string]int{"ai": 2, "tech": 1}}
	srv := m.serve(t)
	out, _, err := runSyncCmd(t, srv.URL, "--topics", "ai,tech")
	if err != nil {
		t.Fatalf("sync: %v\n%s", err, out)
	}
	db := openSyncedDB(t)
	if got := topicCount(t, db, "ai"); got != 40 {
		t.Fatalf("ai stories = %d, want 40", got)
	}
	if got := topicCount(t, db, "tech"); got != 20 {
		t.Fatalf("tech stories = %d, want 20", got)
	}
	if !syncKeyExists(t, db, "feed:ai:30d") || !syncKeyExists(t, db, "feed:tech:30d") {
		t.Fatal("completed walks must record freshness for both topics")
	}
}

func TestSyncRejectsRetiredTopic(t *testing.T) {
	withTempHome(t)
	_, _, err := runSyncCmd(t, "http://127.0.0.1:1", "--topics", "technology")
	if err == nil || !strings.Contains(err.Error(), "world-business") {
		t.Fatalf("err = %v, want a usage error listing the current sections", err)
	}
}

func TestSyncPartialTopicFailureKeepsProgress(t *testing.T) {
	withTempHome(t)
	m := &mockFeed{pages: map[string]int{"ai": 3, "tech": 1}, failPage: map[string]int{"ai": 2}}
	srv := m.serve(t)
	_, stderr, err := runSyncCmd(t, srv.URL, "--topics", "ai,tech")
	if err != nil {
		t.Fatalf("sync should exit 0 when another topic succeeded: %v", err)
	}
	if !strings.Contains(stderr, "ai") {
		t.Fatalf("stderr %q should warn about the ai failure", stderr)
	}
	db := openSyncedDB(t)
	if got := topicCount(t, db, "ai"); got != 20 {
		t.Fatalf("ai stories = %d, want page 1's 20 kept", got)
	}
	if syncKeyExists(t, db, "feed:ai:30d") {
		t.Fatal("a failed walk must not record freshness")
	}
	if !syncKeyExists(t, db, "feed:tech:30d") {
		t.Fatal("the successful topic must record freshness")
	}
}

func TestSyncAllTopicsFailingExitsNonZero(t *testing.T) {
	withTempHome(t)
	m := &mockFeed{pages: map[string]int{"ai": 1}, failPage: map[string]int{"ai": 1}}
	srv := m.serve(t)
	if _, _, err := runSyncCmd(t, srv.URL, "--topics", "ai"); err == nil {
		t.Fatal("sync must fail when every topic failed")
	}
}

func TestSyncMaxPagesCapsWalk(t *testing.T) {
	withTempHome(t)
	m := &mockFeed{pages: map[string]int{"ai": 5}}
	srv := m.serve(t)
	out, _, err := runSyncCmd(t, srv.URL, "--topics", "ai", "--max-pages", "2")
	if err != nil {
		t.Fatal(err)
	}
	if m.hits["ai"] != 2 {
		t.Fatalf("feed requests = %d, want 2", m.hits["ai"])
	}
	if !strings.Contains(out, "max-pages") {
		t.Fatalf("output %q should say --max-pages cut the walk short", out)
	}
	if syncKeyExists(t, openSyncedDB(t), "feed:ai:30d") {
		t.Fatal("a capped walk that only reached 2 days back must not mark the 30d range fresh")
	}
}

func TestSyncCappedWalkReachingRangeStartCountsAsSynced(t *testing.T) {
	withTempHome(t)
	// The mock's page N stories are N days old, so 28 pages reach 28 days
	// into the 30d range.
	m := &mockFeed{pages: map[string]int{"ai": 40}}
	srv := m.serve(t)
	if _, _, err := runSyncCmd(t, srv.URL, "--topics", "ai", "--max-pages", "28"); err != nil {
		t.Fatal(err)
	}
	if !syncKeyExists(t, openSyncedDB(t), "feed:ai:30d") {
		t.Fatal("a capped walk that reached the start of the range should count as synced")
	}
}
