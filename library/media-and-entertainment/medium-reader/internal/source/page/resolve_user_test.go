// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package page

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/source"
)

// TestParseUserID_HermeticFixture is the hermetic contract for keyless
// handle->id resolution: a saved profile-page fixture that embeds the owner's
// User entity alongside several referenced authors' User entities must resolve
// the @handle ("101") to the OWNER's id (bcab753a4d4e) by matching username,
// not by picking the first User: key encountered. No network.
func TestParseUserID_HermeticFixture(t *testing.T) {
	html := readFixture(t, "fixtures", "profile-@101.synthetic.html")

	// With the handle hint, the username match must pick the owner.
	if got := ParseUserID(html, "101"); got != "bcab753a4d4e" {
		t.Errorf("ParseUserID(handle=101) = %q, want bcab753a4d4e", got)
	}
	// A leading @ on the hint must be tolerated (stripped before matching).
	if got := ParseUserID(html, "@101"); got != "bcab753a4d4e" {
		t.Errorf("ParseUserID(handle=@101) = %q, want bcab753a4d4e", got)
	}
	// No hint: the ROOT_QUERY userResult ref still pins the owner.
	if got := ParseUserID(html, ""); got != "bcab753a4d4e" {
		t.Errorf("ParseUserID(no hint) = %q, want bcab753a4d4e", got)
	}
}

// TestParseUserID_NoApolloState asserts a page with no __APOLLO_STATE__ yields
// "" (the caller turns this into a clear usage error, never a panic).
func TestParseUserID_NoApolloState(t *testing.T) {
	if got := ParseUserID([]byte("<html><body>no state here</body></html>"), "101"); got != "" {
		t.Errorf("ParseUserID with no apollo state = %q, want empty", got)
	}
}

// TestParseUserID_UnmatchedHintDoesNotFallBack guards the soft-404 bug: Medium
// returns HTTP 200 for a nonexistent handle, serving a page that still embeds
// unrelated authors' User entities. When a handle hint is supplied but matches
// no User on the page, resolution must yield "" (the caller turns this into a
// clear usage error) — it must NEVER fall back to an arbitrary embedded author,
// or `author-archive <garbage>` would silently mirror the wrong writer's corpus.
func TestParseUserID_UnmatchedHintDoesNotFallBack(t *testing.T) {
	html := readFixture(t, "fixtures", "profile-@101.synthetic.html")

	// The @101 page embeds the owner (username "101") plus several referenced
	// authors. A hint that matches none of them must not resolve to any of them.
	if got := ParseUserID(html, "notarealhandle"); got != "" {
		t.Errorf("ParseUserID(unmatched hint) = %q, want empty (must not fall back to an arbitrary embedded author)", got)
	}
}

// TestLiveResolveUserID is the live, env-gated counterpart asserting @101
// resolves to bcab753a4d4e against Medium's real profile page. Gated behind
// MEDIUM_LIVE=1 so the default suite stays offline-green.
func TestLiveResolveUserID(t *testing.T) {
	if os.Getenv("MEDIUM_LIVE") != "1" {
		t.Skip("live test: set MEDIUM_LIVE=1 to run (default suite stays offline)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	s := New(source.NewHTTPClient(60 * time.Second))
	got, err := s.ResolveUserID(ctx, "@101")
	if err != nil {
		t.Fatalf("live ResolveUserID(@101): %v", err)
	}
	if got != "bcab753a4d4e" {
		t.Errorf("live ResolveUserID(@101) = %q, want bcab753a4d4e", got)
	}
}
