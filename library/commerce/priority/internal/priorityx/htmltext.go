// Copyright 2026 alon-auto and contributors. Licensed under Apache-2.0. See LICENSE.

package priorityx

import (
	"html"
	"regexp"
	"strings"
)

var (
	reStyleScript = regexp.MustCompile(`(?is)<(style|script)[^>]*>.*?</(style|script)>`)
	reBreaks      = regexp.MustCompile(`(?i)<(br\s*/?|/p|/div|/li|/tr)>`)
	reTags        = regexp.MustCompile(`<[^>]*>`)
	reBlankLines  = regexp.MustCompile(`\n{3,}`)
	reSpaceRuns   = regexp.MustCompile(`[ \t]{2,}`)
)

// StripHTML converts Priority text-subform HTML into readable plain text:
// drops style/script blocks, turns block-level closers and <br> into
// newlines, strips remaining tags, unescapes entities, and collapses
// whitespace runs.
func StripHTML(s string) string {
	s = reStyleScript.ReplaceAllString(s, "")
	s = reBreaks.ReplaceAllString(s, "\n")
	s = reTags.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = reSpaceRuns.ReplaceAllString(s, " ")
	var lines []string
	for _, ln := range strings.Split(s, "\n") {
		lines = append(lines, strings.TrimSpace(ln))
	}
	s = strings.Join(lines, "\n")
	s = reBlankLines.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}
