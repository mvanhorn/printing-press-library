package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// sectionStub serves Digg topic pages whose embedded stream carries a
// ranked list, the way the live pages do. ranks[topic] lists the topic's
// stories in rank order by short id; fail[topic] makes that page return
// HTTP 500. It also counts requests to the story feed, which sync must
// not read.
type sectionStub struct {
	mu       sync.Mutex
	ranks    map[string][]string
	fail     map[string]bool
	pages    []string
	feedHits int
}

func (s *sectionStub) setRanks(topic string, ids ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ranks[topic] = ids
}

func (s *sectionStub) serve(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if r.URL.Path == "/api/feed/continuous" {
			s.feedHits++
			_, _ = w.Write([]byte(`{"nextCursor":null,"posts":[{"clusterId":"feed-only","clusterUrlId":"feedonly","rank":1,"summary":{"title":"From the feed"}}]}`))
			return
		}
		topic := strings.TrimPrefix(r.URL.Path, "/")
		s.pages = append(s.pages, r.URL.Path)
		ids, ok := s.ranks[topic]
		if s.fail[topic] || !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var objs []string
		for i, id := range ids {
			objs = append(objs, fmt.Sprintf(`{"clusterId":"uuid-%s","clusterUrlId":"%s","rank":%d,"authors":[{"username":"author_%s","displayName":"Author %s"}]}`, id, id, i+1, id, id))
		}
		payload := `{"posts":[` + strings.Join(objs, ",") + `]}`
		// The page carries the list as a JavaScript string literal.
		fmt.Fprintf(w, `<html><script>self.__next_f.push([1,%s])</script></html>`, strconv.Quote(payload))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// runCLIAgainst runs one command with every Digg endpoint pointed at the
// stub (or at nothing), so no test reaches digg.com.
func runCLIAgainst(t *testing.T, srvURL string, args ...string) (string, string, error) {
	t.Helper()
	prevSite, prevFeed, prevMCP := diggSiteURL, feedURLOverride, diggMCPURLOverride
	diggSiteURL, feedURLOverride, diggMCPURLOverride = srvURL, srvURL+"/api/feed/continuous", "http://127.0.0.1:1"
	t.Cleanup(func() { diggSiteURL, feedURLOverride, diggMCPURLOverride = prevSite, prevFeed, prevMCP })

	var flags rootFlags
	root := newRootCmd(&flags)
	root.SetArgs(args)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

// Sync takes one snapshot of each topic's ranked list and never reads the
// story feed.
func TestSyncSnapshotsRankedListsWithoutReadingTheFeed(t *testing.T) {
	withTempHome(t)
	stub := &sectionStub{ranks: map[string][]string{"ai": {"aaa11111", "bbb22222", "ccc33333"}, "sports": {"ddd44444", "eee55555"}}}
	url := stub.serve(t)
	// A store last synced by the retired feed walk.
	db, closeFn, err := openStoreWaiting(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO sync_state (resource_type, last_synced_at, total_count) VALUES ('feed:ai:30d', '2026-10-01 00:00:00', 2400)`); err != nil {
		t.Fatal(err)
	}
	closeFn()

	out, _, err := runCLIAgainst(t, url, "sync", "--topics", "ai,sports", "--json")
	if err != nil {
		t.Fatalf("sync: %v\n%s", err, out)
	}
	if stub.feedHits != 0 {
		t.Errorf("sync made %d story feed requests, want 0", stub.feedHits)
	}
	if len(stub.pages) != 2 {
		t.Errorf("sync requested pages %v, want one request each for /ai and /sports", stub.pages)
	}

	statsOut, _, err := runCLIAgainst(t, url, "stats", "--json")
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	var stats map[string]any
	if err := json.Unmarshal([]byte(statsOut), &stats); err != nil {
		t.Fatalf("stats output: %v\n%s", err, statsOut)
	}
	if stats["clusters"] != float64(5) || stats["snapshots"] != float64(5) || stats["authors"] != float64(5) {
		t.Errorf("stats = %v, want 5 clusters, 5 snapshots and 5 authors from the two ranked lists", stats)
	}
	// Counters for data Digg no longer serves are gone with their commands.
	for _, gone := range []string{"events", "replacements"} {
		if _, has := stats[gone]; has {
			t.Errorf("stats still reports %q", gone)
		}
	}

	// `doctor` judges cache freshness from sync_state: each topic synced
	// has a current record, and the retired feed walk's record is gone.
	db, closeFn, err = openStoreWaiting(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	rows, err := db.Query(`SELECT resource_type, total_count FROM sync_state ORDER BY resource_type`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var state []string
	for rows.Next() {
		var key string
		var n int
		if err := rows.Scan(&key, &n); err != nil {
			t.Fatal(err)
		}
		state = append(state, fmt.Sprintf("%s=%d", key, n))
	}
	if got := strings.Join(state, " "); got != "ranked-list:ai=3 ranked-list:sports=2" {
		t.Errorf("sync_state = %q, want one record per synced topic and no feed record", got)
	}

	authorsOut, _, err := runCLIAgainst(t, url, "authors", "top", "--json")
	if err != nil {
		t.Fatalf("authors top: %v", err)
	}
	if !strings.Contains(authorsOut, "author_aaa11111") {
		t.Errorf("authors top should list an author from the snapshot:\n%s", authorsOut)
	}
}

// Two syncs give `history` a rank trajectory for a story.
func TestHistoryShowsRankAcrossTwoSyncs(t *testing.T) {
	withTempHome(t)
	stub := &sectionStub{ranks: map[string][]string{"ai": {"aaa11111", "bbb22222", "ccc33333"}}}
	url := stub.serve(t)
	if _, _, err := runCLIAgainst(t, url, "sync"); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	stub.setRanks("ai", "ccc33333", "aaa11111", "bbb22222")
	if _, _, err := runCLIAgainst(t, url, "sync"); err != nil {
		t.Fatalf("second sync: %v", err)
	}
	out, _, err := runCLIAgainst(t, url, "history", "ccc33333", "--json")
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	var hist struct {
		Snapshots []struct {
			Rank int `json:"rank"`
		} `json:"snapshots"`
	}
	if err := json.Unmarshal([]byte(out), &hist); err != nil {
		t.Fatalf("history output: %v\n%s", err, out)
	}
	if len(hist.Snapshots) != 2 || hist.Snapshots[0].Rank != 3 || hist.Snapshots[1].Rank != 1 {
		t.Fatalf("snapshots = %+v, want rank 3 then rank 1", hist.Snapshots)
	}
}

func TestSyncRejectsRetiredTopic(t *testing.T) {
	withTempHome(t)
	_, _, err := runCLIAgainst(t, "http://127.0.0.1:1", "sync", "--topics", "technology")
	if err == nil || !strings.Contains(err.Error(), "world-business") {
		t.Fatalf("err = %v, want a usage error listing the current topics", err)
	}
}

func TestSyncOneTopicFailingKeepsTheOthers(t *testing.T) {
	withTempHome(t)
	stub := &sectionStub{ranks: map[string][]string{"ai": {"aaa11111"}, "tech": {"ttt11111", "ttt22222"}}, fail: map[string]bool{"ai": true}}
	url := stub.serve(t)
	_, stderr, err := runCLIAgainst(t, url, "sync", "--topics", "ai,tech")
	if err != nil {
		t.Fatalf("sync should exit 0 when another topic succeeded: %v", err)
	}
	if !strings.Contains(stderr, "ai") {
		t.Errorf("stderr %q should name the topic that failed", stderr)
	}
	statsOut, _, err := runCLIAgainst(t, url, "stats", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(statsOut, `"clusters":2`) && !strings.Contains(statsOut, `"clusters": 2`) {
		t.Errorf("stats = %s, want the 2 tech stories kept", statsOut)
	}
}

func TestSyncEveryTopicFailingExitsNonZero(t *testing.T) {
	withTempHome(t)
	stub := &sectionStub{ranks: map[string][]string{}, fail: map[string]bool{"ai": true}}
	if _, _, err := runCLIAgainst(t, stub.serve(t), "sync", "--topics", "ai"); err == nil {
		t.Fatal("sync must fail when every topic failed")
	}
}

// The feed walk's flags went with it.
func TestSyncFeedWalkFlagsAreGone(t *testing.T) {
	withTempHome(t)
	stub := &sectionStub{ranks: map[string][]string{"ai": {"aaa11111"}}}
	url := stub.serve(t)
	for _, args := range [][]string{{"--max-pages", "2"}, {"--range", "7d"}, {"--with-details"}} {
		_, _, err := runCLIAgainst(t, url, append([]string{"sync"}, args...)...)
		if err == nil || !strings.Contains(err.Error(), "unknown flag") {
			t.Errorf("sync %v: err = %v, want unknown flag", args, err)
		}
	}
}
