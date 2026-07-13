// Copyright 2026 Angelo Pullen and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

// TestSplitFrontmatterCRLF pins the 2026-07-13 incident: files re-saved with
// CRLF endings carry a `---\r\n` frontmatter delimiter, which the LF-only
// delimiter regex rejects — the note parses with ZERO frontmatter keys while
// remaining perfectly valid to a human and to Obsidian. normalizeNewlines
// before splitFrontmatter is the fix; these tests document both halves.
func TestSplitFrontmatterCRLF(t *testing.T) {
	crlf := "---\r\ntype: email\r\nclient: test-client\r\n---\r\n\r\n# Body\r\n"

	// The raw CRLF form must fail the split — this is the silent failure the
	// normalization exists to prevent. If this ever starts passing, the regex
	// grew CRLF tolerance and normalizeNewlines can be retired.
	if fm, _ := splitFrontmatter(crlf); fm != "" {
		t.Errorf("raw CRLF input unexpectedly split frontmatter: %q (normalization may be redundant now)", fm)
	}

	// Normalized, the same bytes must parse.
	fm, body := splitFrontmatter(normalizeNewlines(crlf))
	if fm != "type: email\nclient: test-client" {
		t.Errorf("normalized CRLF frontmatter = %q, want the two yaml lines", fm)
	}
	if body != "\n# Body\n" {
		t.Errorf("normalized CRLF body = %q", body)
	}

	// LF input passes through normalizeNewlines untouched.
	lf := "---\ntype: email\n---\nbody\n"
	if normalizeNewlines(lf) != lf {
		t.Error("normalizeNewlines altered a pure-LF document")
	}
}
