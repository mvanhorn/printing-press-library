// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written regression test for issue #1848: `search --data-source local`
// (and the auto-mode fallback) returned empty results because the untyped
// `case "":` branch in search.go never assigned `results`. The untyped search
// must return the same FTS hits a `--type zzzz` search returns.

package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/internal/store"
)

// seedSearchStore opens a temp-dir file-backed store and inserts FTS-indexed
// fixtures via the store's own upsert path. A file-backed store is required:
// the store opens with SetMaxOpenConns(2) + WAL, and SQLite :memory: databases
// are per-connection, so an in-memory DB could silently use a different
// connection than the migration. An optional variadic fixture list replaces
// the default Inception fixtures.
func seedSearchStore(t *testing.T, query string, extraFixtures ...struct {
	id   string
	data string
}) string {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "search_test.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("opening temp store: %v", err)
	}
	defer s.Close()

	fixtures := []struct {
		id   string
		data string
	}{
		{"973484", `{"id":973484,"title":"Inception: Music from the Motion Picture","type":"movie"}`},
		{"27205", `{"id":27205,"title":"Inception","type":"movie"}`},
		{"1234", `{"id":1234,"title":"A Very Different Film","type":"movie"}`},
	}
	fixtures = append(fixtures, extraFixtures...)
	for _, f := range fixtures {
		if err := s.Upsert("movies", f.id, json.RawMessage(f.data)); err != nil {
			t.Fatalf("upserting fixture %s: %v", f.id, err)
		}
	}

	// Prove the fixture store's FTS index is actually populated — a silently
	// empty index would let a parity-only test false-pass. Expect at least one
	// hit for the proving query.
	got, err := s.Search(query, 50)
	if err != nil {
		t.Fatalf("seeding search: %v", err)
	}
	if len(got) == 0 {
		t.Fatalf("seed store FTS index should hold hits for %q, got 0", query)
	}
	return dbPath
}

// runSearch executes `search <query>` with the given args against a temp-dir
// store and returns the parsed JSON envelope.
func runSearch(t *testing.T, dbPath, query string, extraArgs ...string) map[string]any {
	t.Helper()
	t.Setenv("MOVIE_GOAT_BASE_URL", "http://127.0.0.1:1")

	root := RootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	var errBuf bytes.Buffer
	root.SetErr(&errBuf)
	args := append([]string{"--db", dbPath, "--no-cache", "--json"}, extraArgs...)
	args = append(args, "search", query)
	root.SetArgs(args)

	if err := root.Execute(); err != nil {
		t.Fatalf("search returned error: %v\noutput:\n%s\nstderr:\n%s", err, out.String(), errBuf.String())
	}

	var envelope map[string]any
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("parsing search output %q: %v\nstderr:\n%s", out.String(), err, errBuf.String())
	}
	return envelope
}

// titlesFromEnvelope extracts the "title" field of each result in the
// envelope's "results" array.
func titlesFromEnvelope(t *testing.T, envelope map[string]any) []string {
	t.Helper()
	rawResults, ok := envelope["results"].([]any)
	if !ok {
		t.Fatalf("envelope has no results array: %v", envelope)
	}
	var titles []string
	for _, r := range rawResults {
		obj, ok := r.(map[string]any)
		if !ok {
			t.Fatalf("result is not an object: %v", r)
		}
		title, _ := obj["title"].(string)
		titles = append(titles, title)
	}
	return titles
}

// TestSearchUntypedLocalMatchesTypedControl asserts the fix for issue #1848:
// an untyped `--data-source local` search returns the same non-empty, ordered
// hits as the `--type zzzz` control (which already worked because it falls to
// the default arm).
func TestSearchUntypedLocalMatchesTypedControl(t *testing.T) {
	dbPath := seedSearchStore(t, "inception")

	typed := runSearch(t, dbPath, "inception", "--type", "zzzz", "--data-source", "local")
	typedTitles := titlesFromEnvelope(t, typed)
	if len(typedTitles) != 2 {
		t.Fatalf("typed control should return 2 hits, got %d: %v", len(typedTitles), typedTitles)
	}

	untyped := runSearch(t, dbPath, "inception", "--data-source", "local")
	untypedTitles := titlesFromEnvelope(t, untyped)
	if len(untypedTitles) != 2 {
		t.Fatalf("untyped local search returned %d hits, want 2: %v", len(untypedTitles), untypedTitles)
	}
	if !slices.Equal(untypedTitles, typedTitles) {
		t.Fatalf("untyped local search results differ from typed control: untyped=%v typed=%v", untypedTitles, typedTitles)
	}
}

// TestSearchAutoFallbackReturnsLocalHits asserts the auto-mode fallback returns
// local hits with the api_unreachable provenance when the API is unreachable.
func TestSearchAutoFallbackReturnsLocalHits(t *testing.T) {
	dbPath := seedSearchStore(t, "inception")

	envelope := runSearch(t, dbPath, "inception")
	if reason, _ := envelope["meta"].(map[string]any)["reason"].(string); reason != "api_unreachable" {
		t.Fatalf("auto fallback reason: got %q, want api_unreachable (envelope %v)", reason, envelope["meta"])
	}
	titles := titlesFromEnvelope(t, envelope)
	if len(titles) != 2 {
		t.Fatalf("auto fallback returned %d hits, want 2: %v", len(titles), titles)
	}
}

// TestSearchNoMatchesReturnsEmpty verifies a query with no matches still
// returns an empty (not erroring) results array.
func TestSearchNoMatchesReturnsEmpty(t *testing.T) {
	dbPath := seedSearchStore(t, "inception")

	envelope := runSearch(t, dbPath, "nonexistenttermxyz", "--data-source", "local")
	titles := titlesFromEnvelope(t, envelope)
	if len(titles) != 0 {
		t.Fatalf("no-match query should return zero hits, got %v", titles)
	}
}

// TestSearchFTSSyntaxQueryReturnsLiteralHits asserts a query containing FTS5
// syntax (the ":" column operator) searches literally through the untyped
// local path instead of raising a MATCH parse error.
func TestSearchFTSSyntaxQueryReturnsLiteralHits(t *testing.T) {
	dbPath := seedSearchStore(t, "Space: 1999",
		struct {
			id   string
			data string
		}{"9999", `{"id":9999,"title":"Space: 1999","type":"movie"}`},
	)

	envelope := runSearch(t, dbPath, "Space: 1999", "--data-source", "local")
	titles := titlesFromEnvelope(t, envelope)
	if len(titles) != 1 || titles[0] != "Space: 1999" {
		t.Fatalf("FTS-syntax query should return the literal hit, got %v", titles)
	}
}
