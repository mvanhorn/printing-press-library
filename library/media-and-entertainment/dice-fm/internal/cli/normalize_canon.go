package cli

import (
	"strings"
	"unicode"
)

// canonicalizeName applies Layer-A normalization: unicode punctuation folding,
// case-folding, and whitespace collapse. Conservative by design — it never
// edits word content (no typo "correction"), so it can only merge true
// format/spelling variants, never distinct concepts.
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
	return strings.Join(fields, " ")
}
