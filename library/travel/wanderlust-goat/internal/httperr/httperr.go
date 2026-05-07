// Package httperr formats non-2xx response bodies for inclusion in error
// messages. Many upstream services return multi-kilobyte HTML error pages
// (Cloudflare blocks, Apache 500 pages, full XHTML doctype + <body>) that
// drown the actual cause when embedded verbatim in an error string.
//
// Snippet returns at most 200 characters of the response body, with leading
// and trailing whitespace removed and HTML tags stripped to a best-effort
// single-line summary. The output is safe to embed in an error formatted as
// "%s returned %d: %s".
package httperr

import (
	"regexp"
	"strings"
)

const maxSnippet = 200

var htmlTag = regexp.MustCompile(`(?s)<[^>]*>`)
var collapseWS = regexp.MustCompile(`\s+`)

// Snippet returns a short, single-line summary of body suitable for embedding
// in an error message. HTML tags are stripped, whitespace is collapsed, and
// the result is truncated to 200 characters.
func Snippet(body []byte) string {
	s := string(body)
	if strings.Contains(s, "<") && strings.Contains(s, ">") {
		s = htmlTag.ReplaceAllString(s, " ")
	}
	s = collapseWS.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	if len(s) > maxSnippet {
		// Trim at rune boundary to avoid splitting a multi-byte character.
		runes := []rune(s)
		if len(runes) > maxSnippet {
			return string(runes[:maxSnippet]) + "..."
		}
	}
	return s
}
