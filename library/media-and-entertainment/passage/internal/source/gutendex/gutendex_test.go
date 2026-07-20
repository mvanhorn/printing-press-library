// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.
package gutendex

import (
	"net/url"
	"strings"
	"testing"
)

func TestCleanExcerptSkipsFrontMatter(t *testing.T) {
	raw := "The Project Gutenberg eBook of Whatever\n" +
		"*** START OF THE PROJECT GUTENBERG EBOOK WHATEVER ***\n\n" +
		"[Illustration: GEORGE ALLEN PUBLISHER 156 CHARING CROSS ROAD]\n\n" +
		"CONTENTS\n\n" +
		"     FIRST BOOK\n     SECOND BOOK\n\n" +
		"It is a truth universally acknowledged, that a single man in possession " +
		"of a good fortune, must be in want of a wife. However little known the " +
		"feelings or views of such a man may be on his first entering a neighbourhood.\n\n" +
		"*** END OF THE PROJECT GUTENBERG EBOOK ***\n"

	got := CleanExcerpt(raw, 500)
	if !strings.HasPrefix(got, "It is a truth universally acknowledged") {
		t.Fatalf("expected excerpt to start at the real prose, got: %q", got)
	}
	for _, bad := range []string{"GEORGE ALLEN", "CONTENTS", "FIRST BOOK", "START OF", "END OF"} {
		if strings.Contains(got, bad) {
			t.Errorf("excerpt should not contain front matter %q; got: %q", bad, got)
		}
	}
}

func TestCleanExcerptSkipsIntroToFirstBook(t *testing.T) {
	// Mimics Gutenberg 2680 (Meditations): a biographical INTRODUCTION whose
	// prose contains a sentence beginning "First Book he sets down…", then the
	// real standalone "THE FIRST BOOK" heading, then the actual first verse.
	raw := "*** START OF THE PROJECT GUTENBERG EBOOK MEDITATIONS ***\n\n" +
		"INTRODUCTION\n\n" +
		"MARCUS AURELIUS ANTONINUS was born on April 26, A.D. 121, and was a noble " +
		"Roman emperor whose life this long introduction now recounts at length.\n\n" +
		"First Book he sets down to account all the debts due to his kinsfolk and " +
		"teachers, a habit of gratitude that runs through the whole of the work.\n\n" +
		"THE FIRST BOOK\n\n" +
		"I. Of my grandfather Verus I have learned to be gentle and meek, and to " +
		"refrain from all anger and passion.\n\n" +
		"*** END OF THE PROJECT GUTENBERG EBOOK ***\n"

	got := CleanExcerpt(raw, 400)
	if !strings.HasPrefix(got, "I. Of my grandfather Verus") {
		t.Fatalf("expected excerpt to start at the first verse, got: %q", got)
	}
	if strings.Contains(got, "was born") || strings.Contains(got, "he sets down") {
		t.Errorf("excerpt must skip the biographical introduction; got: %q", got)
	}
}

func TestCleanExcerptSkipsTableOfContents(t *testing.T) {
	// Mimics an anthology (Sherlock Holmes) whose story headings are bare roman
	// numerals, indistinguishable from its table of contents — so the TOC run
	// must be rejected as non-prose and the first story opening returned.
	raw := "*** START OF THE PROJECT GUTENBERG EBOOK ***\n\n" +
		"CONTENTS\n\n" +
		"I. A Scandal in Bohemia II. The Red-Headed League III. A Case of Identity " +
		"IV. The Boscombe Valley Mystery V. The Five Orange Pips\n\n" +
		"I. A SCANDAL IN BOHEMIA\n\n" +
		"To Sherlock Holmes she is always the woman. I have seldom heard him mention " +
		"her under any other name, for in his eyes she eclipses the whole of her sex.\n\n" +
		"*** END OF THE PROJECT GUTENBERG EBOOK ***\n"

	got := CleanExcerpt(raw, 400)
	if !strings.HasPrefix(got, "To Sherlock Holmes") {
		t.Fatalf("expected the story opening, got: %q", got)
	}
	if strings.Contains(got, "Red-Headed League") {
		t.Errorf("excerpt must skip the table of contents; got: %q", got)
	}
}

func TestCleanExcerptTruncates(t *testing.T) {
	body := "This is a full sentence of real prose that goes on for a while and certainly exceeds the small limit we will pass in to force truncation of the output text."
	raw := "*** START OF THE PROJECT GUTENBERG EBOOK X ***\n\n" + body + "\n"
	got := CleanExcerpt(raw, 40)
	if len([]rune(got)) > 41 { // 40 + the ellipsis
		t.Errorf("expected ~40 runes, got %d: %q", len([]rune(got)), got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("expected ellipsis suffix, got %q", got)
	}
}

func TestCleanExcerptFiltersBoilerplateFromShortLineFallback(t *testing.T) {
	raw := "The Project Gutenberg eBook of Poems\n" +
		"PUBLISHER AND COPYRIGHT INFORMATION\n" +
		"*** START OF THE PROJECT GUTENBERG EBOOK POEMS ***\n\n" +
		"Because I could not stop for Death—\n" +
		"He kindly stopped for me—\n" +
		"The Carriage held but just Ourselves—\n" +
		"And Immortality.\n\n" +
		"*** END OF THE PROJECT GUTENBERG EBOOK POEMS ***\n"

	got := CleanExcerpt(raw, 45)
	if strings.Contains(got, "Gutenberg") || strings.Contains(got, "PUBLISHER") {
		t.Fatalf("fallback leaked front matter: %q", got)
	}
	if !strings.HasPrefix(got, "Because I could not stop for Death") {
		t.Fatalf("fallback did not preserve verse: %q", got)
	}
	if len([]rune(got)) > 46 {
		t.Fatalf("fallback exceeded the requested bound: %d runes", len([]rune(got)))
	}
}

func TestURLQueryEscapesReservedCharacters(t *testing.T) {
	query := " C++ 100% = useful "
	got := urlQuery(query)
	values, err := url.ParseQuery("search=" + got)
	if err != nil {
		t.Fatalf("ParseQuery: %v", err)
	}
	if got := values.Get("search"); got != strings.TrimSpace(query) {
		t.Fatalf("decoded search = %q, want %q", got, strings.TrimSpace(query))
	}
}

func TestTextURLPrefersUTF8(t *testing.T) {
	b := Book{Formats: map[string]string{
		"text/html":                    "https://x/h.html",
		"text/plain; charset=us-ascii": "https://x/a.txt",
		"text/plain; charset=utf-8":    "https://x/u.txt",
		"application/zip":              "https://x/z.zip",
	}}
	if got := b.TextURL(); got != "https://x/u.txt" {
		t.Errorf("TextURL = %q, want the utf-8 .txt", got)
	}
	if got := (Book{Formats: map[string]string{"application/epub+zip": "x"}}).TextURL(); got != "" {
		t.Errorf("TextURL with no plain text = %q, want empty", got)
	}
}
