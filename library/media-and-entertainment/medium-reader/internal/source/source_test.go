// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package source

import (
	"context"
	"errors"
	"testing"
)

// stubSource is a fully in-memory Source used to exercise the Resolver's
// dispatch, fallback, and degradation logic with NO network. Each method
// returns a canned result or a canned error; calls are counted so tests can
// assert ordering and fallthrough precisely.
type stubSource struct {
	name string
	caps Caps

	feedOut    []PostSummary
	feedErr    error
	readOut    *Article
	readErr    error
	searchOut  []PostSummary
	searchErr  error
	archiveOut []PostSummary
	archiveErr error

	feedCalls    int
	readCalls    int
	searchCalls  int
	archiveCalls int
}

func (s *stubSource) Name() string       { return s.name }
func (s *stubSource) Capabilities() Caps { return s.caps }

func (s *stubSource) Feed(ctx context.Context, ref string) ([]PostSummary, error) {
	s.feedCalls++
	return s.feedOut, s.feedErr
}

func (s *stubSource) ReadArticle(ctx context.Context, idOrURL string) (*Article, error) {
	s.readCalls++
	return s.readOut, s.readErr
}

func (s *stubSource) Search(ctx context.Context, query string, limit int) ([]PostSummary, error) {
	s.searchCalls++
	return s.searchOut, s.searchErr
}

func (s *stubSource) AuthorArchive(ctx context.Context, userIDOrHandle string, max int) ([]PostSummary, error) {
	s.archiveCalls++
	return s.archiveOut, s.archiveErr
}

