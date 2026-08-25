// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package graphql

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/source"
)

// fixturePath resolves a fixture relative to the repo's testdata directory. The
// graphql package lives at internal/source/graphql, so testdata is three dirs up.
func fixturePath(t *testing.T, parts ...string) string {
	t.Helper()
	all := append([]string{"..", "..", "..", "testdata"}, parts...)
	return filepath.Join(all...)
}

func readFixture(t *testing.T, parts ...string) []byte {
	t.Helper()
	b, err := os.ReadFile(fixturePath(t, parts...))
	if err != nil {
		t.Fatalf("reading fixture %v: %v", parts, err)
	}
	return b
}

// TestParseSearch is the spec's hermetic search contract: feed the saved page-0
// response into ParseSearch and expect the exact item ids (in order) plus the
// next page index (1, per the fixture's pagingInfo.next.page).
func TestParseSearch(t *testing.T) {
	body := readFixture(t, "fixtures", "g4-search-product-builder.page0.json")
	items, next, err := ParseSearch(body)
	if err != nil {
		t.Fatalf("ParseSearch: %v", err)
	}
	wantIDs := []string{
		"f8fab42387ee", "0d4f7be1ab7c", "dda893cd5558", "41e8ce3d1753", "3178d894051d",
		"470f65a4fc1f", "b34bc05606ae", "b275bc14ecd3", "21f860cfbbc6", "beab955be6dd",
	}
	if len(items) != len(wantIDs) {
		t.Fatalf("got %d items, want %d", len(items), len(wantIDs))
	}
	for i, w := range wantIDs {
		if items[i].ID != w {
			t.Errorf("item[%d].ID = %q, want %q", i, items[i].ID, w)
		}
	}
	if next != 1 {
		t.Errorf("next page = %d, want 1", next)
	}

	// Spot-check a fully-projected summary so the field mapping is verified, not
	// just the ids.
	first := items[0]
	if first.Author != "莫力全 Kyle Mo" {
		t.Errorf("item[0].Author = %q", first.Author)
	}
	if first.AuthorID != "fac5c5351760" {
		t.Errorf("item[0].AuthorID = %q", first.AuthorID)
	}
	if first.Username != "oldmo860617" {
		t.Errorf("item[0].Username = %q", first.Username)
	}
	if first.URL != "https://medium.com/p/f8fab42387ee" {
		t.Errorf("item[0].URL = %q", first.URL)
	}
	if first.PublishedAt.IsZero() {
		t.Error("item[0].PublishedAt is zero")
	}
}

// TestParseAuthorArchive is the spec's hermetic archive contract: feed the saved
// page-0 response into ParseAuthorArchive and expect the post ids plus the next
// cursor (from = "L1779192785759", per the fixture's pagingInfo.next.from).
func TestParseAuthorArchive(t *testing.T) {
	body := readFixture(t, "fixtures", "g5-nickbabich-archive.page0.json")
	items, nextFrom, name, err := ParseAuthorArchive(body)
	if err != nil {
		t.Fatalf("ParseAuthorArchive: %v", err)
	}
	if len(items) != 25 {
		t.Fatalf("got %d items, want 25", len(items))
	}
	// Assert the first and last ids on the page, and a couple in the middle.
	if items[0].ID != "43c711bbc07d" {
		t.Errorf("items[0].ID = %q, want 43c711bbc07d", items[0].ID)
	}
	if items[24].ID != "697aaabe76c8" {
		t.Errorf("items[24].ID = %q, want 697aaabe76c8", items[24].ID)
	}
	if nextFrom != "L1779192785759" {
		t.Errorf("nextFrom = %q, want L1779192785759", nextFrom)
	}
	if name != "Nick Babich" {
		t.Errorf("author name = %q, want Nick Babich", name)
	}
	// Author propagation: every summary carries the user.name and the post's
	// creator id/username.
	if items[0].Author != "Nick Babich" {
		t.Errorf("items[0].Author = %q", items[0].Author)
	}
	if items[0].AuthorID != "bcab753a4d4e" {
		t.Errorf("items[0].AuthorID = %q", items[0].AuthorID)
	}
	if items[0].Username != "101" {
		t.Errorf("items[0].Username = %q", items[0].Username)
	}
	if items[0].URL != "https://medium.com/p/43c711bbc07d" {
		t.Errorf("items[0].URL = %q", items[0].URL)
	}
}

