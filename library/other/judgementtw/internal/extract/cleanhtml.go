// Copyright 2026 wayne-lai. Licensed under Apache-2.0. See LICENSE.

package extract

import (
	"html"
	"regexp"
	"strings"
)

var (
	scriptTag  = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	styleTag   = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	commentTag = regexp.MustCompile(`(?s)<!--.*?-->`)
	tagAny     = regexp.MustCompile(`<[^>]+>`)
	wsRun      = regexp.MustCompile(`\s+`)
	nbspChar   = regexp.MustCompile(`&nbsp;|\x{00A0}`)
)

// CleanHTML strips tags, comments, scripts/styles, decodes entities, collapses
// whitespace and returns plain text. Suitable for full-text indexing and
// citation/sentence extraction.
func CleanHTML(s string) string {
	if s == "" {
		return ""
	}
	s = scriptTag.ReplaceAllString(s, " ")
	s = styleTag.ReplaceAllString(s, " ")
	s = commentTag.ReplaceAllString(s, " ")
	s = tagAny.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = nbspChar.ReplaceAllString(s, " ")
	s = wsRun.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// ExtractByID returns the inner HTML of the first <div id="<id>">...</div>
// (or any tag with that id). Returns empty string when not found. The match
// is greedy and stops at the structurally-matching closing tag — sufficient
// for the well-formed output produced by judicial.gov.tw.
func ExtractByID(htmlBody, id string) string {
	pattern := regexp.MustCompile(`(?is)<\w+[^>]*\bid="` + regexp.QuoteMeta(id) + `"[^>]*>(.*?)</\w+>`)
	m := pattern.FindStringSubmatch(htmlBody)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// ExtractByClass returns the inner HTML of the first element matching the
// given class attribute. Tolerates multi-class declarations (`class="foo bar"`).
func ExtractByClass(htmlBody, class string) string {
	pattern := regexp.MustCompile(`(?is)<\w+[^>]*\bclass="(?:[^"]*\s)?` + regexp.QuoteMeta(class) + `(?:\s[^"]*)?"[^>]*>(.*?)</\w+>`)
	m := pattern.FindStringSubmatch(htmlBody)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}
