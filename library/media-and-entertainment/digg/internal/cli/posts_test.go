// Tests for the `posts <clusterUrlId>` command and the `story`
// command's new `posts` envelope field. The parser layer is exercised
// end-to-end via diggparse package tests; here we wire a local
// httptest server that returns the canned cluster fixture and verify:
//
//   - cobra wiring + JSON envelope shape
//   - --by rank / --by type / --by time ordering
//   - --type filter (tweet | reply | quote | retweet)
//   - --limit clamp
//   - 1h in-store cache: second call hits cache, --no-cache forces refetch
//   - story envelope has the same posts list as the standalone command

package cli

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// postsEnvelopeForTest mirrors the JSON shape printed by `posts` so we
// can decode without importing the unexported postsEnvelope type.
type postsEnvelopeForTest struct {
	Meta    map[string]any `json:"meta"`
	Results []struct {
		PostXID    string  `json:"post_x_id"`
		PostType   string  `json:"post_type"`
		PostedAt   string  `json:"posted_at"`
		Author     struct {
			Username    string `json:"username"`
			DisplayName string `json:"display_name"`
			Rank        int    `json:"rank"`
			Category    string `json:"category"`
		} `json:"author"`
		XURL       string   `json:"xUrl"`
		Body       *string  `json:"body"`
		BodyLoaded bool     `json:"body_loaded"`
		MediaURLs  []string `json:"media_urls"`
		Repost     *struct {
			RepostingHandle string `json:"reposting_handle"`
			OriginalHandle  string `json:"original_handle"`
		} `json:"repost_context"`
	} `json:"results"`
}

// loadFixtureBytes reads a checked-in cluster fixture for use as the
// httptest server's canned response.
func loadFixtureBytes(t *testing.T, name string) []byte {
	t.Helper()
	candidates := []string{
		filepath.Join("..", "..", "testdata", name),
		filepath.Join("testdata", name),
	}
	for _, p := range candidates {
		if data, err := os.ReadFile(p); err == nil {
			return data
		}
	}
	t.Fatalf("%s not found; tried: %v", name, candidates)
	return nil
}

// startMockClusterPostsServer returns an httptest server whose root
// path matches `<base>/<clusterUrlId>` and returns the supplied HTML.
// It also tracks a request-count atomic so cache-hit tests can assert
// the second call did NOT round-trip.
func startMockClusterPostsServer(t *testing.T, html []byte) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var count atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write(html)
	}))
	t.Cleanup(srv.Close)
	return srv, &count
}