// TestParseAuthorArchiveEngagement asserts the widened archive query's
// engagement fields (clapCount/voterCount/readingTime/wordCount/postResponses/
// tags) decode onto PostSummary — the data author-compare averages. Tag slugs
// are flattened to []string and empty slugs skipped.
func TestParseAuthorArchiveEngagement(t *testing.T) {
	body := []byte(`{"data":{"user":{"id":"u1","name":"Quincy","homepagePostsConnection":{"posts":[{"id":"abcdef012345","title":"T","firstPublishedAt":1700000000000,"clapCount":4919,"voterCount":631,"readingTime":17.836,"wordCount":4382,"postResponses":{"count":18},"tags":[{"normalizedTagSlug":"writing"},{"normalizedTagSlug":""},{"normalizedTagSlug":"tech"}],"creator":{"id":"u1","username":"quincylarson"}}],"pagingInfo":{"next":null}}}}}`)
	items, _, _, err := ParseAuthorArchive(body)
	if err != nil {
		t.Fatalf("ParseAuthorArchive: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	p := items[0]
	if p.Claps != 4919 {
		t.Errorf("Claps = %d, want 4919", p.Claps)
	}
	if p.Voters != 631 {
		t.Errorf("Voters = %d, want 631", p.Voters)
	}
	if p.Responses != 18 {
		t.Errorf("Responses = %d, want 18", p.Responses)
	}
	if p.ReadingTime != 17.836 {
		t.Errorf("ReadingTime = %v, want 17.836", p.ReadingTime)
	}
	if p.WordCount != 4382 {
		t.Errorf("WordCount = %d, want 4382", p.WordCount)
	}
	if len(p.Tags) != 2 || p.Tags[0] != "writing" || p.Tags[1] != "tech" {
		t.Errorf("Tags = %v, want [writing tech] (empty slug skipped)", p.Tags)
	}
}

// TestParseSearchNoNext asserts that a final page (pagingInfo.next null) yields
// next == -1, the loop-termination signal.
func TestParseSearchNoNext(t *testing.T) {
	body := []byte(`{"data":{"search":{"posts":{"__typename":"SearchPost","pagingInfo":{"next":null},"items":[{"id":"abcdef012345","title":"T","creator":{"id":"c1","name":"N","username":"u"}}]}}}}`)
	items, next, err := ParseSearch(body)
	if err != nil {
		t.Fatalf("ParseSearch: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if next != -1 {
		t.Errorf("next = %d, want -1 (no next page)", next)
	}
}

// TestParseAuthorArchiveNoNext asserts that a final page (pagingInfo.next null)
// yields nextFrom == "", the loop-termination signal.
func TestParseAuthorArchiveNoNext(t *testing.T) {
	body := []byte(`{"data":{"user":{"id":"u1","name":"Solo","homepagePostsConnection":{"posts":[{"id":"abcdef012345","title":"T","creator":{"id":"u1","username":"solo"}}],"pagingInfo":{"next":null}}}}}`)
	items, nextFrom, _, err := ParseAuthorArchive(body)
	if err != nil {
		t.Fatalf("ParseAuthorArchive: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if nextFrom != "" {
		t.Errorf("nextFrom = %q, want empty (no next page)", nextFrom)
	}
}

// TestParseSearchGraphQLError asserts that a GraphQL "errors" block degrades to
// the typed ErrSurfaceUnavailable (Medium changed the surface), not a panic or
// an opaque success.
func TestParseSearchGraphQLError(t *testing.T) {
	body := []byte(`{"errors":[{"message":"Cannot query field \"posts\" on type \"Search\""}],"data":null}`)
	_, _, err := ParseSearch(body)
	if err == nil {
		t.Fatal("ParseSearch error = nil, want ErrSurfaceUnavailable")
	}
	if !errors.Is(err, source.ErrSurfaceUnavailable) {
		t.Errorf("err = %v, want errors.Is ErrSurfaceUnavailable", err)
	}
}

// TestParseAuthorArchiveGraphQLError mirrors the search degradation case.
func TestParseAuthorArchiveGraphQLError(t *testing.T) {
	body := []byte(`{"errors":[{"message":"Variable \"$id\" got invalid value"}],"data":null}`)
	_, _, _, err := ParseAuthorArchive(body)
	if err == nil {
		t.Fatal("ParseAuthorArchive error = nil, want ErrSurfaceUnavailable")
	}
	if !errors.Is(err, source.ErrSurfaceUnavailable) {
		t.Errorf("err = %v, want errors.Is ErrSurfaceUnavailable", err)
	}
}

// TestCapabilities asserts the graphql source advertises only Search +
// AuthorArchive and that the other methods return ErrUnsupported (never a panic).
func TestCapabilities(t *testing.T) {
	s := New(nil)
	caps := s.Capabilities()
	if !caps.Search || !caps.AuthorArchive {
		t.Errorf("graphql source should advertise Search and AuthorArchive; got %+v", caps)
	}
	if caps.Feed || caps.ReadArticle {
		t.Errorf("graphql source advertised an unsupported capability: %+v", caps)
	}
	ctx := context.Background()
	if _, err := s.Feed(ctx, "x"); err != source.ErrUnsupported {
		t.Errorf("Feed err = %v, want ErrUnsupported", err)
	}
	if _, err := s.ReadArticle(ctx, "x"); err != source.ErrUnsupported {
		t.Errorf("ReadArticle err = %v, want ErrUnsupported", err)
	}
	if s.Name() != "graphql" {
		t.Errorf("Name() = %q", s.Name())
	}
}

// TestQueryConstantsMatchSpec guards against accidental drift of the inline
// queries away from the live-validated shapes (the exact strings Medium accepts).
func TestQueryConstantsMatchSpec(t *testing.T) {
	if !contains(SearchQuery, "pagingOptions:$pagingOptions") {
		t.Error("SearchQuery missing page-based pagingOptions argument")
	}
	if !contains(SearchQuery, "creator{id name username}") {
		t.Error("SearchQuery missing creator projection")
	}
	if !contains(AuthorArchiveQuery, "homepagePostsConnection(paging:$paging,includeDistributedResponses:true)") {
		t.Error("AuthorArchiveQuery missing homepagePostsConnection with includeDistributedResponses")
	}
	if !contains(AuthorArchiveQuery, "pagingInfo{next{from limit}}") {
		t.Error("AuthorArchiveQuery missing cursor pagingInfo")
	}
	// The widened archive query must request the engagement fields author-compare
	// reports. The minimal fallback query intentionally omits them.
	for _, f := range []string{"clapCount", "voterCount", "readingTime", "wordCount", "postResponses{count}", "tags{normalizedTagSlug}"} {
		if !contains(AuthorArchiveQuery, f) {
			t.Errorf("AuthorArchiveQuery missing engagement field %q", f)
		}
		if contains(authorArchiveQueryMinimal, f) {
			t.Errorf("authorArchiveQueryMinimal should NOT contain engagement field %q (it is the resilient fallback)", f)
		}
	}
	if !contains(authorArchiveQueryMinimal, "posts{id title firstPublishedAt creator{id username}}") {
		t.Error("authorArchiveQueryMinimal missing the minimal post projection")
	}
}

// roundTripFunc adapts a function to http.RoundTripper for hermetic transport
// stubs — no real network, full control over the canned response per request.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

// TestAuthorArchiveFallsBackOnRejectedQuery is the F4 resilience guard: when the
// engagement-widened query is REJECTED by the server (a renamed/removed field →
// GraphQL "errors" block), AuthorArchive must re-issue the minimal query so core
// mirroring still succeeds (engagement comes back zero), instead of failing the
// whole archive.
func TestAuthorArchiveFallsBackOnRejectedQuery(t *testing.T) {
	var calls int
	var sawMinimal bool
	stub := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		buf, _ := io.ReadAll(r.Body)
		if strings.Contains(string(buf), "clapCount") {
			// Widened query — simulate Medium rejecting an engagement field.
			return jsonResponse(200, `{"errors":[{"message":"Cannot query field \"clapCount\" on type \"Post\""}],"data":null}`), nil
		}
		// Minimal fallback query — return a valid single-post page.
		sawMinimal = true
		return jsonResponse(200, `{"data":{"user":{"id":"u1","name":"Solo","homepagePostsConnection":{"posts":[{"id":"abcdef012345","title":"T","firstPublishedAt":1700000000000,"creator":{"id":"u1","username":"solo"}}],"pagingInfo":{"next":null}}}}}`), nil
	})
	s := New(&http.Client{Transport: stub})

	items, err := s.AuthorArchive(context.Background(), "u1", 10)
	if err != nil {
		t.Fatalf("AuthorArchive should succeed via fallback, got: %v", err)
	}
	if len(items) != 1 || items[0].ID != "abcdef012345" {
		t.Fatalf("items = %+v, want one post abcdef012345 from the minimal fallback", items)
	}
	if items[0].Claps != 0 {
		t.Errorf("Claps = %d, want 0 (the fallback query carries no engagement)", items[0].Claps)
	}
	if !sawMinimal {
		t.Error("fallback never issued the minimal query")
	}
	if calls != 2 {
		t.Errorf("HTTP calls = %d, want 2 (widened rejected, then minimal)", calls)
	}
}

// TestAuthorArchiveNoRetryOnTransportError asserts the fallback does NOT fire on
// a transport/HTTP outage — retrying the minimal query there is pointless and
// would just double the requests. A 500 yields exactly one call and a typed error.
func TestAuthorArchiveNoRetryOnTransportError(t *testing.T) {
	var calls int
	stub := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return jsonResponse(500, `upstream boom`), nil
	})
	s := New(&http.Client{Transport: stub})

	_, err := s.AuthorArchive(context.Background(), "u1", 10)
	if err == nil {
		t.Fatal("AuthorArchive err = nil, want ErrSurfaceUnavailable on HTTP 500")
	}
	if !errors.Is(err, source.ErrSurfaceUnavailable) {
		t.Errorf("err = %v, want errors.Is ErrSurfaceUnavailable", err)
	}
	if calls != 1 {
		t.Errorf("HTTP calls = %d, want 1 (no fallback on transport outage)", calls)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