func TestResolverFeedUsesFirstCapableSource(t *testing.T) {
	primary := &stubSource{
		name:    "primary",
		caps:    Caps{Feed: true},
		feedOut: []PostSummary{{ID: "p1"}},
	}
	secondary := &stubSource{
		name:    "secondary",
		caps:    Caps{Feed: true},
		feedOut: []PostSummary{{ID: "s1"}},
	}
	r := NewResolver(primary, secondary)

	got, err := r.Feed(context.Background(), "@someone")
	if err != nil {
		t.Fatalf("Feed: unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "p1" {
		t.Fatalf("Feed: want primary result [p1], got %+v", got)
	}
	if primary.feedCalls != 1 {
		t.Fatalf("primary should be called once, got %d", primary.feedCalls)
	}
	if secondary.feedCalls != 0 {
		t.Fatalf("secondary should NOT be called when primary succeeds, got %d", secondary.feedCalls)
	}
}

func TestResolverFeedFallsBackOnError(t *testing.T) {
	primary := &stubSource{
		name:    "primary",
		caps:    Caps{Feed: true},
		feedErr: errors.New("boom"),
	}
	secondary := &stubSource{
		name:    "secondary",
		caps:    Caps{Feed: true},
		feedOut: []PostSummary{{ID: "s1"}},
	}
	r := NewResolver(primary, secondary)

	got, err := r.Feed(context.Background(), "@someone")
	if err != nil {
		t.Fatalf("Feed: expected fallback success, got error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "s1" {
		t.Fatalf("Feed: want fallback result [s1], got %+v", got)
	}
	if primary.feedCalls != 1 || secondary.feedCalls != 1 {
		t.Fatalf("expected both tried once: primary=%d secondary=%d", primary.feedCalls, secondary.feedCalls)
	}
}

func TestResolverSkipsIncapableSources(t *testing.T) {
	// An RSS-shaped source (Feed only) sitting ahead of a GraphQL-shaped
	// source (Search only): a Search call must skip the RSS source entirely
	// (no wasted call) and dispatch straight to the capable one.
	feedOnly := &stubSource{name: "rss", caps: Caps{Feed: true}}
	searchOnly := &stubSource{
		name:      "graphql",
		caps:      Caps{Search: true},
		searchOut: []PostSummary{{ID: "g1"}},
	}
	r := NewResolver(feedOnly, searchOnly)

	got, err := r.Search(context.Background(), "product builder", 10)
	if err != nil {
		t.Fatalf("Search: unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "g1" {
		t.Fatalf("Search: want [g1], got %+v", got)
	}
	if feedOnly.searchCalls != 0 {
		t.Fatalf("feed-only source must not be called for Search, got %d", feedOnly.searchCalls)
	}
	if searchOnly.searchCalls != 1 {
		t.Fatalf("graphql source should be called once, got %d", searchOnly.searchCalls)
	}
}

// TestResolverGracefulDegradationGraphQLDown is the core degradation contract:
// when the GraphQL surface is the only thing that can serve search/author-
// archive and it is down, those commands return a typed
// ErrSurfaceUnavailable (never a panic) — while feed/read, served by other
// sources, keep working.
func TestResolverGracefulDegradationGraphQLDown(t *testing.T) {
	rss := &stubSource{
		name:    "rss",
		caps:    Caps{Feed: true},
		feedOut: []PostSummary{{ID: "p1"}},
	}
	page := &stubSource{
		name:    "page",
		caps:    Caps{ReadArticle: true},
		readOut: &Article{ID: "a1", Title: "ok"},
	}
	// GraphQL source is capable of search + archive but every call errors,
	// simulating Medium changing/removing its internal API.
	graphql := &stubSource{
		name:       "graphql",
		caps:       Caps{Search: true, AuthorArchive: true},
		searchErr:  errors.New("graphql 503"),
		archiveErr: errors.New("graphql 503"),
	}
	r := NewResolver(rss, page, graphql)

	// feed still works
	feed, err := r.Feed(context.Background(), "tag/ux")
	if err != nil || len(feed) != 1 || feed[0].ID != "p1" {
		t.Fatalf("feed should still work with graphql down: feed=%+v err=%v", feed, err)
	}

	// read still works
	art, err := r.ReadArticle(context.Background(), "a1")
	if err != nil || art == nil || art.ID != "a1" {
		t.Fatalf("read should still work with graphql down: art=%+v err=%v", art, err)
	}

	// search degrades to the typed sentinel
	_, err = r.Search(context.Background(), "anything", 10)
	if err == nil {
		t.Fatalf("search: expected ErrSurfaceUnavailable, got nil")
	}
	if !errors.Is(err, ErrSurfaceUnavailable) {
		t.Fatalf("search: expected ErrSurfaceUnavailable, got %v", err)
	}

	// author-archive degrades to the typed sentinel
	_, err = r.AuthorArchive(context.Background(), "bcab753a4d4e", 200)
	if err == nil {
		t.Fatalf("author-archive: expected ErrSurfaceUnavailable, got nil")
	}
	if !errors.Is(err, ErrSurfaceUnavailable) {
		t.Fatalf("author-archive: expected ErrSurfaceUnavailable, got %v", err)
	}
}

// TestResolverDegradesWhenNoCapableSource verifies that a command with no
// capable source at all (e.g. search with only an RSS source configured)
// also degrades to the typed sentinel rather than returning a nil/nil or
// panicking.
func TestResolverDegradesWhenNoCapableSource(t *testing.T) {
	rss := &stubSource{name: "rss", caps: Caps{Feed: true}}
	r := NewResolver(rss)

	_, err := r.Search(context.Background(), "x", 5)
	if !errors.Is(err, ErrSurfaceUnavailable) {
		t.Fatalf("expected ErrSurfaceUnavailable when no capable source, got %v", err)
	}
	if rss.searchCalls != 0 {
		t.Fatalf("rss must not be called for Search, got %d", rss.searchCalls)
	}
}

// TestResolverUnderlyingErrorIsWrapped ensures operators can still see the
// root cause through the degradation wrapper.
func TestResolverUnderlyingErrorIsWrapped(t *testing.T) {
	graphql := &stubSource{
		name:      "graphql",
		caps:      Caps{Search: true},
		searchErr: errors.New("connection refused"),
	}
	r := NewResolver(graphql)

	_, err := r.Search(context.Background(), "x", 5)
	if !errors.Is(err, ErrSurfaceUnavailable) {
		t.Fatalf("expected ErrSurfaceUnavailable, got %v", err)
	}
	if got := err.Error(); !contains(got, "connection refused") {
		t.Fatalf("expected underlying cause in message, got %q", got)
	}
}

func TestResolverSourcesViewIsCopy(t *testing.T) {
	a := &stubSource{name: "a", caps: Caps{Feed: true}}
	r := NewResolver(a)
	view := r.Sources()
	if len(view) != 1 || view[0].Name() != "a" {
		t.Fatalf("Sources view wrong: %+v", view)
	}
	// Mutating the returned slice must not affect the resolver's internal order.
	view[0] = &stubSource{name: "tampered"}
	if r.sources[0].Name() != "a" {
		t.Fatalf("Sources() leaked internal slice; resolver mutated to %s", r.sources[0].Name())
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
