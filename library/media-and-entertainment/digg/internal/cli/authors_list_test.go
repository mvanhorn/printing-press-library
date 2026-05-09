// Tests for `authors list` command. The live path is exercised in the
// parser/store layer; here we validate the cobra wiring + filter/sort
// SQL, the JSON envelope shape, and the local-cache fallback.
//
// Tests run in --data-source local mode so we don't need a real network.
// HOME is redirected to a tempdir so each test gets an isolated SQLite
// database that doesn't collide with the user's real cache.

package cli

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/diggparse"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/diggstore"
)

// authorsListEnvelope mirrors the JSON shape printed by `authors list`.
type authorsListEnvelope struct {
	Meta    map[string]any `json:"meta"`
	Results []struct {
		Username       string  `json:"username"`
		DisplayName    string  `json:"displayName,omitempty"`
		Rank           int     `json:"rank"`
		PreviousRank   *int    `json:"previousRank"`
		RankChange     *int    `json:"rankChange"`
		Category       string  `json:"category,omitempty"`
		CategoryRank   int     `json:"categoryRank,omitempty"`
		FollowersCount int     `json:"followersCount,omitempty"`
		Bio            string  `json:"bio,omitempty"`
		GithubURL      string  `json:"githubUrl,omitempty"`
		XURL           string  `json:"xUrl,omitempty"`
		Score          float64 `json:"score,omitempty"`
	} `json:"results"`
}

// withTempHome redirects HOME so the CLI's defaultDBPath resolves to a
// throwaway tempdir. Returns a function the test can call to seed the
// store with parsed fixture authors.
func withTempHome(t *testing.T) func(authors []diggparse.Roster1000Author) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir) // safety on Windows builds; harmless on macOS

	// Pre-create the path so EnsureSchema can run.
	dbDir := filepath.Join(dir, ".local", "share", "digg-pp-cli")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}

	return func(authors []diggparse.Roster1000Author) {
		dbPath := filepath.Join(dbDir, "data.db")
		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		if err := diggstore.EnsureSchema(db); err != nil {
			t.Fatal(err)
		}
		if _, err := diggstore.UpsertRoster1000(db, authors, time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
	}
}

// loadFixtureAuthors parses the testdata fixture (re-using parser tests' path).
func loadFixtureAuthors(t *testing.T) []diggparse.Roster1000Author {
	t.Helper()
	candidates := []string{
		filepath.Join("..", "..", "testdata", "ai-1000-fixture.html"),
		filepath.Join("testdata", "ai-1000-fixture.html"),
	}
	var data []byte
	var err error
	for _, p := range candidates {
		if data, err = os.ReadFile(p); err == nil {
			break
		}
	}
	if data == nil {
		t.Fatalf("fixture not found: %v", err)
	}
	authors, err := diggparse.ParseRoster1000(data)
	if err != nil && len(authors) == 0 {
		t.Fatalf("ParseRoster1000: %v", err)
	}
	return authors
}

// runAuthorsList builds the cobra command tree, wires test stdout, and
// runs `authors list` with the given args (in addition to --json).
func runAuthorsList(t *testing.T, extraArgs ...string) (string, string, error) {
	t.Helper()
	var flags rootFlags
	root := newRootCmd(&flags)
	args := append([]string{"authors", "list", "--json", "--data-source", "local"}, extraArgs...)
	root.SetArgs(args)

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

func TestAuthorsList_LocalReadsCachedRoster(t *testing.T) {
	seed := withTempHome(t)
	seed(loadFixtureAuthors(t))

	out, _, err := runAuthorsList(t, "--limit", "5")
	if err != nil {
		t.Fatalf("authors list: %v", err)
	}
	var env authorsListEnvelope
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("invalid JSON envelope: %v\n%s", err, out)
	}
	if env.Meta["source"] != "local" {
		t.Errorf("meta.source = %v, want local", env.Meta["source"])
	}
	if len(env.Results) != 5 {
		t.Errorf("results = %d, want 5", len(env.Results))
	}
	// Default --by rank: ranks must be ascending.
	for i := 1; i < len(env.Results); i++ {
		if env.Results[i].Rank < env.Results[i-1].Rank {
			t.Errorf("results not sorted by rank ASC: %d < %d at index %d",
				env.Results[i].Rank, env.Results[i-1].Rank, i)
		}
	}
	// Top result should be sama (rank 1) given fixture coverage.
	if env.Results[0].Username != "sama" {
		t.Errorf("top result = %q, want sama", env.Results[0].Username)
	}
	if env.Results[0].XURL != "https://x.com/sama" {
		t.Errorf("xUrl mint = %q", env.Results[0].XURL)
	}
}

