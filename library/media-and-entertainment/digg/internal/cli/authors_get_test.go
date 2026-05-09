// Tests for the `authors get <handle>` command (U2 of the digg
// search/roster plan).
//
// Approach: spin up an httptest server returning canned
// /api/search/users envelopes, point the command at it via
// searchUsersURLOverride, and assert exact-match / fuzzy / off-1000
// distance / cache-cold-fallback behaviour.
//
// The off-1000 distance path resolves the rank-1000 anchor from the
// local digg_authors cache. Tests use withTempHome (defined in
// authors_list_test.go) to isolate the SQLite store; the cache-cold
// scenario additionally wires a mock /ai/1000 server through
// roster1000URLOverride so the live-fallback path is exercised
// without a real network call.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/diggparse"
)

// authorGetEnvelopeForTest mirrors both the exact-match and
// fuzzy-list shapes returned by `authors get`. We declare the union
// locally with optional fields so the same struct can decode either
// envelope; the test inspects which side is populated to know which
// branch the command took.
type authorGetEnvelopeForTest struct {
	Meta    map[string]any         `json:"meta"`
	Result  *authorGetResultForTst `json:"result,omitempty"`
	Results []authorGetResultForTst `json:"results,omitempty"`
}

type authorGetResultForTst struct {
	XID                    string  `json:"x_id,omitempty"`
	Username               string  `json:"username"`
	DisplayName            string  `json:"display_name,omitempty"`
	FollowersCount         int     `json:"followers_count"`
	Category               *string `json:"category"`
	CurrentRank            *int    `json:"current_rank"`
	SimilarityScore        float64 `json:"similarity_score"`
	IsPrefixMatch          bool    `json:"is_prefix_match"`
	XURL                   string  `json:"xUrl,omitempty"`
	GithubURL              string  `json:"githubUrl,omitempty"`
	TierStatus             string  `json:"tier_status,omitempty"`
	SubjectPeerFollowCount *int    `json:"subject_peer_follow_count,omitempty"`
	NearestIn1000          *struct {
		Username        string  `json:"username"`
		Rank            int     `json:"rank"`
		FollowersCount  int     `json:"followers_count"`
		PeerFollowCount int     `json:"peer_follow_count"`
		Score           float64 `json:"score,omitempty"`
	} `json:"nearest_in_1000,omitempty"`
	PeerFollowGap *int   `json:"peer_follow_gap,omitempty"`
	MatchType     string `json:"match_type,omitempty"`
	// FollowersGap remains here as a *deliberate negative-assertion
	// hook: U2-fix removed the field from the live response, but
	// tests parse it so they can assert it stays absent. If the
	// field is ever reintroduced upstream this surfaces as a test
	// failure rather than silent drift.
	FollowersGap *int `json:"followers_gap,omitempty"`
}

// startMockUsersServer returns an httptest server that responds to
// every /api/search/users call with the supplied results.
//
// Mimics the real upstream by (a) sorting results by similarity_score
// desc before applying any client-supplied `limit` and (b) honoring
// `limit` to clamp the slice. The real upstream verified by curl
// probe 2026-05-09 returns highest-similarity first; the mock has to
// match that so CLI-side resort tests reflect production behaviour.
func startMockUsersServer(t *testing.T, results []map[string]any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Defensive copy so re-runs of the handler don't mutate the
		// caller's slice.
		out := make([]map[string]any, len(results))
		copy(out, results)
		// Mirror upstream similarity-desc sort so mock and live
		// behaviour stay aligned.
		sortBySimilarityDesc(out)
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

// sortBySimilarityDesc sorts in place by similarity_score desc.
// Defensive helper for the mock; tolerates missing/non-numeric
// scores by treating them as 0.
func sortBySimilarityDesc(rows []map[string]any) {
	pick := func(m map[string]any) float64 {
		v, ok := m["similarity_score"]
		if !ok || v == nil {
			return 0
		}
		switch n := v.(type) {
		case float64:
			return n
		case int:
			return float64(n)
		}
		return 0
	}
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && pick(rows[j]) > pick(rows[j-1]); j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}
}

