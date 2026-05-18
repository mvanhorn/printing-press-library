package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

func mustOpen(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestOpenMigratesIdempotently(t *testing.T) {
	st := mustOpen(t)
	if st.Path == "" {
		t.Fatal("Path should be set")
	}
	// Re-open the same path; migrations should be no-ops.
	st2, err := Open(st.Path)
	if err != nil {
		t.Fatalf("re-open: %v", err)
	}
	_ = st2.Close()
}

func TestUpsertProject(t *testing.T) {
	st := mustOpen(t)
	ctx := context.Background()
	now := time.Now().UTC()
	p := Project{
		ID:           "proj_1",
		Name:         "Acme",
		BrandName:    "Acme Inc",
		RawJSON:      `{"id":"proj_1"}`,
		LastSyncedAt: now,
	}
	if err := st.UpsertProject(ctx, p); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	// Upsert with different name should overwrite, not duplicate.
	p.Name = "Acme v2"
	if err := st.UpsertProject(ctx, p); err != nil {
		t.Fatalf("Upsert v2: %v", err)
	}
	got, err := st.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 project, got %d", len(got))
	}
	if got[0].Name != "Acme v2" {
		t.Errorf("name not updated: got %q", got[0].Name)
	}
}

func TestVisibilitySnapshotPair(t *testing.T) {
	st := mustOpen(t)
	ctx := context.Background()
	// Insert two snapshots for the same metric on different dates.
	for _, date := range []string{"2026-05-01", "2026-05-08"} {
		err := st.UpsertVisibilitySnapshot(ctx, VisibilitySnapshot{
			ProjectID: "p1", BrandName: "Acme", SnapshotDate: date, MetricType: "overview",
			Payload: json.RawMessage(`{"score":1}`),
		})
		if err != nil {
			t.Fatalf("upsert %s: %v", date, err)
		}
	}
	newer, older, err := st.LatestSnapshotPair(ctx, "p1", "Acme", "overview")
	if err != nil {
		t.Fatalf("pair: %v", err)
	}
	if newer == nil || older == nil {
		t.Fatalf("expected both snapshots; got newer=%v older=%v", newer, older)
	}
	// modernc.org/sqlite returns DATE columns with an appended "T00:00:00Z"
	// suffix; we match by prefix so the test stays decoupled from that detail.
	if !startsWith(newer.SnapshotDate, "2026-05-08") || !startsWith(older.SnapshotDate, "2026-05-01") {
		t.Errorf("ordering wrong: newer=%s older=%s", newer.SnapshotDate, older.SnapshotDate)
	}
	// Re-upsert the newer date with a different payload — should overwrite, not duplicate.
	if err := st.UpsertVisibilitySnapshot(ctx, VisibilitySnapshot{
		ProjectID: "p1", BrandName: "Acme", SnapshotDate: "2026-05-08", MetricType: "overview",
		Payload: json.RawMessage(`{"score":2}`),
	}); err != nil {
		t.Fatalf("upsert overwrite: %v", err)
	}
	newer, _, _ = st.LatestSnapshotPair(ctx, "p1", "Acme", "overview")
	if string(newer.Payload) != `{"score":2}` {
		t.Errorf("payload not overwritten: %s", newer.Payload)
	}
}

