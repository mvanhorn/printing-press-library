// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package page

import (
	"strings"
	"testing"
)

// preloadedBodyFixture is a synthetic minimal page with NO __APOLLO_STATE__ but
// a __PRELOADED_STATE__ carrying a normalized Post: cache with a renderable
// body. It exercises the second link in Parse's fallback chain.
const preloadedBodyFixture = `<!DOCTYPE html><html><body>
<script>
window.__PRELOADED_STATE__ = {
  "Post:abc123def456": {
    "__typename": "Post",
    "id": "abc123def456",
    "title": "Fallback From Preloaded State",
    "isLocked": false,
    "creator": { "__ref": "User:u1" },
    "content({\"postMeteringOptions\":{\"referrer\":\"\"}})": {
      "isLockedPreviewOnly": false,
      "bodyModel": { "__ref": "Body:b1" }
    }
  },
  "User:u1": { "__typename": "User", "id": "u1", "name": "Pre Loader" },
  "Body:b1": { "__typename": "BodyModel", "paragraphs": [ { "__ref": "Para:p1" }, { "__ref": "Para:p2" } ] },
  "Para:p1": { "__typename": "Paragraph", "type": "H3", "text": "Section One" },
  "Para:p2": { "__typename": "Paragraph", "type": "P", "text": "A paragraph from the preloaded fallback path." }
};
</script>
</body></html>`

// TestParsePreloadedStateBody asserts that a page carrying only
// __PRELOADED_STATE__ (no APOLLO_STATE) still yields a full Article with its
// body reconstructed via the shared post walk.
func TestParsePreloadedStateBody(t *testing.T) {
	art, err := Parse([]byte(preloadedBodyFixture), "abc123def456")
	if err != nil {
		t.Fatalf("Parse preloaded fixture: %v", err)
	}
	if art.Title != "Fallback From Preloaded State" {
		t.Errorf("Title = %q, want %q", art.Title, "Fallback From Preloaded State")
	}
	if art.Author != "Pre Loader" {
		t.Errorf("Author = %q, want Pre Loader", art.Author)
	}
	if art.IsPreviewOnly {
		t.Errorf("IsPreviewOnly = true, want false for a free preloaded post")
	}
	// The first heading promotes to the H1 title; the paragraph follows.
	if !strings.Contains(art.Markdown, "# Section One") {
		t.Errorf("markdown missing promoted first heading; got: %q", art.Markdown)
	}
	if !strings.Contains(art.Markdown, "A paragraph from the preloaded fallback path.") {
		t.Errorf("markdown missing body paragraph; got: %q", art.Markdown)
	}
}

// lockedPreloadedNoContentFixture is a __PRELOADED_STATE__ locked post with NO
// content node — the exact shape where the APOLLO and PRELOADED paths used to
// diverge on IsPreviewOnly. After unifying both on postToArticle, a locked post
// with no content must be IsPreviewOnly==true on this path too.
const lockedPreloadedNoContentFixture = `<!DOCTYPE html><html><body>
<script>
window.__PRELOADED_STATE__ = {
  "Post:locked9876":  {
    "__typename": "Post",
    "id": "locked9876",
    "title": "A Locked Post With No Content Node",
    "isLocked": true,
    "creator": { "__ref": "User:u9" }
  },
  "User:u9": { "__typename": "User", "id": "u9", "name": "Locked Author" }
};
</script>
</body></html>`

// TestParsePreloadedLockedNoContentIsPreviewOnly pins the IsPreviewOnly fix: a
// locked post with no content node, parsed via the __PRELOADED_STATE__ path,
// must report IsPreviewOnly==true (mirroring the __APOLLO_STATE__ path), not the
// zero value the divergent code returned before.
func TestParsePreloadedLockedNoContentIsPreviewOnly(t *testing.T) {
	art, err := Parse([]byte(lockedPreloadedNoContentFixture), "locked9876")
	if err != nil {
		t.Fatalf("Parse locked preloaded fixture: %v", err)
	}
	if !art.IsLocked {
		t.Errorf("IsLocked = false, want true")
	}
	if !art.IsPreviewOnly {
		t.Errorf("IsPreviewOnly = false, want true (locked post, no content node) — APOLLO/PRELOADED paths must agree")
	}
	if art.Markdown != "" {
		t.Errorf("Markdown = %q, want empty for a content-less locked post", art.Markdown)
	}
}

