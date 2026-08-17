// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package page

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/source"
)

// fixturePath resolves a fixture relative to the repo's testdata directory. The
// page package lives at internal/source/page, so testdata is three dirs up.
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

// oracleMarkdown loads the v1 oracle markdown for the locked preview article.
func oracleMarkdown(t *testing.T) string {
	t.Helper()
	b := readFixture(t, "oracle", "g3-designers2026-markdown.json")
	var doc struct {
		Results struct {
			Markdown string `json:"markdown"`
		} `json:"results"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("parsing oracle markdown json: %v", err)
	}
	return doc.Results.Markdown
}

// TestParseLockedPreview is the spec's hermetic contract: the anonymized,
// locked-article fixture parses to IsPreviewOnly==true, the exact title, and
// extracted Markdown whose head matches the oracle's head (preview = first ~10
// paragraphs). We assert the key headings and sentences appear in order rather
// than byte-for-byte, because the oracle markdown came from RapidAPI's
// server-side renderer while v2 reconstructs it client-side.
func TestParseLockedPreview(t *testing.T) {
	html := readFixture(t, "fixtures", "g3-article-818e7841df9c.anon.html")
	art, err := Parse(html, "818e7841df9c")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !art.IsPreviewOnly {
		t.Errorf("IsPreviewOnly = false, want true (locked preview fixture)")
	}
	if !art.IsLocked {
		t.Errorf("IsLocked = false, want true")
	}
	if art.Title != "Designers will OWN 2026–2030" {
		t.Errorf("Title = %q, want %q", art.Title, "Designers will OWN 2026–2030")
	}
	if art.ID != "818e7841df9c" {
		t.Errorf("ID = %q", art.ID)
	}
	if art.Author != "Michal Malewicz" {
		t.Errorf("Author = %q, want Michal Malewicz", art.Author)
	}
	if art.AuthorID != "fde1eb3eb589" {
		t.Errorf("AuthorID = %q", art.AuthorID)
	}
	if art.PublishedAt.IsZero() {
		t.Error("PublishedAt is zero")
	}

	got := art.Markdown
	if got == "" {
		t.Fatal("Markdown is empty")
	}

	// The fixture preview holds ~10 paragraphs. Assert the leading H1 title.
	if !strings.HasPrefix(got, "# Designers will OWN 2026–2030\n\n") {
		t.Errorf("markdown does not start with the H1 title; head=%q", head(got, 120))
	}

	// Key markers must appear, in this order, near the top — proving heading
	// levels, image rendering, blockquote, and inline markups all reconstruct.
	wantInOrder := []string{
		"# Designers will OWN 2026–2030",
		"### Why design is the most essential, future proof job right now!",
		"![](https://miro.medium.com/1*MRso-URPxMkm5osjn498aw.jpeg)",
		"**U.S. Bureau of Labor Statistics[1]**",
		"> So let me say it out loud: UI Design has a future!",
		"![](https://miro.medium.com/1*QxIDU8OEXyZTYt8sdCyMzQ.png)",
		"_the UI design_",
	}
	assertInOrder(t, got, wantInOrder)

	// The reconstructed head must align with the oracle's head: take the oracle
	// up to the second blockquote and assert every non-empty line of it appears
	// in our output, in order.
	oracle := oracleMarkdown(t)
	oracleHead := firstNBlocks(oracle, 7) // covers H1, H3, IMG, P(bold), PQ, IMG, P
	assertOracleHeadAligns(t, got, oracleHead)
}

// TestParseFreeArticleFullBody asserts that a captured free (non-locked) article
// renders the COMPLETE body: every paragraph in the embedded bodyModel becomes a
// rendered block, and the article is not flagged preview-only. This is the
// completeness check the spec requires alongside the locked-preview test.
func TestParseFreeArticleFullBody(t *testing.T) {
	dir := fixturePath(t, "fixtures")
	matches, _ := filepath.Glob(filepath.Join(dir, "*.free.html"))
	if len(matches) == 0 {
		t.Skip("no *.free.html fixture captured; run capture step (see capture_test.go)")
	}
	html, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("reading free fixture: %v", err)
	}
	id := freeIDFromName(matches[0])

	art, err := Parse(html, id)
	if err != nil {
		t.Fatalf("Parse free article: %v", err)
	}
	if art.IsPreviewOnly {
		t.Errorf("IsPreviewOnly = true, want false for a free article")
	}
	if art.Markdown == "" {
		t.Fatal("free article rendered empty markdown")
	}

	// Count rendered blocks vs embedded paragraphs that carry renderable
	// content. The rendered body must reproduce one block per embedded
	// paragraph (consecutive list items are the only multi-line block kind, and
	// these test articles don't rely on that for the count parity check; if a
	// future fixture does, this assertion uses >= to stay robust).
	embedded := countEmbeddedParagraphs(t, html, id)
	rendered := countRenderedBlocks(art.Markdown)
	if embedded == 0 {
		t.Fatal("embedded paragraph count is 0; fixture may be malformed")
	}
	if rendered < embedded {
		t.Errorf("rendered blocks (%d) < embedded paragraphs (%d): body is incomplete", rendered, embedded)
	}
}

// TestCapabilities asserts the page source advertises only ReadArticle and that
// the other methods return ErrUnsupported (never a panic).
func TestCapabilities(t *testing.T) {
	s := New(nil)
	caps := s.Capabilities()
	if !caps.ReadArticle {
		t.Error("page source should advertise ReadArticle")
	}
	if caps.Feed || caps.Search || caps.AuthorArchive {
		t.Errorf("page source advertised an unsupported capability: %+v", caps)
	}
	ctx := context.Background()
	if _, err := s.Feed(ctx, "x"); err != source.ErrUnsupported {
		t.Errorf("Feed err = %v, want ErrUnsupported", err)
	}
	if _, err := s.Search(ctx, "q", 10); err != source.ErrUnsupported {
		t.Errorf("Search err = %v, want ErrUnsupported", err)
	}
	if _, err := s.AuthorArchive(ctx, "x", 10); err != source.ErrUnsupported {
		t.Errorf("AuthorArchive err = %v, want ErrUnsupported", err)
	}
	if s.Name() != "page" {
		t.Errorf("Name() = %q", s.Name())
	}
}

// TestExtractID covers URL/id normalization the read command relies on.
func TestExtractID(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"818e7841df9c", "818e7841df9c"},
		{"https://medium.com/p/818e7841df9c", "818e7841df9c"},
		{"https://medium.com/p/818e7841df9c?source=foo", "818e7841df9c"},
		{"https://michalmalewicz.medium.com/designers-will-own-2026-2030-818e7841df9c", "818e7841df9c"},
		{"https://michalmalewicz.medium.com/designers-will-own-2026-2030-818e7841df9c?source=x", "818e7841df9c"},
		{"designers-will-own-2026-2030-818e7841df9c", "818e7841df9c"},
		{"not-an-id", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := ExtractID(tt.in); got != tt.want {
			t.Errorf("ExtractID(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestApplyMarkupsMultibyte guards the UTF-16-offset -> rune-index conversion:
// a markup after a multibyte character (em dash) must still wrap the right span.
func TestApplyMarkupsMultibyte(t *testing.T) {
	// "a — bold end": the em dash is one UTF-16 unit, so "bold" starts at unit
	// 4 and ends at unit 8. A naive byte-offset renderer would mis-wrap here
	// because the em dash is 3 UTF-8 bytes.
	text := "a — bold end"
	mks := []markup{{Type: "STRONG", Start: 4, End: 8}}
	got := applyMarkups(text, mks)
	want := "a — **bold** end"
	if got != want {
		t.Errorf("applyMarkups multibyte = %q, want %q", got, want)
	}
}

// TestApplyMarkupsTrailingSpace asserts trailing whitespace inside a span is
// pushed outside the emphasis markers (CommonMark-style, matching the oracle).
func TestApplyMarkupsTrailingSpace(t *testing.T) {
	text := "see the bold word here"
	// Span "bold " (with trailing space): start=8, end=13.
	mks := []markup{{Type: "STRONG", Start: 8, End: 13}}
	got := applyMarkups(text, mks)
	want := "see the **bold** word here"
	if got != want {
		t.Errorf("applyMarkups trailing-space = %q, want %q", got, want)
	}
}

// ---- test helpers ----------------------------------------------------------

func head(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// assertInOrder fails if any of subs is missing, or if they do not appear in
// the given order in s.
func assertInOrder(t *testing.T, s string, subs []string) {
	t.Helper()
	pos := 0
	for _, sub := range subs {
		idx := strings.Index(s[pos:], sub)
		if idx < 0 {
			t.Errorf("missing or out-of-order marker %q (searching from offset %d)", sub, pos)
			continue
		}
		pos += idx + len(sub)
	}
}

// firstNBlocks returns the first n blank-line-separated blocks of s.
func firstNBlocks(s string, n int) []string {
	blocks := strings.Split(s, "\n\n")
	if len(blocks) > n {
		blocks = blocks[:n]
	}
	return blocks
}

// assertOracleHeadAligns checks each oracle head block appears in got, in order.
// Comparison is normalized (collapse internal whitespace) to tolerate benign
// renderer differences while still proving structural fidelity.
func assertOracleHeadAligns(t *testing.T, got string, oracleHead []string) {
	t.Helper()
	normGot := normalizeWS(got)
	pos := 0
	for _, blk := range oracleHead {
		nb := normalizeWS(blk)
		if nb == "" {
			continue
		}
		idx := strings.Index(normGot[pos:], nb)
		if idx < 0 {
			t.Errorf("oracle head block not found in order: %q", head(blk, 80))
			continue
		}
		pos += idx + len(nb)
	}
}

// normalizeWS collapses internal whitespace and folds typographic punctuation
// to ASCII. The fold matters because the oracle markdown came from RapidAPI's
// server-side renderer, which normalizes smart quotes/apostrophes (' " ' ") and
// dashes to their ASCII forms, whereas v2 faithfully preserves Medium's source
// JSON. Folding both sides makes the structural-fidelity comparison robust to
// that benign character-set difference (the spec asks for a normalized prefix,
// not byte-exact parity).
func normalizeWS(s string) string {
	s = foldTypography(s)
	return strings.Join(strings.Fields(s), " ")
}

func foldTypography(s string) string {
	r := strings.NewReplacer(
		"’", "'", // right single quote / apostrophe
		"‘", "'", // left single quote
		"“", `"`, // left double quote
		"”", `"`, // right double quote
		"—", "-", // em dash
		"–", "-", // en dash
		"…", "...", // ellipsis
	)
	return r.Replace(s)
}