func TestFTSSearchRoundTrip(t *testing.T) {
	st := mustOpen(t)
	ctx := context.Background()
	docs := []FTSDoc{
		{Kind: "prompt", ID: "pr1", ProjectID: "p1", Title: "AI search optimization for ecommerce", Body: "How to rank in AI answer engines"},
		{Kind: "doc", ID: "d1", ProjectID: "p1", Title: "Brownie recipe", Body: "Soft fudgy brownies"},
		{Kind: "citation", ID: "c1", ProjectID: "p1", Title: "openai.com/blog/answer-engines", Body: "AI answer engines compared"},
	}
	for _, d := range docs {
		if err := st.UpsertFTS(ctx, d); err != nil {
			t.Fatalf("UpsertFTS: %v", err)
		}
	}
	// Term query.
	hits, err := st.Search(ctx, "answer", nil, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 2 {
		t.Errorf("expected 2 hits for 'answer', got %d (%+v)", len(hits), hits)
	}
	// Kind filter.
	hits, err = st.Search(ctx, "answer", []string{"prompt"}, 10)
	if err != nil {
		t.Fatalf("Search filtered: %v", err)
	}
	if len(hits) != 1 || hits[0].Kind != "prompt" {
		t.Errorf("expected exactly one prompt hit, got %+v", hits)
	}
	// Re-upsert same id should not duplicate.
	if err := st.UpsertFTS(ctx, docs[0]); err != nil {
		t.Fatalf("UpsertFTS re-insert: %v", err)
	}
	hits, _ = st.Search(ctx, "ecommerce", nil, 10)
	if len(hits) != 1 {
		t.Errorf("FTS de-dup failed: got %d", len(hits))
	}
}

func TestBumpCursorIdempotent(t *testing.T) {
	st := mustOpen(t)
	ctx := context.Background()
	if err := st.BumpCursor(ctx, "prompts", "p1", 10); err != nil {
		t.Fatal(err)
	}
	if err := st.BumpCursor(ctx, "prompts", "p1", 15); err != nil {
		t.Fatal(err)
	}
	cursors, err := st.Cursors(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(cursors) != 1 {
		t.Fatalf("want 1 cursor, got %d", len(cursors))
	}
	if cursors[0].RowCount != 15 {
		t.Errorf("row_count not updated: got %d", cursors[0].RowCount)
	}
}

func startsWith(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}

// TestKnowledgeImpactExactURLMatch pins the post-greptile-10 behavior of
// KnowledgeImpact: citation joins are computed by parsing each snapshot's
// raw_json and exact-matching URL strings, not by SQL substring search over
// the serialized blob. Two failure modes from the prior implementation are
// covered:
//
//	(1) a short library URL prefix-matching a longer citation URL
//	(2) a library URL appearing inside a non-URL field (image src) of a
//	    citation that is otherwise unrelated.
//
// Both must now contribute zero citations to the affected library.
func TestKnowledgeImpactExactURLMatch(t *testing.T) {
	st := mustOpen(t)
	ctx := context.Background()
	now := time.Now().UTC()

	if err := st.UpsertProject(ctx, Project{
		ID: "p1", Name: "Acme", RawJSON: `{}`, LastSyncedAt: now,
	}); err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}

	// lib_a: a single short URL that previously over-matched.
	// lib_b: an exact-URL document that should produce one true-positive cite.
	for _, lib := range []KnowledgeLibrary{
		{ID: "lib_a", ProjectID: "p1", Name: "Short URL library", RawJSON: json.RawMessage(`{}`), LastSeenAt: now},
		{ID: "lib_b", ProjectID: "p1", Name: "Exact-match library", RawJSON: json.RawMessage(`{}`), LastSeenAt: now},
	} {
		if err := st.UpsertKnowledgeLibrary(ctx, lib); err != nil {
			t.Fatalf("UpsertKnowledgeLibrary %s: %v", lib.ID, err)
		}
	}
	for _, d := range []KnowledgeLibraryDocument{
		{ID: "doc_a1", LibraryID: "lib_a", Title: "Blog index", URL: "https://example.com/blog", RawJSON: json.RawMessage(`{}`), LastSeenAt: now},
		{ID: "doc_b1", LibraryID: "lib_b", Title: "Post A", URL: "https://example.com/blog/post-a", RawJSON: json.RawMessage(`{}`), LastSeenAt: now},
	} {
		if err := st.UpsertKnowledgeLibraryDocument(ctx, d); err != nil {
			t.Fatalf("UpsertKnowledgeLibraryDocument %s: %v", d.ID, err)
		}
	}

	// Snapshot 1: citation is the longer URL (lib_b true positive; under the
	// old SQL it would also have matched lib_a via prefix).
	// Snapshot 2: citation is unrelated (other.com/post) but its image_src
	// contains a longer URL that begins with lib_a's URL — under the old SQL
	// the doc URL appeared as a substring of the og.png path; under exact
	// match the extracted URL is the og.png URL itself, which does not equal
	// the doc URL.
	for _, cs := range []CitationSnapshot{
		{
			ProjectID: "p1", SnapshotDate: "2026-05-13", Domain: "example.com",
			CitationCount: 10, URLCount: 1,
			RawJSON: json.RawMessage(`{"citations":[{"url":"https://example.com/blog/post-a"}]}`),
		},
		{
			ProjectID: "p1", SnapshotDate: "2026-05-12", Domain: "other.com",
			CitationCount: 5, URLCount: 1,
			RawJSON: json.RawMessage(`{"citations":[{"url":"https://other.com/post","image_src":"https://example.com/blog/og.png"}]}`),
		},
	} {
		if err := st.UpsertCitationSnapshot(ctx, cs); err != nil {
			t.Fatalf("UpsertCitationSnapshot %s: %v", cs.SnapshotDate, err)
		}
	}

	rows, err := st.KnowledgeImpact(ctx, "p1")
	if err != nil {
		t.Fatalf("KnowledgeImpact: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 libraries in result, got %d (%+v)", len(rows), rows)
	}

	byID := map[string]KnowledgeImpactRow{}
	for _, r := range rows {
		byID[r.LibraryID] = r
	}

	libA := byID["lib_a"]
	if libA.LibraryID == "" {
		t.Fatal("lib_a missing from result")
	}
	if libA.DocCount != 1 {
		t.Errorf("lib_a DocCount: want 1, got %d", libA.DocCount)
	}
	if libA.CitedDocCount != 0 {
		t.Errorf("lib_a CitedDocCount: want 0 (no exact match), got %d — substring false-positive regressed", libA.CitedDocCount)
	}
	if libA.TotalCites != 0 {
		t.Errorf("lib_a TotalCites: want 0, got %d — substring false-positive regressed", libA.TotalCites)
	}

	libB := byID["lib_b"]
	if libB.LibraryID == "" {
		t.Fatal("lib_b missing from result")
	}
	if libB.DocCount != 1 {
		t.Errorf("lib_b DocCount: want 1, got %d", libB.DocCount)
	}
	if libB.CitedDocCount != 1 {
		t.Errorf("lib_b CitedDocCount: want 1 (exact match), got %d", libB.CitedDocCount)
	}
	if libB.TotalCites != 10 {
		t.Errorf("lib_b TotalCites: want 10, got %d", libB.TotalCites)
	}

	// Result must be ordered by TotalCites desc to preserve the SQL ORDER BY
	// 5 DESC contract callers expect.
	if rows[0].LibraryID != "lib_b" {
		t.Errorf("expected lib_b first (sorted by TotalCites desc), got %s with cites=%d", rows[0].LibraryID, rows[0].TotalCites)
	}
}

// TestKnowledgeImpactEmpty ensures the method returns cleanly when there are
// no libraries at all (returns nil), no documents (cited counts stay zero),
// and no snapshots (cited counts stay zero).
func TestKnowledgeImpactEmpty(t *testing.T) {
	st := mustOpen(t)
	ctx := context.Background()

	// No libraries -> nil result.
	rows, err := st.KnowledgeImpact(ctx, "p_empty")
	if err != nil {
		t.Fatalf("empty project: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("want empty result for project with no libraries, got %d rows", len(rows))
	}

	// Library with no docs, no snapshots -> one row with zero counts.
	now := time.Now().UTC()
	if err := st.UpsertProject(ctx, Project{ID: "p1", Name: "Acme", RawJSON: `{}`, LastSyncedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertKnowledgeLibrary(ctx, KnowledgeLibrary{ID: "lib_x", ProjectID: "p1", Name: "Empty", RawJSON: json.RawMessage(`{}`), LastSeenAt: now}); err != nil {
		t.Fatal(err)
	}
	rows, err = st.KnowledgeImpact(ctx, "p1")
	if err != nil {
		t.Fatalf("KnowledgeImpact: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	if rows[0].DocCount != 0 || rows[0].CitedDocCount != 0 || rows[0].TotalCites != 0 {
		t.Errorf("expected all zero counts, got %+v", rows[0])
	}
}

func TestExtractCitationURLs(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{name: "empty", raw: "", want: nil},
		{name: "null", raw: "null", want: nil},
		{name: "invalid-json", raw: "{not json", want: nil},
		{name: "single-url", raw: `{"url":"https://example.com/a"}`, want: []string{"https://example.com/a"}},
		{name: "trailing-slash-and-casing-normalized", raw: `{"url":"https://Example.COM/A/"}`, want: []string{"https://example.com/a"}},
		{name: "nested-array-of-objects", raw: `{"data":[{"citation":{"url":"https://example.com/x"}},{"citation":{"url":"https://example.com/y"}}]}`, want: []string{"https://example.com/x", "https://example.com/y"}},
		{name: "ignores-non-url-string-fields", raw: `{"title":"see https://example.com/blog in text","url":"https://other.com/y"}`, want: []string{"https://other.com/y"}},
		{name: "http-and-https", raw: `{"a":"http://x.com","b":"https://y.com"}`, want: []string{"http://x.com", "https://y.com"}},
		{name: "dedupes-identical-urls", raw: `{"a":"https://x.com","b":"https://x.com/"}`, want: []string{"https://x.com"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			urls := extractCitationURLs(json.RawMessage(tc.raw))
			got := make([]string, 0, len(urls))
			for u := range urls {
				got = append(got, u)
			}
			sort.Strings(got)
			sort.Strings(tc.want)
			if len(got) != len(tc.want) {
				t.Fatalf("len mismatch: want %d %v, got %d %v", len(tc.want), tc.want, len(got), got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("index %d: want %q, got %q", i, tc.want[i], got[i])
				}
			}
		})
	}
}

// TestSyncDiffHonorsSince pins PATCH(greptile-4): the per-resource
// since-aware queries must actually execute, not be discarded with
// `_ = since`. The previous implementation hardcoded RowsSince=0 for every
// row, making `sync diff --since X` a silent no-op.
//
// We use far-past vs far-future cutoffs so the test does not depend on
// sqlite's 1-second CURRENT_TIMESTAMP resolution or on the modernc.org
// driver's specific time.Time bind format: any reasonable implementation
// must count the row when since is the zero time, and exclude it when
// since is in the future. If `_ = since` were still in place both calls
// would return 0 and the test would fail.
func TestSyncDiffHonorsSince(t *testing.T) {
	st := mustOpen(t)
	ctx := context.Background()

	if err := st.UpsertCitationSnapshot(ctx, CitationSnapshot{
		ProjectID: "p1", SnapshotDate: "2026-05-13", Domain: "example.com",
		CitationCount: 5, URLCount: 1, RawJSON: json.RawMessage(`{}`),
	}); err != nil {
		t.Fatalf("UpsertCitationSnapshot: %v", err)
	}

	find := func(rows []SyncDiffRow) *SyncDiffRow {
		for i := range rows {
			if rows[i].Resource == "citation_snapshots" && rows[i].ProjectID == "p1" {
				return &rows[i]
			}
		}
		return nil
	}

	pastRows, err := st.SyncDiff(ctx, time.Time{})
	if err != nil {
		t.Fatalf("SyncDiff(zero): %v", err)
	}
	past := find(pastRows)
	if past == nil {
		t.Fatalf("citation_snapshots row missing for zero since: %+v", pastRows)
	}
	if past.RowsTotal != 1 {
		t.Errorf("RowsTotal (zero since): want 1, got %d", past.RowsTotal)
	}
	if past.RowsSince != 1 {
		t.Errorf("RowsSince (zero since): want 1, got %d — PATCH(greptile-4) regressed if 0", past.RowsSince)
	}

	futureRows, err := st.SyncDiff(ctx, time.Now().UTC().Add(1*time.Hour))
	if err != nil {
		t.Fatalf("SyncDiff(future): %v", err)
	}
	future := find(futureRows)
	if future == nil {
		t.Fatalf("citation_snapshots row missing for future since: %+v", futureRows)
	}
	if future.RowsTotal != 1 {
		t.Errorf("RowsTotal (future since): want 1, got %d", future.RowsTotal)
	}
	if future.RowsSince != 0 {
		t.Errorf("RowsSince (future since): want 0, got %d — since cutoff not applied", future.RowsSince)
	}
}

func TestNormalizeCitationURL(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"  ", ""},
		{"https://example.com/a", "https://example.com/a"},
		{"https://example.com/a/", "https://example.com/a"},
		{"  https://Example.COM/A  ", "https://example.com/a"},
		{"https://example.com/a///", "https://example.com/a"},
	}
	for _, tc := range cases {
		got := normalizeCitationURL(tc.in)
		if got != tc.want {
			t.Errorf("normalizeCitationURL(%q): want %q, got %q", tc.in, tc.want, got)
		}
	}
}
