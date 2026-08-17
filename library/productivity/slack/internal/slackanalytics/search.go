// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package slackanalytics

import (
	"regexp"
	"strings"
)

// queryTokenRE splits a recall query into searchable tokens, dropping
// punctuation that FTS5 would treat as syntax.
var queryTokenRE = regexp.MustCompile(`[\p{L}\p{N}_]+`)

// QueryTokens splits a free-text query into lower-cased tokens.
func QueryTokens(query string) []string {
	matches := queryTokenRE.FindAllString(strings.ToLower(query), -1)
	tokens := make([]string, 0, len(matches))
	for _, m := range matches {
		tokens = append(tokens, m)
	}
	return tokens
}

// MatchesAllTokens reports whether every token of query appears in text,
// case-insensitively. FTS5's porter stemmer happily returns "deployment" for
// "deploy" — desirable — but it also returns rows whose only match was in a
// non-text field, which is not. This is the precision gate applied to FTS
// candidates so a returned row always actually contains what was asked for.
// An empty query matches nothing, so a bare invocation never dumps the store.
func MatchesAllTokens(text, query string) bool {
	tokens := QueryTokens(query)
	if len(tokens) == 0 {
		return false
	}
	haystack := strings.ToLower(text)
	for _, token := range tokens {
		if !strings.Contains(haystack, token) {
			return false
		}
	}
	return true
}

// FTSMatchQuery renders a free-text query as an FTS5 MATCH expression:
// every token quoted and ANDed (FTS5's default operator between bare terms).
// Quoting neutralises FTS5 syntax characters a user might type. An empty
// token set returns "", which callers must treat as "match nothing" rather
// than passing to MATCH (FTS5 rejects an empty pattern).
func FTSMatchQuery(query string) string {
	tokens := QueryTokens(query)
	if len(tokens) == 0 {
		return ""
	}
	quoted := make([]string, 0, len(tokens))
	for _, token := range tokens {
		quoted = append(quoted, `"`+token+`"`)
	}
	return strings.Join(quoted, " ")
}

// Snippet trims text to at most max runes, appending an ellipsis when it had
// to cut. Used for thread-context and parent-message previews.
func Snippet(text string, max int) string {
	flat := strings.Join(strings.Fields(text), " ")
	if max <= 0 {
		return flat
	}
	runes := []rune(flat)
	if len(runes) <= max {
		return flat
	}
	return strings.TrimSpace(string(runes[:max])) + "…"
}
