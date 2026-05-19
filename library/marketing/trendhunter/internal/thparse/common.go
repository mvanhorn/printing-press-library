package thparse

import (
	"html"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

var tagRE = regexp.MustCompile(`(?s)<[^>]+>`)
var wsRE = regexp.MustCompile(`\s+`)

func cleanText(s string) string {
	s = html.UnescapeString(s)
	s = tagRE.ReplaceAllString(s, " ")
	s = wsRE.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// truncateRunes returns s capped at max runes (not bytes). html.UnescapeString
// can introduce multi-byte UTF-8 sequences, so a byte-level slice can split
// a code point and leave a U+FFFD replacement glyph at the boundary.
func truncateRunes(s string, max int) string {
	if max <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

func collapse(s string) string {
	return strings.TrimSpace(wsRE.ReplaceAllString(html.UnescapeString(s), " "))
}

func attrValue(tag, name string) string {
	// PATCH: Go regexp does not support backreferences; accept either quote.
	re := regexp.MustCompile(`(?is)\b` + regexp.QuoteMeta(name) + `\s*=\s*["']([^"']*)["']`)
	m := re.FindStringSubmatch(tag)
	if len(m) < 2 {
		return ""
	}
	return html.UnescapeString(m[1])
}

func slugFromURL(raw string) string {
	u, err := url.Parse(html.UnescapeString(raw))
	if err != nil {
		return ""
	}
	path := strings.Trim(u.Path, "/")
	if path == "" {
		return ""
	}
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

func trendSlugFromURL(raw string) string {
	u, err := url.Parse(html.UnescapeString(raw))
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] == "trends" {
			return parts[i+1]
		}
	}
	return ""
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
