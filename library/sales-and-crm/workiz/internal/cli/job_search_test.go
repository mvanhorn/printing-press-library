// Copyright 2026 Eldar and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestSnippetAroundLongText(t *testing.T) {
	text := "The quick brown fox jumps over the lazy dog and keeps running past the old barn down the road, then circles back home before it gets dark outside tonight."
	got := snippetAround(text, "lazy dog")
	if got == text {
		t.Fatalf("expected a truncated snippet around the match, got the full text back")
	}
	if len(got) >= len(text) {
		t.Fatalf("expected snippet shorter than source text, got length %d vs %d", len(got), len(text))
	}
}