// runAuthorsGet builds the cobra tree, points the live search at the
// supplied URL, captures stdout/stderr, and runs `authors get
// <handle> --json`. Extra args are appended after.
func runAuthorsGet(t *testing.T, mockURL, handle string, extra ...string) (string, string, error) {
	t.Helper()
	prev := searchUsersURLOverride
	searchUsersURLOverride = mockURL
	t.Cleanup(func() { searchUsersURLOverride = prev })

	var flags rootFlags
	root := newRootCmd(&flags)
	args := append([]string{"authors", "get", handle, "--json"}, extra...)
	root.SetArgs(args)

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

// startMockUserPeerFollowServer returns an httptest server that
// responds to GET /<handle> with an HTML page containing a
// <meta name="description"> tag whose copy carries
// "followed by N tracked AI influencers" — the phrase the live
// upstream uses on /u/x/<handle>. peerCounts maps handle (lower) to
// the integer the page should advertise. Handles missing from the
// map produce a 404 (matches the upstream's "we don't track this
// handle" path, which the client returns as 0/nil).
func startMockUserPeerFollowServer(t *testing.T, peerCounts map[string]int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The CLI's override base is joined with "/<handle>" by
		// fetchSubjectPeerFollowCount; the path is therefore
		// /<handle> with no extra prefix.
		handle := strings.TrimPrefix(r.URL.Path, "/")
		handle = strings.ToLower(handle)
		n, ok := peerCounts[handle]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body := fmt.Sprintf(`<html><head><meta name="description" content="%s is tracked in the latest Digg AI graph — followed by %d tracked AI influencers on X."/></head><body></body></html>`, handle, n)
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// withUserPeerFollowOverride wires the package-level
// userPeerFollowURLOverride to the supplied base URL for the
// duration of one test, restoring the previous value afterwards.
func withUserPeerFollowOverride(t *testing.T, baseURL string) {
	t.Helper()
	prev := userPeerFollowURLOverride
	userPeerFollowURLOverride = baseURL
	t.Cleanup(func() { userPeerFollowURLOverride = prev })
}

// cannedUsersLogangraham returns one in-1000 record. Matches the
// upstream shape verified via curl probe 2026-05-09.
func cannedUsersLogangraham() []map[string]any {
	return []map[string]any{
		{
			"x_id":              "47195143",
			"username":          "logangraham",
			"display_name":      "Logan Graham",
			"profile_image_url": "https://example.test/logan.jpg",
			"followers_count":   16948,
			"category":          "Researcher",
			"current_rank":      991,
			"similarity_score":  1.0,
			"is_prefix_match":   true,
		},
	}
}

// cannedUsersMVH returns one off-1000 record. Matches the upstream
// shape verified via curl probe (current_rank=null, category=null).
func cannedUsersMVH() []map[string]any {
	return []map[string]any{
		{
			"x_id":              "9999999",
			"username":          "mvanhorn",
			"display_name":      "Matt Van Horn",
			"profile_image_url": "https://example.test/mvh.jpg",
			"followers_count":   21911,
			"category":          nil,
			"current_rank":      nil,
			"similarity_score":  1.0,
			"is_prefix_match":   true,
		},
	}
}

// seedRank1000 adds a rank-1000 row to the in-test digg_authors
// table so the off-1000 anchor lookup has data to find. Uses the
// public UpsertRoster1000 path so the test reflects production
// upserts (not a hand-rolled INSERT shape that could drift).
//
// Carries `FollowedByCount: 90` — the AI-1000 peer-follow count the
// distance view (peer_follow_gap) compares against. 90 is the
// observed live value for the rank-1000 author at probe time
// 2026-05-09, which keeps the seeded fixture honest about the
// metric magnitudes.
func seedRank1000(t *testing.T, seed func([]diggparse.Roster1000Author)) {
	t.Helper()
	prev := 1010
	change := -10
	seed([]diggparse.Roster1000Author{
		{
			Rank:            1000,
			TargetXID:       "1000-anchor",
			Username:        "anchorbot",
			DisplayName:     "Anchor Bot",
			FollowersCount:  9000,
			FollowedByCount: 90,
			Score:           12.5,
			Category:        "AI Safety",
			PreviousRank:    &prev,
			RankChange:      &change,
		},
	})
}

// TestAuthorsGet_InThousandExactMatch covers the load-bearing path
// for in-1000 handles: exact case-sensitive match returns the single
// rich record, tier_status is "in_1000", no anchor fields.
func TestAuthorsGet_InThousandExactMatch(t *testing.T) {
	withTempHome(t)
	srv := startMockUsersServer(t, cannedUsersLogangraham())

	out, _, err := runAuthorsGet(t, srv.URL, "logangraham")
	if err != nil {
		t.Fatalf("authors get: %v\nstdout=%s", err, out)
	}
	var env authorGetEnvelopeForTest
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if env.Result == nil {
		t.Fatalf("expected exact-match envelope (.result), got fuzzy: %s", out)
	}
	if env.Results != nil && len(env.Results) > 0 {
		t.Errorf(".results should be omitted on exact match; got %d entries", len(env.Results))
	}
	if env.Result.Username != "logangraham" {
		t.Errorf("username = %q, want logangraham", env.Result.Username)
	}
	if env.Result.CurrentRank == nil || *env.Result.CurrentRank != 991 {
		t.Errorf("current_rank = %v, want 991", env.Result.CurrentRank)
	}
	if env.Result.TierStatus != "in_1000" {
		t.Errorf("tier_status = %q, want in_1000", env.Result.TierStatus)
	}
	if env.Result.Category == nil || *env.Result.Category != "Researcher" {
		t.Errorf("category = %v, want Researcher", env.Result.Category)
	}
	if env.Result.XURL != "https://x.com/logangraham" {
		t.Errorf("xUrl = %q, want https://x.com/logangraham", env.Result.XURL)
	}
	// In-1000 path: no anchor fields. Assert both the new
	// peer_follow_gap and the retired followers_gap are absent so
	// we catch any accidental field reintroduction.
	if env.Result.NearestIn1000 != nil {
		t.Errorf("nearest_in_1000 should be omitted on in-1000 path; got %+v", env.Result.NearestIn1000)
	}
	if env.Result.PeerFollowGap != nil {
		t.Errorf("peer_follow_gap should be omitted on in-1000 path; got %d", *env.Result.PeerFollowGap)
	}
	if env.Result.SubjectPeerFollowCount != nil {
		t.Errorf("subject_peer_follow_count should be omitted on in-1000 path; got %d", *env.Result.SubjectPeerFollowCount)
	}
	if env.Result.FollowersGap != nil {
		t.Errorf("legacy followers_gap should never appear; got %d", *env.Result.FollowersGap)
	}
	if env.Meta["match_type"] != "exact" {
		t.Errorf("meta.match_type = %v, want exact", env.Meta["match_type"])
	}
	if env.Meta["source"] != "live" {
		t.Errorf("meta.source = %v, want live", env.Meta["source"])
	}
	if env.Meta["tier_status_resolved"] != true {
		t.Errorf("meta.tier_status_resolved = %v, want true", env.Meta["tier_status_resolved"])
	}
}

// TestAuthorsGet_OffThousandWithWarmCache exercises the off-1000
// path against a pre-seeded digg_authors cache. The anchor must be
// populated with peer_follow_count from the seeded
// followed_by_count, and peer_follow_gap must be a signed integer
// (anchor_peer_follow - subject_peer_follow). The subject's
// peer-follow count is fetched from a mocked /u/x/<handle> server.
//
// mvanhorn live values (probe 2026-05-09): peer-follow count is 19;
// rank-1000 anchor's followed_by_count is 90 (seeded above). The
// seeded fixture mirrors those numbers so the gap math (90 - 19 =
// 71) matches the value an agent would compute against the live
// upstream.
func TestAuthorsGet_OffThousandWithWarmCache(t *testing.T) {
	seed := withTempHome(t)
	seedRank1000(t, seed)

	usersSrv := startMockUsersServer(t, cannedUsersMVH())
	peerSrv := startMockUserPeerFollowServer(t, map[string]int{"mvanhorn": 19})
	withUserPeerFollowOverride(t, peerSrv.URL)

	out, _, err := runAuthorsGet(t, usersSrv.URL, "mvanhorn")
	if err != nil {
		t.Fatalf("authors get mvanhorn: %v\nstdout=%s", err, out)
	}
	var env authorGetEnvelopeForTest
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if env.Result == nil {
		t.Fatalf("expected exact-match envelope, got: %s", out)
	}
	if env.Result.CurrentRank != nil {
		t.Errorf("current_rank should be null for off-1000; got %d", *env.Result.CurrentRank)
	}
	if env.Result.TierStatus != "off_1000" {
		t.Errorf("tier_status = %q, want off_1000", env.Result.TierStatus)
	}
	if env.Result.XURL != "https://x.com/mvanhorn" {
		t.Errorf("xUrl = %q", env.Result.XURL)
	}
	if env.Result.NearestIn1000 == nil {
		t.Fatal("nearest_in_1000 should be populated when warm cache has rank=1000 row")
	}
	if env.Result.NearestIn1000.Username != "anchorbot" {
		t.Errorf("nearest_in_1000.username = %q, want anchorbot", env.Result.NearestIn1000.Username)
	}
	if env.Result.NearestIn1000.Rank != 1000 {
		t.Errorf("nearest_in_1000.rank = %d, want 1000", env.Result.NearestIn1000.Rank)
	}
	if env.Result.NearestIn1000.FollowersCount != 9000 {
		t.Errorf("nearest_in_1000.followers_count = %d, want 9000 (kept for context, not the ranking metric)", env.Result.NearestIn1000.FollowersCount)
	}
	if env.Result.NearestIn1000.PeerFollowCount != 90 {
		t.Errorf("nearest_in_1000.peer_follow_count = %d, want 90", env.Result.NearestIn1000.PeerFollowCount)
	}
	if env.Result.SubjectPeerFollowCount == nil {
		t.Fatal("subject_peer_follow_count should be populated when /u/x mock resolves")
	}
	if *env.Result.SubjectPeerFollowCount != 19 {
		t.Errorf("subject_peer_follow_count = %d, want 19", *env.Result.SubjectPeerFollowCount)
	}
	if env.Result.PeerFollowGap == nil {
		t.Fatal("peer_follow_gap should be set when both anchor and subject resolve")
	}
	// 90 (anchor peer-follow) - 19 (mvanhorn peer-follow) = 71.
	if *env.Result.PeerFollowGap != 71 {
		t.Errorf("peer_follow_gap = %d, want 71 (anchor.peer_follow_count - subject.peer_follow_count)", *env.Result.PeerFollowGap)
	}
	if env.Result.FollowersGap != nil {
		t.Errorf("legacy followers_gap should not appear; got %d", *env.Result.FollowersGap)
	}
	if env.Meta["tier_status_resolved"] != true {
		t.Errorf("meta.tier_status_resolved should be true with warm cache + subject fetch; got %v", env.Meta["tier_status_resolved"])
	}
}

// TestAuthorsGet_CaseInsensitiveExact confirms that `LoganGraham`
// finds `logangraham` as an exact match (not a fuzzy fallback).
// The match itself is upstream-driven, but the case-insensitive
// resolve to "exact" is CLI-side logic.
func TestAuthorsGet_CaseInsensitiveExact(t *testing.T) {
	withTempHome(t)
	srv := startMockUsersServer(t, cannedUsersLogangraham())

	out, _, err := runAuthorsGet(t, srv.URL, "LoganGraham")
	if err != nil {
		t.Fatalf("authors get LoganGraham: %v", err)
	}
	var env authorGetEnvelopeForTest
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if env.Result == nil {
		t.Fatalf("LoganGraham should resolve to exact match, not fuzzy; got: %s", out)
	}
	if env.Meta["match_type"] != "exact" {
		t.Errorf("match_type = %v, want exact", env.Meta["match_type"])
	}
}

// TestAuthorsGet_NoMatchReturnsEmptyFuzzy exercises the "handle
// genuinely doesn't exist" path: empty results envelope with
// match_type fuzzy and exit code 0 — distinct from an error so
// callers can fan out lookups without per-handle error checks.
func TestAuthorsGet_NoMatchReturnsEmptyFuzzy(t *testing.T) {
	withTempHome(t)
	srv := startMockUsersServer(t, []map[string]any{}) // no results

	out, _, err := runAuthorsGet(t, srv.URL, "nobodyactuallyhere12345xyz")
	if err != nil {
		t.Fatalf("authors get (no match) should exit 0; got: %v", err)
	}
	var env authorGetEnvelopeForTest
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if env.Result != nil {
		t.Errorf("result should be omitted on no-match; got %+v", env.Result)
	}
	if env.Results == nil {
		// JSON unmarshal of `"results": []` puts an empty slice here, not nil.
		// If results is nil, the envelope didn't have the key — that's the bug.
		t.Errorf("results array should be present (possibly empty); got nil envelope shape: %s", out)
	}
	if len(env.Results) != 0 {
		t.Errorf("expected 0 fuzzy results; got %d", len(env.Results))
	}
	if env.Meta["match_type"] != "fuzzy" {
		t.Errorf("match_type = %v, want fuzzy", env.Meta["match_type"])
	}
	if env.Meta["count"].(float64) != 0 {
		t.Errorf("meta.count = %v, want 0", env.Meta["count"])
	}
}

// TestAuthorsGet_FuzzyResultsSortedBySimilarity confirms that when
// no exact match is present, the CLI sorts the upstream candidates
// by similarity_score desc and respects the --limit flag. Mirrors
// upstream behaviour but tests the CLI-side defensive resort.
func TestAuthorsGet_FuzzyResultsSortedBySimilarity(t *testing.T) {
	withTempHome(t)
	// Three "logan…" candidates, intentionally out of order so the
	// CLI's sort matters.
	results := []map[string]any{
		{
			"x_id": "1", "username": "logansmith", "display_name": "Logan Smith",
			"followers_count": 100, "category": nil, "current_rank": nil,
			"similarity_score": 0.7, "is_prefix_match": true,
		},
		{
			"x_id": "2", "username": "loganzero", "display_name": "Logan Zero",
			"followers_count": 50, "category": nil, "current_rank": nil,
			"similarity_score": 0.95, "is_prefix_match": true,
		},
		{
			"x_id": "3", "username": "loganalpha", "display_name": "Logan Alpha",
			"followers_count": 200, "category": nil, "current_rank": nil,
			"similarity_score": 0.8, "is_prefix_match": true,
		},
	}
	seed := withTempHome(t) // re-seed: anchor lookup will fail (no fallback) but command should still work
	seedRank1000(t, seed)

	srv := startMockUsersServer(t, results)
	out, _, err := runAuthorsGet(t, srv.URL, "logan", "--limit", "2")
	if err != nil {
		t.Fatalf("authors get logan: %v\n%s", err, out)
	}
	var env authorGetEnvelopeForTest
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if env.Result != nil {
		t.Fatalf("logan has no exact match; expected fuzzy envelope, got result: %+v", env.Result)
	}
	if len(env.Results) != 2 {
		t.Fatalf("--limit 2: got %d fuzzy results, want 2", len(env.Results))
	}
	// Sorted by similarity desc: loganzero (0.95), loganalpha (0.8).
	if env.Results[0].Username != "loganzero" {
		t.Errorf("[0] = %q, want loganzero (highest similarity)", env.Results[0].Username)
	}
	if env.Results[1].Username != "loganalpha" {
		t.Errorf("[1] = %q, want loganalpha", env.Results[1].Username)
	}
	if env.Meta["match_type"] != "fuzzy" {
		t.Errorf("match_type = %v, want fuzzy", env.Meta["match_type"])
	}
	for _, r := range env.Results {
		if r.MatchType != "fuzzy" {
			t.Errorf("@%s match_type = %q, want fuzzy", r.Username, r.MatchType)
		}
	}
}

// TestAuthorsGet_CacheColdLiveFallback exercises the off-1000
// distance path against an empty cache: the command must do a live
// /ai/1000 fetch (mocked here), populate the cache, and return a
// complete payload with the peer-follow distance view. The mocked
// rank-1000 row carries `followed_by_count: 84` so the cache-cold
// path also exercises the peer_follow_count round-trip through the
// upsert/read pipeline rather than just the anchor lookup.
func TestAuthorsGet_CacheColdLiveFallback(t *testing.T) {
	withTempHome(t) // empty cache

	// Build a tiny RSC-shaped HTML payload that ParseRoster1000 will
	// decode. The parser looks for `target_x_id` inside RSC chunks
	// pushed via self.__next_f.push. We construct one chunk with a
	// rank-1000 row (with followed_by_count populated) plus one
	// rank-1 row so the parser sees a real roster.
	rsc := `[{"rank":1,"target_x_id":"1","username":"sama","display_name":"sama","followers_count":1000,"score":99.9,"previousRank":null,"rankChange":null,"followed_by_count":600},{"rank":1000,"target_x_id":"1000","username":"livefallback","display_name":"Live Fallback","followers_count":7777,"score":1.0,"previousRank":null,"rankChange":null,"followed_by_count":84}]`
	rscJSON, _ := json.Marshal(rsc)
	html := []byte(`<html><body>` +
		`<script>self.__next_f=self.__next_f||[];self.__next_f.push([1,` + string(rscJSON) + `])</script>` +
		`</body></html>`)

	rosterSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write(html)
	}))
	t.Cleanup(rosterSrv.Close)
	prev := roster1000URLOverride
	roster1000URLOverride = rosterSrv.URL
	t.Cleanup(func() { roster1000URLOverride = prev })

	peerSrv := startMockUserPeerFollowServer(t, map[string]int{"mvanhorn": 19})
	withUserPeerFollowOverride(t, peerSrv.URL)

	usersSrv := startMockUsersServer(t, cannedUsersMVH())
	out, stderr, err := runAuthorsGet(t, usersSrv.URL, "mvanhorn")
	if err != nil {
		t.Fatalf("authors get mvanhorn (cache cold): %v\nstderr=%s", err, stderr)
	}
	var env authorGetEnvelopeForTest
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if env.Result == nil {
		t.Fatalf("expected exact-match result, got: %s", out)
	}
	if env.Result.NearestIn1000 == nil {
		t.Fatalf("cache-cold fallback did not populate nearest_in_1000; stderr=%s", stderr)
	}
	if env.Result.NearestIn1000.Username != "livefallback" {
		t.Errorf("nearest_in_1000.username = %q, want livefallback (the rank=1000 row from mocked /ai/1000)",
			env.Result.NearestIn1000.Username)
	}
	if env.Result.NearestIn1000.FollowersCount != 7777 {
		t.Errorf("nearest_in_1000.followers_count = %d, want 7777", env.Result.NearestIn1000.FollowersCount)
	}
	if env.Result.NearestIn1000.PeerFollowCount != 84 {
		t.Errorf("nearest_in_1000.peer_follow_count = %d, want 84 (round-tripped from RSC followed_by_count)",
			env.Result.NearestIn1000.PeerFollowCount)
	}
	if env.Result.SubjectPeerFollowCount == nil {
		t.Fatal("subject_peer_follow_count should be set after /u/x mock resolves")
	}
	if *env.Result.SubjectPeerFollowCount != 19 {
		t.Errorf("subject_peer_follow_count = %d, want 19", *env.Result.SubjectPeerFollowCount)
	}
	if env.Result.PeerFollowGap == nil {
		t.Fatal("peer_follow_gap should be set after live fallback populates anchor and subject fetch resolves")
	}
	// 84 (anchor) - 19 (subject) = 65.
	if *env.Result.PeerFollowGap != 65 {
		t.Errorf("peer_follow_gap = %d, want 65", *env.Result.PeerFollowGap)
	}
	if env.Result.FollowersGap != nil {
		t.Errorf("legacy followers_gap should not appear; got %d", *env.Result.FollowersGap)
	}
	if env.Meta["tier_status_resolved"] != true {
		t.Errorf("tier_status_resolved = %v, want true after successful fallback", env.Meta["tier_status_resolved"])
	}
}