func TestAuthorsList_OnlyNewFiltersToNullPreviousRank(t *testing.T) {
	seed := withTempHome(t)
	seed(loadFixtureAuthors(t))

	out, _, err := runAuthorsList(t, "--only-new")
	if err != nil {
		t.Fatalf("authors list --only-new: %v", err)
	}
	var env authorsListEnvelope
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(env.Results) == 0 {
		t.Fatal("--only-new returned 0 results; fixture should contain at least some null-previousRank authors")
	}
	for _, r := range env.Results {
		if r.PreviousRank != nil {
			t.Errorf("@%s has previousRank=%d but --only-new should filter to null only", r.Username, *r.PreviousRank)
		}
	}
}

func TestAuthorsList_CategoryFilterAndSort(t *testing.T) {
	seed := withTempHome(t)
	seed(loadFixtureAuthors(t))

	// Don't pass --by explicitly; --category alone should auto-sort by categoryRank.
	out, _, err := runAuthorsList(t, "--category", "AI Safety")
	if err != nil {
		t.Fatalf("authors list --category: %v", err)
	}
	var env authorsListEnvelope
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(env.Results) == 0 {
		t.Fatal("AI Safety category should have at least one result in fixture (logangraham)")
	}
	for _, r := range env.Results {
		if r.Category != "AI Safety" {
			t.Errorf("@%s category = %q, want AI Safety", r.Username, r.Category)
		}
	}
	// Sorted by categoryRank ascending.
	for i := 1; i < len(env.Results); i++ {
		if env.Results[i].CategoryRank < env.Results[i-1].CategoryRank {
			t.Errorf("not sorted by categoryRank ASC at index %d", i)
		}
	}
	// logangraham must be in the result set.
	saw := false
	for _, r := range env.Results {
		if r.Username == "logangraham" {
			saw = true
			break
		}
	}
	if !saw {
		t.Error("AI Safety filter should include logangraham (rank 991, AI Safety in fixture)")
	}
}

func TestAuthorsList_ByRankChangeBiggestMoversFirst(t *testing.T) {
	seed := withTempHome(t)
	// Seed a small synthetic roster so we control the sort outcome.
	authors := []diggparse.Roster1000Author{
		{Rank: 1, Username: "a", DisplayName: "A", PreviousRank: intp(11), RankChange: intp(10)},
		{Rank: 2, Username: "b", DisplayName: "B", PreviousRank: intp(7), RankChange: intp(5)},
		{Rank: 3, Username: "c", DisplayName: "C", PreviousRank: intp(1), RankChange: intp(-2)},
		{Rank: 4, Username: "d", DisplayName: "D", PreviousRank: nil, RankChange: nil},
	}
	seed(authors)
	out, _, err := runAuthorsList(t, "--by", "rankChange", "--limit", "10")
	if err != nil {
		t.Fatalf("authors list --by rankChange: %v", err)
	}
	var env authorsListEnvelope
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(env.Results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(env.Results))
	}
	// ABS ordering: 10, 5, -2, then nulls.
	wantOrder := []string{"a", "b", "c", "d"}
	for i, w := range wantOrder {
		if env.Results[i].Username != w {
			t.Errorf("rank-change order: index %d = %q, want %q", i, env.Results[i].Username, w)
		}
	}
}

func intp(v int) *int { return &v }

func TestAuthorsList_LocalEmptyHasFriendlyError(t *testing.T) {
	withTempHome(t)
	// No seed: empty store + --data-source local should return a hint, not crash.
	out, stderr, err := runAuthorsList(t)
	// The command returns an error envelope or a hint message; we accept either.
	if err == nil && !strings.Contains(out, "no authors") && !strings.Contains(stderr, "no authors") {
		t.Errorf("empty local store should produce empty-hint; got out=%q stderr=%q", out, stderr)
	}
}
