// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/passage/internal/source/gutendex"
)

func TestNormTitle(t *testing.T) {
	cases := map[string]string{
		"The Meditations":           "meditations",
		"Pride and Prejudice":       "pride and prejudice",
		"MOBY-DICK; or, The Whale":  "moby dick or the whale", // "the" only stripped at the front
		"  Walden  ":                "walden",
		"Meditations (Illustrated)": "meditations illustrated",
	}
	for in, want := range cases {
		if got := normTitle(in); got != want {
			t.Errorf("normTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMatchGutenberg(t *testing.T) {
	books := []gutendex.Book{
		{ID: 2680, Title: "Meditations"},
		{ID: 1342, Title: "Pride and Prejudice"},
	}
	// exact + containment match (no author on the fixtures → title match stands)
	if b, ok := matchGutenberg(normTitle("The Meditations of Marcus Aurelius"), "", books); !ok || b.ID != 2680 {
		t.Errorf("expected Meditations to match 2680, got %+v ok=%v", b, ok)
	}
	if b, ok := matchGutenberg(normTitle("Pride and Prejudice"), "", books); !ok || b.ID != 1342 {
		t.Errorf("expected Pride and Prejudice to match 1342, got %+v ok=%v", b, ok)
	}
	// no false positive for an unrelated title
	if _, ok := matchGutenberg(normTitle("Meditaciones"), "", books); ok {
		t.Error("Meditaciones (Spanish) should not match Meditations")
	}
	// prefix-match guard: a book *about* Walden is not Walden itself
	wal := []gutendex.Book{{ID: 26289, Title: "Walden"}}
	if _, ok := matchGutenberg(normTitle("After Walden"), "", wal); ok {
		t.Error("'After Walden' should not match the book 'Walden' (suffix, not prefix)")
	}
	if b, ok := matchGutenberg(normTitle("Walden, or, Life in the Woods"), "", wal); !ok || b.ID != 26289 {
		t.Errorf("a Walden title variant should match 26289, got %+v ok=%v", b, ok)
	}
	// too-short guard
	if _, ok := matchGutenberg("a", "", books); ok {
		t.Error("a 1-char title must not match")
	}
	// author-collision guard: same title, different author → no match
	essays := []gutendex.Book{{ID: 575, Title: "Essays", Authors: []gutendex.Author{{Name: "Bacon, Francis"}}}}
	if _, ok := matchGutenberg(normTitle("Essays"), "Ralph Waldo Emerson", essays); ok {
		t.Error("Emerson's Essays must not match Bacon's Essays")
	}
	if b, ok := matchGutenberg(normTitle("Essays"), "Francis Bacon", essays); !ok || b.ID != 575 {
		t.Errorf("Bacon's Essays should match, got %+v ok=%v", b, ok)
	}
}

func TestAuthorsAgree(t *testing.T) {
	if !authorsAgree("Jane Austen", "Austen, Jane") {
		t.Error("same author in different orders should agree")
	}
	if !authorsAgree("", "Austen, Jane") || !authorsAgree("Jane Austen", "Unknown") {
		t.Error("an unknown author can't disprove a title match")
	}
	if authorsAgree("Ralph Waldo Emerson", "Bacon, Francis") {
		t.Error("distinct authors must not agree")
	}
}
