// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package rss

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/source"
)

// fixturePath resolves a fixture relative to the repo's testdata directory.
// The rss package lives at internal/source/rss, so the testdata root is three
// directories up.
func fixturePath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("..", "..", "..", "testdata", "fixtures", name)
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(fixturePath(t, name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return b
}

// TestParseTagFeed asserts the tag feed parses into exactly 10 items, each
// carrying a title, a link, and a parsed publish date. This is the spec's
// hermetic rss-tag-ux.xml contract.
func TestParseTagFeed(t *testing.T) {
	data := readFixture(t, "rss-tag-ux.xml")
	items, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(items) != 10 {
		t.Fatalf("got %d items, want 10", len(items))
	}
	for i, it := range items {
		if strings.TrimSpace(it.Title) == "" {
			t.Errorf("item %d: empty title", i)
		}
		if strings.TrimSpace(it.URL) == "" {
			t.Errorf("item %d: empty link", i)
		}
		if it.PublishedAt.IsZero() {
			t.Errorf("item %d (%q): zero PublishedAt", i, it.Title)
		}
	}

	// Spot-check the first item against the known fixture content.
	first := items[0]
	if first.Title != "Why Users Abandon Apps in 30 Seconds" {
		t.Errorf("first title = %q", first.Title)
	}
	if first.Author != "Hope so saloni" {
		t.Errorf("first author = %q (want dc:creator)", first.Author)
	}
	if first.ID != "e8afe9858d59" {
		t.Errorf("first id = %q (want extracted from /p/<id>)", first.ID)
	}
	wantTags := map[string]bool{"ui-design": true, "ux-design": true, "ux": true}
	gotTags := map[string]bool{}
	for _, tag := range first.Tags {
		gotTags[tag] = true
	}
	for tag := range wantTags {
		if !gotTags[tag] {
			t.Errorf("first item missing category %q; got %v", tag, first.Tags)
		}
	}
}

// TestParsePublicationFeedContentEncoded asserts the publication feed's item
// carries the full content:encoded HTML body — the distinguishing feature of
// publication/tag feeds for free posts.
func TestParsePublicationFeedContentEncoded(t *testing.T) {
	data := readFixture(t, "rss-pub-uxcollective.xml")
	items, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("got 0 items, want >= 1")
	}
	it := items[0]
	if strings.TrimSpace(it.Body) == "" {
		t.Fatal("item carries no content:encoded body")
	}
	// The body is real HTML, not the truncated description snippet.
	if !strings.Contains(it.Body, "<p>") {
		t.Errorf("body does not look like content:encoded HTML: %.80q", it.Body)
	}
	if it.Author != "Fabricio Teixeira" {
		t.Errorf("author = %q", it.Author)
	}
}

// TestParseInvalid asserts a non-RSS payload returns an error rather than
// panicking or silently yielding zero items as success.
func TestParseInvalid(t *testing.T) {
	if _, err := Parse([]byte("not xml at all <<<")); err == nil {
		t.Fatal("expected error for invalid XML")
	}
}

// TestFeedURL covers the ref -> feed-URL mapping the command relies on for
// auto-detection: @user, tag/<tag>, and a bare publication slug.
func TestFeedURL(t *testing.T) {
	tests := []struct {
		ref  string
		want string
	}{
		{"@nickbabich", "https://medium.com/feed/@nickbabich"},
		{"nickbabich", "https://medium.com/feed/@nickbabich"}, // user kind passes a bare handle? no — handled by Kind. See below.
	}
	// The first case is the authoritative one; the second is intentionally
	// exercised through KindUser to prove the @ is normalized in.
	if got := FeedURL(KindUser, "nickbabich"); got != tests[1].want {
		t.Errorf("FeedURL(user, nickbabich) = %q, want %q", got, tests[1].want)
	}
	if got := FeedURL(KindUser, "@nickbabich"); got != tests[0].want {
		t.Errorf("FeedURL(user, @nickbabich) = %q, want %q", got, tests[0].want)
	}
	if got := FeedURL(KindTag, "ux"); got != "https://medium.com/feed/tag/ux" {
		t.Errorf("FeedURL(tag, ux) = %q", got)
	}
	if got := FeedURL(KindTag, "tag/ux"); got != "https://medium.com/feed/tag/ux" {
		t.Errorf("FeedURL(tag, tag/ux) = %q", got)
	}
	if got := FeedURL(KindPublication, "uxdesign-cc"); got != "https://medium.com/feed/uxdesign-cc" {
		t.Errorf("FeedURL(pub, uxdesign-cc) = %q", got)
	}
}

// TestClassifyRef covers the auto-detection rules.
func TestClassifyRef(t *testing.T) {
	tests := []struct {
		ref  string
		want Kind
	}{
		{"@nickbabich", KindUser},
		{"tag/ux", KindTag},
		{"tag/product-design", KindTag},
		{"uxdesign-cc", KindPublication},
		{"better-programming", KindPublication},
	}
	for _, tt := range tests {
		if got := ClassifyRef(tt.ref); got != tt.want {
			t.Errorf("ClassifyRef(%q) = %v, want %v", tt.ref, got, tt.want)
		}
	}
}

// TestCapabilities asserts the RSS source advertises only Feed, and that the
// unsupported methods return ErrUnsupported (never a panic).
func TestCapabilities(t *testing.T) {
	s := New(nil)
	caps := s.Capabilities()
	if !caps.Feed {
		t.Error("RSS source should advertise Feed")
	}
	if caps.Search || caps.ReadArticle || caps.AuthorArchive {
		t.Errorf("RSS source advertised an unsupported capability: %+v", caps)
	}
	ctx := context.Background()
	if _, err := s.Search(ctx, "q", 10); err != source.ErrUnsupported {
		t.Errorf("Search err = %v, want ErrUnsupported", err)
	}
	if _, err := s.ReadArticle(ctx, "x"); err != source.ErrUnsupported {
		t.Errorf("ReadArticle err = %v, want ErrUnsupported", err)
	}
	if _, err := s.AuthorArchive(ctx, "x", 10); err != source.ErrUnsupported {
		t.Errorf("AuthorArchive err = %v, want ErrUnsupported", err)
	}
	if s.Name() == "" {
		t.Error("Name() empty")
	}
}

// TestToPostSummaries asserts the parsed items map cleanly onto the normalized
// source.PostSummary model the resolver/command consume.
func TestToPostSummaries(t *testing.T) {
	data := readFixture(t, "rss-tag-ux.xml")
	items, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	sums := ToPostSummaries(items)
	if len(sums) != len(items) {
		t.Fatalf("ToPostSummaries len = %d, want %d", len(sums), len(items))
	}
	var _ []source.PostSummary = sums
	if sums[0].Title == "" || sums[0].URL == "" || sums[0].PublishedAt.IsZero() {
		t.Errorf("first summary missing core fields: %+v", sums[0])
	}
}
