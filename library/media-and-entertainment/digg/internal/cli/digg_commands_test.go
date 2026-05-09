// Tests for the `search` command's --since time-window filter (U3 of
// the digg search/roster plan).
//
// Approach: spin up an httptest server returning canned
// /api/search/stories envelopes, point the search command at it via
// searchStoriesURLOverride, and assert which results survive each
// --since window. The local-mode test seeds a temp SQLite store
// directly and checks the SQL filter against digg_clusters.first_post_at.

package cli

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/diggstore"
)

// searchEnvelopeForTest mirrors the JSON shape printed by `search`. We
// declare it locally rather than reusing searchEnvelope so the test
// doesn't have to import unexported types — but the JSON tags must match
// exactly so the unmarshal succeeds.
type searchEnvelopeForTest struct {
	Meta    map[string]any `json:"meta"`
	Results []struct {
		ClusterID     string `json:"clusterId"`
		ClusterURLID  string `json:"clusterUrlId"`
		Title         string `json:"title,omitempty"`
		Label         string `json:"label,omitempty"`
		TLDR          string `json:"tldr,omitempty"`
		Rank          int    `json:"rank,omitempty"`
		CurrentRank   int    `json:"currentRank,omitempty"`
		PostCount     int    `json:"postCount,omitempty"`
		UniqueAuthors int    `json:"uniqueAuthors,omitempty"`
		FirstPostAge  string `json:"firstPostAge,omitempty"`
	} `json:"results"`
}

