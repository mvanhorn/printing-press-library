// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package uberjobs

import (
	"html"
	"regexp"
	"strings"
	"unicode"
)

var (
	// Whole-document wrappers: 64 of 584 live descriptions are full HTML
	// documents whose <title> is "<p> Cleaned Document </p>".
	reDropBlocks = regexp.MustCompile(`(?is)<(script|style|head|title|noscript)\b[^>]*>.*?</(script|style|head|title|noscript)\s*>`)
	reComment    = regexp.MustCompile(`(?s)<!--.*?-->`)
	reBreak      = regexp.MustCompile(`(?i)<br\s*/?>`)
	reBlockEnd   = regexp.MustCompile(`(?i)</(p|div|h[1-6]|ul|ol|table|tr|section|article|header|footer|blockquote)\s*>`)
	reListItem   = regexp.MustCompile(`(?i)<li\b[^>]*>`)
	reTag        = regexp.MustCompile(`(?s)<[^>]*>`)
	reSpaces     = regexp.MustCompile(`[ \t\f\v\x{00a0}]+`)
	reBlankLines = regexp.MustCompile(`\n{3,}`)
)

// StripHTML converts an HTML fragment or document into readable plain text:
// block ends become newlines, list items become "- " lines, entities are
// decoded, and whitespace is collapsed. It runs before any regex matching.
func StripHTML(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	s = reComment.ReplaceAllString(s, " ")
	s = reDropBlocks.ReplaceAllString(s, " ")
	s = reBreak.ReplaceAllString(s, "\n")
	s = reListItem.ReplaceAllString(s, "\n- ")
	s = reBlockEnd.ReplaceAllString(s, "\n")
	s = reTag.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = reSpaces.ReplaceAllString(s, " ")
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "- " || line == "-" {
			continue
		}
		out = append(out, line)
	}
	s = strings.Join(out, "\n")
	s = reBlankLines.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// Sentences splits plain text into sentence-sized units for evidence output.
// StripHTML puts headings and list items on their own lines, so a line break
// always ends a unit; inside a line, terminal punctuation ends one.
func Sentences(text string) []string {
	var out []string
	var b strings.Builder
	flush := func() {
		if s := strings.TrimSpace(b.String()); s != "" {
			out = append(out, s)
		}
		b.Reset()
	}
	for _, line := range strings.Split(text, "\n") {
		for _, f := range strings.Fields(line) {
			b.WriteString(f)
			b.WriteByte(' ')
			if endsSentence(f) {
				flush()
			}
		}
		flush()
	}
	return out
}

// sentenceAbbrev are dotted words that do not end a sentence.
var sentenceAbbrev = map[string]bool{
	"sr.": true, "jr.": true, "mr.": true, "mrs.": true, "ms.": true, "dr.": true, "st.": true,
	"e.g.": true, "i.e.": true, "etc.": true, "vs.": true, "inc.": true, "ltd.": true, "co.": true,
	"no.": true, "approx.": true,
}

// endsSentence reports whether a word ends a sentence: terminal punctuation,
// except common abbreviations ("Sr.", "e.g.") and single-letter initials.
func endsSentence(word string) bool {
	if strings.HasSuffix(word, "!") || strings.HasSuffix(word, "?") {
		return true
	}
	if !strings.HasSuffix(word, ".") {
		return false
	}
	w := strings.ToLower(strings.TrimLeft(word, "([\"'"))
	if sentenceAbbrev[w] {
		return false
	}
	if r := []rune(w); len(r) == 2 && unicode.IsLetter(r[0]) {
		return false
	}
	return true
}
