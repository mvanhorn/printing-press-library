// Copyright 2026 paul-bockewitz. Licensed under Apache-2.0. See LICENSE.

package svc

import (
	"strconv"
	"strings"
)

// Card is one result from a personal card search (/svc/search/search).
type Card struct {
	ID      string `json:"id"`
	Snippet string `json:"snippet,omitempty"`
}

// BuildSearchRequest encodes the /svc/search/search request body:
//
//	field 1 (string) the search query
func BuildSearchRequest(query string) []byte {
	return appendStringField(nil, 1, query)
}

// DecodeSearchResults parses the /svc/search/search response: a repeated field 1
// where each entry is a card result carrying an id (varint field 1) and, for
// some entries, rendered content. The content's exact field placement varies
// (it can be nested and HTML-bearing), so the human-readable snippet is pulled
// with a recursive first-text scan.
func DecodeSearchResults(buf []byte) ([]Card, error) {
	fields, err := Fields(buf)
	var cards []Card
	for _, f := range fields {
		if f.Num != 1 || f.WireType != wireBytes {
			continue
		}
		sub, _ := Fields(f.Bytes)
		c := Card{}
		if id := FirstVarint(sub, 1); id != 0 {
			c.ID = strconv.FormatUint(id, 10)
		}
		c.Snippet = firstText(f.Bytes, 4)
		if c.ID != "" || c.Snippet != "" {
			cards = append(cards, c)
		}
	}
	return cards, err
}

// firstText recursively returns the first human-readable string found in a
// protobuf message, with whitespace collapsed and length capped. Returns "" if
// none is found within the depth budget.
func firstText(buf []byte, depth int) string {
	if depth < 0 {
		return ""
	}
	fields, _ := Fields(buf)
	for _, f := range fields {
		if f.WireType != wireBytes {
			continue
		}
		s := string(f.Bytes)
		// Prefer a cleanly-printable leaf string. Bytes that carry protobuf
		// framing (sub-message tags/lengths) contain control bytes and are not
		// "clean", so we descend into them instead of treating them as text.
		if isCleanText(s) {
			return snippet(s)
		}
		if t := firstText(f.Bytes, depth-1); t != "" {
			return t
		}
	}
	return ""
}

// isCleanText reports whether s is non-empty and contains only printable runes
// and ordinary whitespace — no control bytes that would indicate the bytes are
// actually a packed sub-message rather than a leaf string.
func isCleanText(s string) bool {
	if strings.TrimSpace(s) == "" {
		return false
	}
	for _, r := range s {
		if r == '\n' || r == '\t' || r == '\r' {
			continue
		}
		if r < 0x20 || r == 0x7f || r == 0xFFFD {
			return false
		}
	}
	return true
}

// snippet collapses whitespace and caps the length for a one-line preview.
func snippet(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	const max = 120
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}