// startMockSearchServer returns a server that responds to every
// /api/search/stories call with the supplied results. Callers wire it
// into the search command via the searchStoriesURLOverride sink.
//
// The mock honors the upstream `limit` query param: if a numeric limit
// is sent it clamps the response slice. That mirrors the real upstream
// (verified by curl probe in U1) so tests catch regressions where the
// CLI stops forwarding the param.
func startMockSearchServer(t *testing.T, results []map[string]any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		out := results
		if lim := r.URL.Query().Get("limit"); lim != "" {
			var n int
			if _, err := fmt.Sscanf(lim, "%d", &n); err == nil && n > 0 && n < len(out) {
				out = out[:n]
			}
		}
		body, _ := json.Marshal(map[string]any{
			"query":       r.URL.Query().Get("q"),
			"results":     out,
			"count":       len(out),
			"duration_ms": 1,
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// runSearchCmd builds the cobra tree, points the live search at the
// supplied URL, captures stdout/stderr, and runs `search <query>` with
// the extra args appended. Returns the JSON body, stderr, and any err.
//
// withTempHome is reused from authors_list_test.go (same package), so
// each test gets an isolated SQLite database — important when we mix
// live and local-mode tests in the same suite.
func runSearchCmd(t *testing.T, mockURL, query string, extra ...string) (string, string, error) {
	t.Helper()
	prev := searchStoriesURLOverride
	searchStoriesURLOverride = mockURL
	t.Cleanup(func() { searchStoriesURLOverride = prev })

	var flags rootFlags
	root := newRootCmd(&flags)
	args := append([]string{"search", query, "--json"}, extra...)
	root.SetArgs(args)

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

// canned /api/search/stories results: one 2-day-old, one 26-day-old,
// one with malformed firstPostAge, one with empty firstPostAge.
func cannedStories() []map[string]any {
	return []map[string]any{
		{
			"clusterId":     "c1",
			"clusterUrlId":  "u1",
			"rank":          1,
			"title":         "Two-day old story",
			"description":   "Recent.",
			"postCount":     5,
			"uniqueAuthors": 3,
			"firstPostAge":  "2d",
		},
		{
			"clusterId":     "c2",
			"clusterUrlId":  "u2",
			"rank":          2,
			"title":         "Twenty-six-day old story",
			"description":   "Older.",
			"postCount":     12,
			"uniqueAuthors": 7,
			"firstPostAge":  "26d",
		},
		{
			"clusterId":     "c3",
			"clusterUrlId":  "u3",
			"rank":          3,
			"title":         "Malformed-age story",
			"description":   "Upstream returned an unparseable shape.",
			"postCount":     2,
			"uniqueAuthors": 1,
			"firstPostAge":  "forever",
		},
		{
			"clusterId":     "c4",
			"clusterUrlId":  "u4",
			"rank":          4,
			"title":         "Missing-age story",
			"description":   "Upstream returned no firstPostAge at all.",
			"postCount":     1,
			"uniqueAuthors": 1,
			// firstPostAge intentionally omitted
		},
	}
}

// TestSearchSince_FiltersOutOlderThanWindow exercises the load-bearing
// path: --since 7d must drop the 26d entry and keep the 2d entry.
func TestSearchSince_FiltersOutOlderThanWindow(t *testing.T) {
	withTempHome(t) // isolate sqlite cache; not used in live mode but harmless
	srv := startMockSearchServer(t, cannedStories())

	out, _, err := runSearchCmd(t, srv.URL, "anything", "--since", "7d")
	if err != nil {
		t.Fatalf("search --since 7d: %v\nstdout=%s", err, out)
	}
	var env searchEnvelopeForTest
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}

	// c1 (2d) kept; c2 (26d) dropped; c3 (malformed) kept; c4 (empty) kept.
	wantIDs := []string{"u1", "u3", "u4"}
	gotIDs := make([]string, 0, len(env.Results))
	for _, r := range env.Results {
		gotIDs = append(gotIDs, r.ClusterURLID)
	}
	if strings.Join(gotIDs, ",") != strings.Join(wantIDs, ",") {
		t.Errorf("--since 7d: got %v, want %v", gotIDs, wantIDs)
	}

	if env.Meta["since"] != "7d" {
		t.Errorf("meta.since = %v, want 7d (filter parameter must round-trip)", env.Meta["since"])
	}
	if env.Meta["source"] != "live" {
		t.Errorf("meta.source = %v, want live", env.Meta["source"])
	}
}

// TestSearchSince_30dKeepsBoth widens the window so both 2d and 26d
// entries pass. The malformed and empty rows are also kept (parser
// failure is "include," not "drop").
func TestSearchSince_30dKeepsBoth(t *testing.T) {
	withTempHome(t)
	srv := startMockSearchServer(t, cannedStories())

	out, _, err := runSearchCmd(t, srv.URL, "anything", "--since", "30d")
	if err != nil {
		t.Fatalf("search --since 30d: %v", err)
	}
	var env searchEnvelopeForTest
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(env.Results) != 4 {
		t.Errorf("--since 30d: got %d results, want 4 (all canned rows pass)", len(env.Results))
	}
}

// TestSearchSince_NoFlagReturnsAll confirms the existing behavior is
// preserved when --since isn't passed: no filter, every result through.
// Also confirms meta.since is omitted (not "" — JSON shape contract).
func TestSearchSince_NoFlagReturnsAll(t *testing.T) {
	withTempHome(t)
	srv := startMockSearchServer(t, cannedStories())

	out, _, err := runSearchCmd(t, srv.URL, "anything")
	if err != nil {
		t.Fatalf("search (no --since): %v", err)
	}
	var env searchEnvelopeForTest
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(env.Results) != 4 {
		t.Errorf("got %d results, want 4 (no filter)", len(env.Results))
	}
	if _, ok := env.Meta["since"]; ok {
		t.Errorf("meta.since should be absent when --since not passed; got %v", env.Meta["since"])
	}
}

// TestSearchSince_RejectsMalformed surfaces a parse error to the user
// up front rather than silently no-op'ing the filter. The exit must be
// non-zero (usage error) so scripts notice.
func TestSearchSince_RejectsMalformed(t *testing.T) {
	withTempHome(t)
	srv := startMockSearchServer(t, cannedStories())

	_, _, err := runSearchCmd(t, srv.URL, "anything", "--since", "forever")
	if err == nil {
		t.Fatal("--since forever should error, got nil")
	}
	if !strings.Contains(err.Error(), "--since") {
		t.Errorf("error should mention --since; got: %v", err)
	}
}

// TestSearchSince_FilterBeforeLimit confirms the documented ordering:
// filter runs before limit. With --limit 2 --since 7d, the 26d row is
// dropped first; the limit is then applied to what remains. Without
// "filter first" the user could end up with 0–1 rows even though there
// were 3 in-window matches.
func TestSearchSince_FilterBeforeLimit(t *testing.T) {
	withTempHome(t)
	// Seed the mock with 1x 2d, 1x 26d, 2x 1d so the filter must drop one
	// row to make space within the limit.
	stories := []map[string]any{
		{"clusterId": "c1", "clusterUrlId": "u1", "rank": 1, "title": "1d a", "firstPostAge": "1d"},
		{"clusterId": "c2", "clusterUrlId": "u2", "rank": 2, "title": "26d b", "firstPostAge": "26d"},
		{"clusterId": "c3", "clusterUrlId": "u3", "rank": 3, "title": "1d c", "firstPostAge": "1d"},
		{"clusterId": "c4", "clusterUrlId": "u4", "rank": 4, "title": "2d d", "firstPostAge": "2d"},
	}
	srv := startMockSearchServer(t, stories)

	out, _, err := runSearchCmd(t, srv.URL, "anything", "--since", "7d", "--limit", "2")
	if err != nil {
		t.Fatalf("search --since 7d --limit 2: %v", err)
	}
	var env searchEnvelopeForTest
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(env.Results) != 2 {
		t.Errorf("limit=2 with --since 7d: got %d results, want 2 (filter runs first, then limit caps)", len(env.Results))
	}
	// Specifically, u1 + u3 — the first two in-window rows. u2 (26d) must
	// be dropped before the limit is hit.
	for _, r := range env.Results {
		if r.ClusterURLID == "u2" {
			t.Errorf("u2 (26d) should have been dropped by --since 7d before limit applied")
		}
	}
}

// TestSearchSince_LocalMode seeds digg_clusters with old + new rows and
// confirms the SQL window applies on first_post_at. Documents the
// schema choice: there is no first_seen_at column on digg_clusters; we
// filter on first_post_at because its semantics match Digg's
// firstPostAge (when the cluster's first observed post landed). Rows
// with NULL/empty first_post_at are kept so the policy matches the
// live-mode "keep on parse failure" rule.
func TestSearchSince_LocalMode(t *testing.T) {
	withTempHome(t)

	// Open the test DB the same way the CLI does so the search command
	// reads what we just wrote.
	dbPath := filepath.Join(os.Getenv("HOME"), ".local", "share", "digg-pp-cli", "data.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := diggstore.EnsureSchema(db); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	twoDaysAgo := now.Add(-2 * 24 * time.Hour).Format(time.RFC3339)
	fortyDaysAgo := now.Add(-40 * 24 * time.Hour).Format(time.RFC3339)
	nowStr := now.Format(time.RFC3339Nano)

	// Direct INSERT: bypassing UpsertCluster keeps the test focused on
	// the column we actually care about (first_post_at) and sidesteps the
	// fixture-loading cost of constructing diggparse.Cluster instances.
	insert := func(id, urlID, title, tldr string, currentRank int, firstPostAt string) {
		t.Helper()
		_, err := db.Exec(`
			INSERT INTO digg_clusters (cluster_id, cluster_url_id, label, title, tldr, current_rank, first_post_at, fetched_at, last_seen_at)
			VALUES (?,?,?,?,?,?,?,?,?)
		`, id, urlID, title, title, tldr, currentRank, firstPostAt, nowStr, nowStr)
		if err != nil {
			t.Fatalf("insert %s: %v", id, err)
		}
		_, err = db.Exec(`INSERT INTO digg_clusters_fts (cluster_id, cluster_url_id, label, title, tldr, source_title) VALUES (?,?,?,?,?,?)`,
			id, urlID, title, title, tldr, "")
		if err != nil {
			t.Fatalf("fts %s: %v", id, err)
		}
	}
	insert("c-recent", "u-recent", "fresh widget news", "the widget cluster, recent", 1, twoDaysAgo)
	insert("c-old", "u-old", "ancient widget news", "the widget cluster, old", 2, fortyDaysAgo)
	insert("c-null", "u-null", "undated widget news", "no first_post_at recorded", 3, "")

	var flags rootFlags
	root := newRootCmd(&flags)
	root.SetArgs([]string{"search", "widget", "--json", "--data-source", "local", "--since", "30d"})
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	if err := root.Execute(); err != nil {
		t.Fatalf("search local --since 30d: %v\nstderr=%s", err, stderr.String())
	}
	var env searchEnvelopeForTest
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if env.Meta["source"] != "local" {
		t.Errorf("meta.source = %v, want local", env.Meta["source"])
	}
	gotIDs := make(map[string]bool, len(env.Results))
	for _, r := range env.Results {
		gotIDs[r.ClusterURLID] = true
	}
	if !gotIDs["u-recent"] {
		t.Errorf("expected u-recent (2 days old) to pass --since 30d filter; got %v", env.Results)
	}
	if gotIDs["u-old"] {
		t.Errorf("expected u-old (40 days old) to be filtered out by --since 30d; got %v", env.Results)
	}
	if !gotIDs["u-null"] {
		t.Errorf("expected u-null (NULL first_post_at) to be kept (matches live-mode 'keep on parse failure' policy); got %v", env.Results)
	}
}

// TestSearchSince_HelpMentionsFlag confirms `search --help` documents
// --since with format examples so agents can discover it.
func TestSearchSince_HelpMentionsFlag(t *testing.T) {
	var flags rootFlags
	root := newRootCmd(&flags)
	root.SetArgs([]string{"search", "--help"})
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	if err := root.Execute(); err != nil {
		t.Fatalf("search --help: %v", err)
	}
	help := buf.String()
	if !strings.Contains(help, "--since") {
		t.Errorf("--help output should mention --since; got:\n%s", help)
	}
	// Format examples must appear so the agent doesn't have to guess.
	for _, want := range []string{"30d", "Nh", "Nd"} {
		if !strings.Contains(help, want) {
			t.Errorf("--help should include format hint %q; got:\n%s", want, help)
		}
	}
}