// jsonLDMetadataFixture is a metadata-only page: no APOLLO_STATE, no
// PRELOADED_STATE, just an Article JSON-LD block. It is the last link in the
// fallback chain and must recover title/author/date without a body and without
// panicking.
const jsonLDMetadataFixture = `<!DOCTYPE html><html><head>
<script type="application/ld+json">
{"@context":"https://schema.org","@type":"Article","headline":"Recovered From JSON-LD","url":"https://medium.com/p/ld00112233","author":{"@type":"Person","name":"LD Author"},"datePublished":"2026-02-03T10:11:12.000Z"}
</script>
</head><body><p>rendered body the parser cannot read</p></body></html>`

// TestParseJSONLDMetadataOnly asserts the JSON-LD fallback recovers metadata
// (title, author, date) on a page with no embedded state, and that it produces a
// usable Article with empty Markdown rather than erroring or panicking.
func TestParseJSONLDMetadataOnly(t *testing.T) {
	art, err := Parse([]byte(jsonLDMetadataFixture), "ld00112233")
	if err != nil {
		t.Fatalf("Parse json-ld fixture: %v", err)
	}
	if art.Title != "Recovered From JSON-LD" {
		t.Errorf("Title = %q, want %q", art.Title, "Recovered From JSON-LD")
	}
	if art.Author != "LD Author" {
		t.Errorf("Author = %q, want LD Author", art.Author)
	}
	if art.PublishedAt.IsZero() {
		t.Error("PublishedAt is zero; JSON-LD datePublished should have been parsed")
	}
	if art.PublishedAt.Year() != 2026 || art.PublishedAt.Month() != 2 {
		t.Errorf("PublishedAt = %v, want 2026-02-...", art.PublishedAt)
	}
	if art.Markdown != "" {
		t.Errorf("Markdown = %q, want empty (JSON-LD carries no body model)", art.Markdown)
	}
	if art.ID != "ld00112233" {
		t.Errorf("ID = %q, want ld00112233 (from idHint)", art.ID)
	}
}

// h4Fixture is a synthetic APOLLO_STATE post whose body has H2/H3/H4 headings
// after the first (promoted) heading. It pins the heading-depth mapping so H4
// renders as #### (four hashes), distinct from H3's ###.
const h4Fixture = `<!DOCTYPE html><html><body>
<script>
window.__APOLLO_STATE__ = {
  "Post:depth0001": {
    "__typename": "Post",
    "id": "depth0001",
    "title": "Heading Depths",
    "isLocked": false,
    "creator": { "__ref": "User:hu" },
    "content({\"postMeteringOptions\":{\"referrer\":\"\"}})": {
      "isLockedPreviewOnly": false,
      "bodyModel": { "__ref": "Body:hb" }
    }
  },
  "User:hu": { "__typename": "User", "id": "hu", "name": "Depth Author" },
  "Body:hb": { "__typename": "BodyModel", "paragraphs": [
    { "__ref": "Para:t" }, { "__ref": "Para:h2" }, { "__ref": "Para:h3" }, { "__ref": "Para:h4" }
  ] },
  "Para:t":  { "__typename": "Paragraph", "type": "H2", "text": "Title Heading" },
  "Para:h2": { "__typename": "Paragraph", "type": "H2", "text": "Level Two" },
  "Para:h3": { "__typename": "Paragraph", "type": "H3", "text": "Level Three" },
  "Para:h4": { "__typename": "Paragraph", "type": "H4", "text": "Level Four" }
};
</script>
</body></html>`

// TestRenderH4Depth asserts H4 maps to "#### " (four hashes) while H2/H3 keep
// their depths, and the first heading still promotes to the "# " title.
func TestRenderH4Depth(t *testing.T) {
	art, err := Parse([]byte(h4Fixture), "depth0001")
	if err != nil {
		t.Fatalf("Parse h4 fixture: %v", err)
	}
	md := art.Markdown
	checks := []string{
		"# Title Heading", // first heading promoted to H1
		"## Level Two",    // H2
		"### Level Three", // H3
		"#### Level Four", // H4 (the fix: was "### " before)
	}
	for _, c := range checks {
		if !strings.Contains(md, c) {
			t.Errorf("markdown missing %q; got:\n%s", c, md)
		}
	}
	// Guard against H4 collapsing to H3: the "Level Four" line, taken whole,
	// must start with exactly four hashes, not three. A substring check would
	// false-positive because "#### " contains "### ", so we match the full line.
	for _, line := range strings.Split(md, "\n") {
		if strings.HasSuffix(line, "Level Four") && !strings.HasPrefix(line, "#### ") {
			t.Errorf("H4 line did not render at #### depth: %q", line)
		}
	}
}
