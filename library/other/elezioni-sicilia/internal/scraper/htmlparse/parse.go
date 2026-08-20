// Package htmlparse contains shared regexes and helpers for the static-HTML
// scrapers (comunali and regionali).
package htmlparse

import (
	"regexp"
	"strings"
)

// TableRe matches a full <table>...</table> block (non-greedy).
var TableRe = regexp.MustCompile(`(?is)<table[^>]*>(.*?)</table>`)

// TrRe matches a single <tr>...</tr> row.
var TrRe = regexp.MustCompile(`(?is)<tr[^>]*>(.*?)</tr>`)

// TdRe matches a single <td>...</td> or <th>...</th> cell.
var TdRe = regexp.MustCompile(`(?is)<t[dh][^>]*>(.*?)</t[dh]>`)

// WsRe matches runs of whitespace.
var WsRe = regexp.MustCompile(`\s+`)

var tagRe = regexp.MustCompile(`<[^>]+>`)

// CleanCell strips HTML tags and resolves common entities from a cell's
// inner HTML, returning the trimmed text.
func CleanCell(s string) string {
	s = tagRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&#039;", "'")
	s = strings.TrimSpace(s)
	return s
}