// TestAuthorsGet_OffThousandFetchFailureGraceful covers the
// failure-mode for the off-1000 anchor: empty cache + /ai/1000
// fetch errors. The command must NOT exit non-zero — it should
// return the rest of the record with tier_status off_1000, omit
// nearest_in_1000 / peer_follow_gap, and set
// meta.tier_status_resolved: false so callers detect the partial
// result.
//
// We deliberately *do* wire a working /u/x mock here so the test
// confirms a successful subject-side fetch alongside an anchor
// failure still surfaces subject_peer_follow_count (i.e. partial
// data flows from whichever side resolved). peer_follow_gap stays
// omitted because gap requires both sides.
func TestAuthorsGet_OffThousandFetchFailureGraceful(t *testing.T) {
	withTempHome(t) // empty cache

	// /ai/1000 mock returns 500 to simulate fetch failure.
	rosterSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(rosterSrv.Close)
	prev := roster1000URLOverride
	roster1000URLOverride = rosterSrv.URL
	t.Cleanup(func() { roster1000URLOverride = prev })

	peerSrv := startMockUserPeerFollowServer(t, map[string]int{"mvanhorn": 19})
	withUserPeerFollowOverride(t, peerSrv.URL)

	usersSrv := startMockUsersServer(t, cannedUsersMVH())
	out, stderr, err := runAuthorsGet(t, usersSrv.URL, "mvanhorn")
	if err != nil {
		t.Fatalf("authors get on roster fetch failure should NOT error; got: %v\nstderr=%s", err, stderr)
	}
	var env authorGetEnvelopeForTest
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if env.Result == nil {
		t.Fatalf("expected result envelope; got: %s", out)
	}
	if env.Result.TierStatus != "off_1000" {
		t.Errorf("tier_status = %q, want off_1000 even when anchor unresolved", env.Result.TierStatus)
	}
	if env.Result.NearestIn1000 != nil {
		t.Errorf("nearest_in_1000 should be omitted on anchor failure; got %+v", env.Result.NearestIn1000)
	}
	if env.Result.PeerFollowGap != nil {
		t.Errorf("peer_follow_gap should be omitted on anchor failure; got %d", *env.Result.PeerFollowGap)
	}
	if env.Result.FollowersGap != nil {
		t.Errorf("legacy followers_gap should never appear; got %d", *env.Result.FollowersGap)
	}
	// Subject-side fetch resolved independently; surface what we
	// do have so callers can still see the subject's standing.
	if env.Result.SubjectPeerFollowCount == nil || *env.Result.SubjectPeerFollowCount != 19 {
		t.Errorf("subject_peer_follow_count = %v, want 19 (subject fetch resolved even when anchor did not)",
			env.Result.SubjectPeerFollowCount)
	}
	if env.Meta["tier_status_resolved"] != false {
		t.Errorf("meta.tier_status_resolved = %v, want false", env.Meta["tier_status_resolved"])
	}
	// Stderr should mention the fetch failure so operators can debug.
	if !strings.Contains(stderr, "/ai/1000") {
		t.Errorf("stderr should log /ai/1000 fetch failure; got: %s", stderr)
	}
}

