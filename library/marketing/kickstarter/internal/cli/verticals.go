// Copyright 2026 usc-tk. Licensed under Apache-2.0. See LICENSE.

// Shared vertical-keyword map and scoring helpers used by both
// `latest-news --vertical` and the standalone `vertical` command. Kept in
// one file so the two surfaces can't drift on what "ai-agents" means.

package cli

import (
	"sort"
	"strings"
)

// verticalKeywords is the curated keyword set used to score a project
// against a vertical. The slugs are stable contract — clients tracking
// "ai-agents" should keep finding ai-agent projects across CLI versions
// even if we add or rebalance the keyword list within.
//
// Lowercase only. Matched as case-insensitive substrings against the
// concatenation of project.name, project.blurb, and project.tags.
var verticalKeywords = map[string][]string{
	"ai-agents": {
		"ai agent", "autonomous agent", "llm", "gpt", "claude", "agentic",
		"copilot", "ai assistant",
	},
	"frontier-ai": {
		"ai", "artificial intelligence", "machine learning", "neural",
		"deep learning", "transformer", "foundation model", "frontier",
	},
	"smb-saas": {
		"saas", "small business", "smb", "subscription", "platform",
		"dashboard", "automation",
	},
	"geopolitics": {
		"defense", "geopolitical", "sovereign", "national security",
		"supply chain", "trade",
	},
	"aus-tech": {
		"australia", "australian", "sydney", "melbourne", "brisbane", "perth",
	},
	"india-tech": {
		"india", "indian", "bangalore", "mumbai", "delhi", "hyderabad", "chennai",
	},
}

// knownVerticals returns the supported vertical slugs in stable order
// for error messages and `vertical list` style affordances.
func knownVerticals() []string {
	out := make([]string, 0, len(verticalKeywords))
	for k := range verticalKeywords {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// verticalHitCount returns the number of unique keywords from the named
// vertical that appear (case-insensitive substring) in the haystack. Used
// both for filtering (any match) and scoring (hits / max).
func verticalHitCount(vertical, haystack string) int {
	kws, ok := verticalKeywords[vertical]
	if !ok {
		return 0
	}
	hay := strings.ToLower(haystack)
	hits := 0
	for _, kw := range kws {
		if strings.Contains(hay, strings.ToLower(kw)) {
			hits++
		}
	}
	return hits
}

// verticalScore returns a 0..1 score for a project's match against a
// vertical. Score is hits / len(keywords), capped at 1.0. Missing
// vertical returns 0.
func verticalScore(vertical, haystack string) float64 {
	kws, ok := verticalKeywords[vertical]
	if !ok || len(kws) == 0 {
		return 0
	}
	hits := verticalHitCount(vertical, haystack)
	if hits == 0 {
		return 0
	}
	score := float64(hits) / float64(len(kws))
	if score > 1.0 {
		score = 1.0
	}
	return score
}

// matchAnyVertical reports whether the haystack matches at least one
// keyword of the named vertical. Used by latest-news --vertical filtering
// where any signal is enough to keep an item.
func matchAnyVertical(vertical, haystack string) bool {
	return verticalHitCount(vertical, haystack) > 0
}

// parseSinceFlag converts strings like "7d", "24h", "30m", "1w" into a
// number of seconds. Returns (0, false) on parse failure or empty input.
// Mirrored from sync.go's parseSinceDuration but kept here so novel
// commands don't depend on sync-only helpers.
func parseSinceFlagSeconds(s string) (int64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if len(s) < 2 {
		return 0, false
	}
	unit := s[len(s)-1]
	num := s[:len(s)-1]
	var mult int64
	switch unit {
	case 'd':
		mult = 86400
	case 'h':
		mult = 3600
	case 'm':
		mult = 60
	case 'w':
		mult = 7 * 86400
	default:
		return 0, false
	}
	var n int64
	for i := 0; i < len(num); i++ {
		c := num[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int64(c-'0')
	}
	return n * mult, true
}