// runPostsCmd builds a fresh cobra tree, points the live posts URL at
// the supplied test server, and runs `posts <id>` with extra args.
// withTempHome (defined in authors_list_test.go) isolates the SQLite
// cache so cache-state tests don't leak across cases.
func runPostsCmd(t *testing.T, mockBase, id string, extra ...string) (string, string, error) {
	t.Helper()
	prev := clusterPostsURLOverride
	clusterPostsURLOverride = mockBase
	t.Cleanup(func() { clusterPostsURLOverride = prev })

	var flags rootFlags
	root := newRootCmd(&flags)
	args := append([]string{"posts", id, "--json"}, extra...)
	root.SetArgs(args)

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

func TestPostsCmd_BuddhismDefaultByRank(t *testing.T) {
	withTempHome(t)
	html := loadFixtureBytes(t, "cluster-buddhism-65idu2x5.html")
	srv, _ := startMockClusterPostsServer(t, html)

	out, _, err := runPostsCmd(t, srv.URL, "65idu2x5")
	if err != nil {
		t.Fatalf("posts: %v\n%s", err, out)
	}
	var env postsEnvelopeForTest
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(env.Results) != 9 {
		t.Fatalf("got %d posts, want 9", len(env.Results))
	}
	if env.Meta["source"] != "live" {
		t.Errorf("meta.source = %v, want live", env.Meta["source"])
	}
	// Default --by rank: results sorted by author.rank ascending.
	for i := 1; i < len(env.Results); i++ {
		ri, rj := env.Results[i-1].Author.Rank, env.Results[i].Author.Rank
		if ri == 0 {
			ri = 1 << 30
		}
		if rj == 0 {
			rj = 1 << 30
		}
		if ri > rj {
			t.Errorf("rank-asc violated at index %d: %d > %d", i, ri, rj)
		}
	}
	// First result should be the lowest-rank author (tszzl @ rank 32).
	if env.Results[0].Author.Username != "tszzl" {
		t.Errorf("top result by rank = @%s, want @tszzl", env.Results[0].Author.Username)
	}
	// Spot-check that the first post has body & xUrl.
	first := env.Results[0]
	if first.PostXID != "2053022747765457005" {
		t.Errorf("first post id = %s, want 2053022747765457005", first.PostXID)
	}
	if first.XURL != "https://x.com/tszzl/status/2053022747765457005" {
		t.Errorf("first xUrl = %q", first.XURL)
	}
	if first.Body == nil || *first.Body != "hmm" {
		got := "<nil>"
		if first.Body != nil {
			got = *first.Body
		}
		t.Errorf("first body = %q, want %q", got, "hmm")
	}
}

func TestPostsCmd_ByTypeOrdering(t *testing.T) {
	withTempHome(t)
	html := loadFixtureBytes(t, "cluster-buddhism-65idu2x5.html")
	srv, _ := startMockClusterPostsServer(t, html)

	out, _, err := runPostsCmd(t, srv.URL, "65idu2x5", "--by", "type")
	if err != nil {
		t.Fatalf("posts --by type: %v", err)
	}
	var env postsEnvelopeForTest
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	order := map[string]int{"tweet": 0, "quote": 1, "reply": 2, "retweet": 3}
	for i := 1; i < len(env.Results); i++ {
		oa, aok := order[env.Results[i-1].PostType]
		ob, bok := order[env.Results[i].PostType]
		if !aok {
			oa = 99
		}
		if !bok {
			ob = 99
		}
		if oa > ob {
			t.Errorf("by type ordering violated at %d: %s before %s", i, env.Results[i-1].PostType, env.Results[i].PostType)
		}
	}
}

func TestPostsCmd_TypeFilterTweetOnly(t *testing.T) {
	withTempHome(t)
	html := loadFixtureBytes(t, "cluster-buddhism-65idu2x5.html")
	srv, _ := startMockClusterPostsServer(t, html)

	out, _, err := runPostsCmd(t, srv.URL, "65idu2x5", "--type", "tweet")
	if err != nil {
		t.Fatalf("posts --type tweet: %v", err)
	}
	var env postsEnvelopeForTest
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(env.Results) == 0 {
		t.Fatal("expected at least one tweet-typed post; got 0")
	}
	for _, r := range env.Results {
		if r.PostType != "tweet" {
			t.Errorf("--type tweet leaked %q post (id=%s)", r.PostType, r.PostXID)
		}
	}
}

func TestPostsCmd_LimitClampsAfterFilter(t *testing.T) {
	withTempHome(t)
	html := loadFixtureBytes(t, "cluster-buddhism-65idu2x5.html")
	srv, _ := startMockClusterPostsServer(t, html)

	out, _, err := runPostsCmd(t, srv.URL, "65idu2x5", "--limit", "3")
	if err != nil {
		t.Fatalf("posts --limit 3: %v", err)
	}
	var env postsEnvelopeForTest
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(env.Results) != 3 {
		t.Errorf("--limit 3 returned %d, want 3", len(env.Results))
	}
}

func TestPostsCmd_CacheHitOnSecondCall(t *testing.T) {
	withTempHome(t)
	html := loadFixtureBytes(t, "cluster-buddhism-65idu2x5.html")
	srv, count := startMockClusterPostsServer(t, html)

	// First call: live fetch, populates cache.
	out1, _, err := runPostsCmd(t, srv.URL, "65idu2x5")
	if err != nil {
		t.Fatalf("first posts call: %v", err)
	}
	var env1 postsEnvelopeForTest
	_ = json.Unmarshal([]byte(out1), &env1)
	if env1.Meta["source"] != "live" {
		t.Errorf("first call source = %v, want live", env1.Meta["source"])
	}
	hitsAfterFirst := count.Load()

	// Second call: should serve from cache; request count unchanged.
	out2, _, err := runPostsCmd(t, srv.URL, "65idu2x5")
	if err != nil {
		t.Fatalf("second posts call: %v", err)
	}
	var env2 postsEnvelopeForTest
	_ = json.Unmarshal([]byte(out2), &env2)
	if env2.Meta["source"] != "local" {
		t.Errorf("second call source = %v, want local (cache hit)", env2.Meta["source"])
	}
	if got := count.Load(); got != hitsAfterFirst {
		t.Errorf("expected no second network call (cache hit); got hits=%d, want %d", got, hitsAfterFirst)
	}
	// Result count parity.
	if len(env1.Results) != len(env2.Results) {
		t.Errorf("cache mismatch: live=%d, cached=%d", len(env1.Results), len(env2.Results))
	}
}

func TestPostsCmd_NoCacheForcesRefetch(t *testing.T) {
	withTempHome(t)
	html := loadFixtureBytes(t, "cluster-buddhism-65idu2x5.html")
	srv, count := startMockClusterPostsServer(t, html)

	// Warm the cache.
	if _, _, err := runPostsCmd(t, srv.URL, "65idu2x5"); err != nil {
		t.Fatalf("warmup: %v", err)
	}
	hitsAfterWarmup := count.Load()

	// --no-cache: must hit the network even though cache is fresh.
	out, _, err := runPostsCmd(t, srv.URL, "65idu2x5", "--no-cache")
	if err != nil {
		t.Fatalf("posts --no-cache: %v", err)
	}
	var env postsEnvelopeForTest
	_ = json.Unmarshal([]byte(out), &env)
	if env.Meta["source"] != "live" {
		t.Errorf("--no-cache source = %v, want live", env.Meta["source"])
	}
	if got := count.Load(); got <= hitsAfterWarmup {
		t.Errorf("--no-cache did not trigger a fresh fetch; hits=%d, prior=%d", got, hitsAfterWarmup)
	}
}

func TestStoryCmd_EnvelopeIncludesPostsField(t *testing.T) {
	// Seed the local store with a cluster row that the `story` command
	// can read, then mock /ai/<id> with a fixture so the new `posts`
	// envelope field has data to render.
	seed := withTempHome(t)
	seed(nil) // ensure schema exists

	dbPath := filepath.Join(os.Getenv("HOME"), ".local", "share", "digg-pp-cli", "data.db")
	if err := seedClusterRow(t, dbPath, "c-65idu2x5", "65idu2x5", "Buddhism"); err != nil {
		t.Fatalf("seedClusterRow: %v", err)
	}

	html := loadFixtureBytes(t, "cluster-buddhism-65idu2x5.html")
	srv, _ := startMockClusterPostsServer(t, html)

	prev := clusterPostsURLOverride
	clusterPostsURLOverride = srv.URL
	t.Cleanup(func() { clusterPostsURLOverride = prev })

	var flags rootFlags
	root := newRootCmd(&flags)
	root.SetArgs([]string{"story", "65idu2x5", "--json"})
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	if err := root.Execute(); err != nil {
		t.Fatalf("story: %v\n%s", err, stderr.String())
	}

	var env map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("invalid story JSON: %v", err)
	}
	postsField, ok := env["posts"]
	if !ok {
		t.Fatalf("story envelope missing `posts` field; got keys: %v", keysOf(env))
	}
	postsArr, ok := postsField.([]any)
	if !ok {
		t.Fatalf("posts field is not an array; got %T", postsField)
	}
	if len(postsArr) != 9 {
		t.Errorf("story posts count = %d, want 9", len(postsArr))
	}

	// Compare against standalone `posts` command for the same cluster.
	out2, _, err := runPostsCmd(t, srv.URL, "65idu2x5")
	if err != nil {
		t.Fatalf("posts standalone: %v", err)
	}
	var env2 postsEnvelopeForTest
	if err := json.Unmarshal([]byte(out2), &env2); err != nil {
		t.Fatalf("posts standalone JSON: %v", err)
	}
	if len(env2.Results) != len(postsArr) {
		t.Errorf("story.posts has %d, posts <id> has %d — should match", len(postsArr), len(env2.Results))
	}
}

