package httperr

import (
	"strings"
	"testing"
)

func TestSnippet_PlainShortBody(t *testing.T) {
	got := Snippet([]byte("  too many requests  "))
	if got != "too many requests" {
		t.Fatalf("expected trimmed plain body, got %q", got)
	}
}

func TestSnippet_StripsHTMLAndCollapsesWhitespace(t *testing.T) {
	html := []byte("<!DOCTYPE html><html><body>\n  <h1>Service Unavailable</h1>\n  <p>Try again later.</p>\n</body></html>")
	got := Snippet(html)
	if strings.Contains(got, "<") || strings.Contains(got, ">") {
		t.Fatalf("expected HTML tags stripped, got %q", got)
	}
	if !strings.Contains(got, "Service Unavailable") {
		t.Fatalf("expected visible text preserved, got %q", got)
	}
}

func TestSnippet_TruncatesLongBody(t *testing.T) {
	long := strings.Repeat("a", 500)
	got := Snippet([]byte(long))
	if len(got) > maxSnippet+3 {
		t.Fatalf("expected truncation to ~%d chars (+ ellipsis), got %d", maxSnippet, len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("expected ellipsis suffix on truncated output, got %q", got)
	}
}

func TestSnippet_HandlesMultiByteUTF8(t *testing.T) {
	// Build a string of CJK runes that, before rune-aware truncation, would
	// split a multi-byte character. Each rune is 3 bytes in UTF-8.
	rune3byte := strings.Repeat("漢", 250) // 750 bytes, 250 runes
	got := Snippet([]byte(rune3byte))
	// The output must still be valid UTF-8 (no replacement chars from a split).
	if strings.ContainsRune(got, '�') {
		t.Fatalf("Snippet split a multi-byte rune; got %q", got)
	}
}

func TestSnippet_EmptyInput(t *testing.T) {
	if got := Snippet(nil); got != "" {
		t.Fatalf("expected empty string for nil input, got %q", got)
	}
	if got := Snippet([]byte("")); got != "" {
		t.Fatalf("expected empty string for empty input, got %q", got)
	}
}