// TestAuthorsGet_OffThousandSubjectFetchFailureGraceful covers the
// inverse failure-mode: anchor resolves (warm cache), but the
// /u/x/<handle> page fetch fails. The command must keep
// nearest_in_1000 (so the rank-1000 anchor is still visible to the
// caller), omit subject_peer_follow_count and peer_follow_gap, set
// meta.tier_status_resolved: false, and exit 0.
func TestAuthorsGet_OffThousandSubjectFetchFailureGraceful(t *testing.T) {
	seed := withTempHome(t)
	seedRank1000(t, seed) // anchor resolves cleanly

	// /u/x mock returns 500 to simulate fetch failure (the
	// http.StatusNotFound path is the legitimate "Digg doesn't
	// track this handle" case and returns 0/nil — we want a
	// hard failure here).
	subjectSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(subjectSrv.Close)
	withUserPeerFollowOverride(t, subjectSrv.URL)

	usersSrv := startMockUsersServer(t, cannedUsersMVH())
	out, stderr, err := runAuthorsGet(t, usersSrv.URL, "mvanhorn")
	if err != nil {
		t.Fatalf("authors get on /u/x fetch failure should NOT error; got: %v\nstderr=%s", err, stderr)
	}
	var env authorGetEnvelopeForTest
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if env.Result == nil {
		t.Fatalf("expected result envelope; got: %s", out)
	}
	if env.Result.TierStatus != "off_1000" {
		t.Errorf("tier_status = %q, want off_1000", env.Result.TierStatus)
	}
	if env.Result.NearestIn1000 == nil {
		t.Errorf("nearest_in_1000 should still be present when anchor resolves but subject fetch fails (caller still sees the rank-1000 anchor)")
	}
	if env.Result.SubjectPeerFollowCount != nil {
		t.Errorf("subject_peer_follow_count should be omitted on /u/x fetch failure; got %d", *env.Result.SubjectPeerFollowCount)
	}
	if env.Result.PeerFollowGap != nil {
		t.Errorf("peer_follow_gap should be omitted on /u/x fetch failure; got %d", *env.Result.PeerFollowGap)
	}
	if env.Result.FollowersGap != nil {
		t.Errorf("legacy followers_gap should never appear; got %d", *env.Result.FollowersGap)
	}
	if env.Meta["tier_status_resolved"] != false {
		t.Errorf("meta.tier_status_resolved = %v, want false (subject fetch failed)", env.Meta["tier_status_resolved"])
	}
	if !strings.Contains(stderr, "/u/x/mvanhorn") {
		t.Errorf("stderr should log /u/x fetch failure; got: %s", stderr)
	}
}

// TestAuthorsGet_HelpMentionsHandle ensures `--help` surfaces the
// positional `<handle>` and the `--limit` flag so agents discover
// them via `--help` introspection without reading source.
func TestAuthorsGet_HelpMentionsHandle(t *testing.T) {
	var flags rootFlags
	root := newRootCmd(&flags)
	root.SetArgs([]string{"authors", "get", "--help"})
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	if err := root.Execute(); err != nil {
		t.Fatalf("authors get --help: %v", err)
	}
	help := buf.String()
	if !strings.Contains(help, "<handle>") {
		t.Errorf("--help should mention <handle> positional; got:\n%s", help)
	}
	if !strings.Contains(help, "--limit") {
		t.Errorf("--help should mention --limit flag; got:\n%s", help)
	}
}