// freeIDFromName recovers the article id from a "<id>.free.html" fixture name.
func freeIDFromName(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, ".free.html")
	return base
}

// countRenderedBlocks counts blank-line-separated blocks in the markdown.
func countRenderedBlocks(md string) int {
	n := 0
	for _, b := range strings.Split(md, "\n\n") {
		if strings.TrimSpace(b) != "" {
			n++
		}
	}
	return n
}

// countEmbeddedParagraphs counts the paragraphs in the embedded bodyModel of the
// fixture, mirroring what the renderer consumes — the parity baseline.
func countEmbeddedParagraphs(t *testing.T, html []byte, id string) int {
	t.Helper()
	raw := extractAssignment(string(html), "__APOLLO_STATE__")
	if raw == "" {
		t.Fatal("no __APOLLO_STATE__ in free fixture")
	}
	var cache apolloCache
	if err := json.Unmarshal([]byte(raw), &cache); err != nil {
		t.Fatalf("decoding apollo cache: %v", err)
	}
	key := pickPostKey(cache, id)
	if key == "" {
		t.Fatal("no Post: key in free fixture")
	}
	var post map[string]json.RawMessage
	if err := json.Unmarshal(cache[key], &post); err != nil {
		t.Fatalf("decoding post: %v", err)
	}
	contentRaw := fieldByPrefix(post, "content(")
	if contentRaw == nil {
		t.Fatal("no content() field in free fixture")
	}
	var content map[string]json.RawMessage
	if err := json.Unmarshal(contentRaw, &content); err != nil {
		t.Fatalf("decoding content: %v", err)
	}
	paras := collectParagraphs(cache, content["bodyModel"])
	// Count only paragraphs the renderer turns into a block (every type does,
	// except an unknown type with empty text). Mirror renderMarkdown's drop
	// rule so the parity check is apples-to-apples.
	n := 0
	for _, p := range paras {
		if isRenderable(p) {
			n++
		}
	}
	return n
}

// isRenderable mirrors renderMarkdown's per-paragraph emit decision for the
// parity count: known types always emit; an unknown type emits only when it
// carries non-empty text.
func isRenderable(p paragraph) bool {
	switch p.Type {
	case "H2", "H3", "H4", "P", "PRE", "BQ", "PQ", "ULI", "OLI", "IMG", "HR":
		return true
	default:
		return strings.TrimSpace(p.Text) != ""
	}
}
