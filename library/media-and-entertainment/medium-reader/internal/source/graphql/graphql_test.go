// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package graphql

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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
