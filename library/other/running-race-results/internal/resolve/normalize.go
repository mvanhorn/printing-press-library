// internal/resolve/normalize.go
package resolve

import (
	"strings"
	"unicode"
)

// sponsorPrefixes are title sponsors stripped before matching.
var sponsorPrefixes = map[string]bool{
	"tcs": true, "bmw": true, "bank": true, "of": true, "america": true,
	"virgin": true, "money": true, "abbott": true,
}

// Normalize lowercases, removes punctuation, drops leading sponsor tokens,
// and collapses whitespace.
func Normalize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		case unicode.IsSpace(r) || unicode.IsPunct(r):
			b.WriteRune(' ')
		}
	}
	fields := strings.Fields(b.String())
	// Drop leading sponsor words only (not interior).
	for len(fields) > 0 && sponsorPrefixes[fields[0]] {
		fields = fields[1:]
	}
	return strings.Join(fields, " ")
}

// Tokens returns the normalized whitespace-split tokens.
func Tokens(s string) []string {
	return strings.Fields(Normalize(s))
}