func TestPostsCmd_HelpDocumentsFlags(t *testing.T) {
	var flags rootFlags
	root := newRootCmd(&flags)
	root.SetArgs([]string{"posts", "--help"})
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	if err := root.Execute(); err != nil {
		t.Fatalf("posts --help: %v", err)
	}
	help := buf.String()
	for _, want := range []string{"--by", "--type", "--limit", "rank", "type", "time"} {
		if !strings.Contains(help, want) {
			t.Errorf("posts --help missing %q; got:\n%s", want, help)
		}
	}
}

// keysOf returns the keys of a map[string]any sorted for deterministic
// error output.
func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// seedClusterRow inserts a minimal digg_clusters row so the `story`
// command has something to read. We bypass UpsertCluster (which
// requires a fully-formed diggparse.Cluster) and write the columns
// the story SELECT statement reads.
func seedClusterRow(t *testing.T, dbPath, clusterID, urlID, label string) error {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = db.Exec(`
		INSERT INTO digg_clusters (cluster_id, cluster_url_id, label, title, tldr, current_rank, fetched_at, last_seen_at)
		VALUES (?,?,?,?,?,?,?,?)
		ON CONFLICT(cluster_id) DO UPDATE SET cluster_url_id = excluded.cluster_url_id, label = excluded.label
	`, clusterID, urlID, label, label, label+" tldr", 1, now, now)
	return err
}
