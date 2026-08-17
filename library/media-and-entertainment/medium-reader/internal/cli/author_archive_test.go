// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/source"
)

// TestLooksLikeUserID pins the handle-vs-id heuristic, including the
// case-insensitive hex acceptance that keeps an uppercase-copied id from
// falling through to (and failing) HTTP handle resolution.
func TestLooksLikeUserID(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"canonical lowercase 12-hex", "bcab753a4d4e", true},
		{"uppercase hex", "BCAB753A4D4E", true},
		{"mixed-case hex", "BcAb753a4D4e", true},
		{"leading @ is a handle", "@quincylarson", false},
		{"plain username", "quincylarson", false},
		{"too short", "abc123", false},
		{"too long (>16)", "0123456789abcdef0", false},
		{"non-hex letter g", "bcab753a4g4e", false},
		{"empty", "", false},
		{"surrounding whitespace trimmed", "  bcab753a4d4e  ", true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := looksLikeUserID(tc.in); got != tc.want {
				t.Errorf("looksLikeUserID(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestBuildArchiveRecord_CanonicalStoreKeys is the regression guard for the
// writer↔reader field contract. author-archive must store the keys digest and
// corpus read (author, published_at), not the orphan author_name/
// first_published_at that previously shipped and silently emptied digest.
func TestBuildArchiveRecord_CanonicalStoreKeys(t *testing.T) {
	t.Parallel()
	pub := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	s := source.PostSummary{
		ID:          "abc123",
		Title:       "Hello World",
		URL:         "https://medium.com/p/abc123",
		Author:      "Quincy Larson",
		AuthorID:    "17756313f41a",
		Username:    "quincylarson",
		PublishedAt: pub,
	}
	art := &source.Article{Markdown: "# Body", Subtitle: "sub", WordCount: 42}

	obj := buildArchiveRecord(s, art, "17756313f41a", "quincylarson")

	// The two keys digest/corpus read must be present and populated.
	if got, ok := obj["author"].(string); !ok || got != "Quincy Larson" {
		t.Errorf("author key = %v (ok=%v), want %q", obj["author"], ok, "Quincy Larson")
	}
	pubStr, ok := obj["published_at"].(string)
	if !ok || pubStr == "" {
		t.Fatalf("published_at key missing/blank: %v", obj["published_at"])
	}
	// digest parses published_at back via the same path; it must round-trip.
	if when := parsePublishedAt(pubStr); when.IsZero() {
		t.Errorf("published_at %q does not parse back through parsePublishedAt", pubStr)
	}
	// The orphan keys from the bug must NOT reappear.
	if _, bad := obj["author_name"]; bad {
		t.Error("buildArchiveRecord wrote orphan key author_name")
	}
	if _, bad := obj["first_published_at"]; bad {
		t.Error("buildArchiveRecord wrote orphan key first_published_at")
	}
	// archived_author is the hex id; archived_handle is the stable handle key
	// author-compare also matches on so a @handle archive can be compared by @handle.
	if got := obj["archived_author"]; got != "17756313f41a" {
		t.Errorf("archived_author = %v, want %q", got, "17756313f41a")
	}
	if got := obj["archived_handle"]; got != "quincylarson" {
		t.Errorf("archived_handle = %v, want %q", got, "quincylarson")
	}
}

// TestBuildArchiveRecord_NilArticleArchivesMetadata confirms a missing body
// still yields a usable metadata record (no panic, canonical keys intact).
func TestBuildArchiveRecord_NilArticleArchivesMetadata(t *testing.T) {
	t.Parallel()
	obj := buildArchiveRecord(source.PostSummary{ID: "x", Author: "A"}, nil, "A", "")
	if obj["author"] != "A" {
		t.Errorf("author = %v, want A", obj["author"])
	}
	if _, ok := obj["markdown"]; ok {
		t.Error("nil article must not populate markdown")
	}
}

// TestBuildArchiveRecord_EngagementKeys is the F4 writer↔reader guard: author-
// archive must store the engagement keys author-compare reads (claps, voters,
// responses, reading_time, tags), and prefer Medium's authoritative word_count
// from the GraphQL summary over the page-derived count.
func TestBuildArchiveRecord_EngagementKeys(t *testing.T) {
	t.Parallel()
	s := source.PostSummary{
		ID:          "abc123",
		Author:      "Quincy Larson",
		Claps:       4919,
		Voters:      631,
		Responses:   18,
		ReadingTime: 17.836,
		WordCount:   4382, // Medium-authoritative
		Tags:        []string{"writing", "tech"},
	}
	// Page-derived word count differs; the authoritative summary value must win.
	art := &source.Article{Markdown: "# Body", WordCount: 4000}

	obj := buildArchiveRecord(s, art, "17756313f41a", "quincylarson")

	if obj["claps"] != 4919 {
		t.Errorf("claps = %v, want 4919", obj["claps"])
	}
	if obj["voters"] != 631 {
		t.Errorf("voters = %v, want 631", obj["voters"])
	}
	if obj["responses"] != 18 {
		t.Errorf("responses = %v, want 18", obj["responses"])
	}
	if obj["reading_time"] != 17.836 {
		t.Errorf("reading_time = %v, want 17.836", obj["reading_time"])
	}
	if obj["word_count"] != 4382 {
		t.Errorf("word_count = %v, want 4382 (authoritative summary value preferred over page-derived 4000)", obj["word_count"])
	}
	tags, ok := obj["tags"].([]string)
	if !ok || len(tags) != 2 || tags[0] != "writing" || tags[1] != "tech" {
		t.Errorf("tags = %v, want [writing tech]", obj["tags"])
	}
}

// TestBuildArchiveRecord_WordCountFallsBackToPage asserts that when the GraphQL
// summary carries no authoritative word count (0) — e.g. the minimal-query
// fallback ran — the page-derived count is still used.
func TestBuildArchiveRecord_WordCountFallsBackToPage(t *testing.T) {
	t.Parallel()
	s := source.PostSummary{ID: "x", WordCount: 0}
	art := &source.Article{WordCount: 1234}
	obj := buildArchiveRecord(s, art, "x", "")
	if obj["word_count"] != 1234 {
		t.Errorf("word_count = %v, want 1234 (page fallback when summary word count is 0)", obj["word_count"])
	}
}

// TestBuildArchiveRecord_NoWordCountAnywhereOmitsKey asserts no zero word_count is
// stored when neither source has one (the reader treats a missing key as 0).
func TestBuildArchiveRecord_NoWordCountAnywhereOmitsKey(t *testing.T) {
	t.Parallel()
	obj := buildArchiveRecord(source.PostSummary{ID: "x", WordCount: 0}, nil, "x", "")
	if _, ok := obj["word_count"]; ok {
		t.Errorf("word_count present (%v); want omitted when no count is available", obj["word_count"])
	}
}

// TestRecordArchiveSyncState_SetsMarkerAndClearsHint is the F1 regression guard.
// author-archive writes article rows to the resources table via db.Upsert, which
// never touches the SEPARATE sync_state table that doctor and the analytics/
// digest sync hints read. The result was a fully populated store still reporting
// "local store has not been synced yet". recordArchiveSyncState writes the marker
// so the false hint clears — this test pins both the marker write and the
// downstream consumer behavior (and proves SaveSyncState's RFC3339 timestamp
// round-trips back through the hint readers).
func TestRecordArchiveSyncState_SetsMarkerAndClearsHint(t *testing.T) {
	db := newSyncHintTestStore(t)
	cmd, _ := newSyncHintTestCmd()

	// Precondition: an empty store fires the "not synced yet" hint — the bug's
	// user-visible symptom on a store that author-archive has not yet marked.
	if !hintIfUnsynced(cmd, db, "articles") {
		t.Fatalf("precondition: empty store should emit the not-synced hint")
	}

	if err := recordArchiveSyncState(db, 3); err != nil {
		t.Fatalf("recordArchiveSyncState: %v", err)
	}

	// The marker row must carry last_synced_at (so staleness checks work) and the
	// archived count.
	_, lastSynced, count, err := db.GetSyncState("articles")
	if err != nil {
		t.Fatalf("GetSyncState: %v", err)
	}
	if lastSynced.IsZero() {
		t.Errorf("last_synced_at is zero after marker write; staleness checks would break")
	}
	if count != 3 {
		t.Errorf("total_count = %d, want 3", count)
	}

	// The false hint must now be gone for both the resource-scoped reader
	// (analytics --type articles) and the all-resource reader (doctor).
	cmd, stderr := newSyncHintTestCmd()
	if hintIfUnsynced(cmd, db, "articles") {
		t.Errorf("not-synced hint still fires after marker write (resource-scoped)")
	}
	if hintIfUnsynced(cmd, db, "") {
		t.Errorf("not-synced hint still fires after marker write (all-resource)")
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want no hint after marker write", stderr.String())
	}
}

// TestRecordArchiveSyncState_ReportsCumulativeStoreCount pins that the marker's
// total_count — which doctor displays as the cached "rows" — reflects the whole
// store, not just the last archive run's delta. Two sequential archives must
// report the cumulative count, or doctor under-reports after the second author.
func TestRecordArchiveSyncState_ReportsCumulativeStoreCount(t *testing.T) {
	db := newSyncHintTestStore(t)
	// First archive run: 3 rows land in the articles table, then mark synced.
	for _, id := range []string{"a1", "a2", "a3"} {
		if err := db.Upsert("articles", id, []byte(`{"id":"`+id+`"}`)); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	if err := recordArchiveSyncState(db, 3); err != nil {
		t.Fatalf("recordArchiveSyncState run 1: %v", err)
	}
	// Second archive run adds 2 more rows (a different author).
	for _, id := range []string{"b1", "b2"} {
		if err := db.Upsert("articles", id, []byte(`{"id":"`+id+`"}`)); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	if err := recordArchiveSyncState(db, 2); err != nil {
		t.Fatalf("recordArchiveSyncState run 2: %v", err)
	}

	_, _, count, err := db.GetSyncState("articles")
	if err != nil {
		t.Fatalf("GetSyncState: %v", err)
	}
	if count != 5 {
		t.Errorf("sync_state total_count = %d, want 5 (cumulative store count, not last-run delta of 2)", count)
	}
}

// TestRecordArchiveSyncState_NoArticlesLeavesNoMarker pins the archived > 0
// guard: a crawl that fetched nothing must not stamp a misleading last_synced_at
// that would make an empty store look freshly synced.
func TestRecordArchiveSyncState_NoArticlesLeavesNoMarker(t *testing.T) {
	db := newSyncHintTestStore(t)

	if err := recordArchiveSyncState(db, 0); err != nil {
		t.Fatalf("recordArchiveSyncState(0): %v", err)
	}

	cmd, _ := newSyncHintTestCmd()
	if !hintIfUnsynced(cmd, db, "articles") {
		t.Errorf("guard failed: a zero-article archive wrote a sync marker")
	}
}

// TestRateLimiter_DisabledByDefault documents that the wired --rate-limit flag
// is a true no-op at its default, so archiving is not silently throttled.
func TestRateLimiter_DisabledByDefault(t *testing.T) {
	t.Parallel()
	if l := cliutil.NewAdaptiveLimiter(0); l != nil {
		t.Errorf("NewAdaptiveLimiter(0) = %v, want nil (rate-limiting disabled)", l)
	}
	if l := cliutil.NewAdaptiveLimiter(2); l == nil {
		t.Error("NewAdaptiveLimiter(2) = nil, want a live limiter")
	}
}
