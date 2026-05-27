package cli

import (
	"strings"
	"unicode"
)

// commonMisspellings maps individual lowercased tokens to their corrected form.
// This map ships in every printed CLI, so entries must be generic English typo
// fixes only — no domain vocabulary (events, coins, real estate, music, etc.).
// Keep it small; prefer safety over coverage.
var commonMisspellings = map[string]string{
	"genral": "general",
}

// canonicalizeName applies Layer-A normalization: unicode punctuation folding,
// case-folding, whitespace collapse, and token-wise common-misspelling fixes.
// Conservative by design — only merges true format/spelling variants, never
// distinct concepts.
func canonicalizeName(s string) string {
	// Fold common unicode punctuation to ASCII.
	repl := strings.NewReplacer(
		"‘", "'", "’", "'", // curly single quotes
		"“", `"`, "”", `"`, // curly double quotes
		"–", "-", "—", "-", // en/em dash
		" ", " ", // non-breaking space
	)
	s = repl.Replace(s)
	s = strings.ToLower(s)
	// Collapse all whitespace runs to single spaces and trim.
	fields := strings.FieldsFunc(s, func(r rune) bool { return unicode.IsSpace(r) })
	// Apply token-wise misspelling corrections.
	for i, tok := range fields {
		if corrected, ok := commonMisspellings[tok]; ok {
			fields[i] = corrected
		}
	}
	return strings.Join(fields, " ")
}
