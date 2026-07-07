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

	obj := buildArchiveRecord(s, art, "17756313f41a")

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
	// archived_author is what author-compare filters on.
	if got := obj["archived_author"]; got != "17756313f41a" {
		t.Errorf("archived_author = %v, want %q", got, "17756313f41a")
	}
}

// TestBuildArchiveRecord_NilArticleArchivesMetadata confirms a missing body
// still yields a usable metadata record (no panic, canonical keys intact).
func TestBuildArchiveRecord_NilArticleArchivesMetadata(t *testing.T) {
	t.Parallel()
	obj := buildArchiveRecord(source.PostSummary{ID: "x", Author: "A"}, nil, "A")
	if obj["author"] != "A" {
		t.Errorf("author = %v, want A", obj["author"])
	}
	if _, ok := obj["markdown"]; ok {
		t.Error("nil article must not populate markdown")
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
