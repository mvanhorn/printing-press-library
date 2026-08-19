package scengine

import "strings"

// normalizeText folds the typographic characters that scientific publishers use
// into their ASCII equivalents, so that a plain strings.Contains match behaves
// the same on "Omega-3" and "Omega-3" with a typographic hyphen.
//
// Measured on the twelve full corpora: 107 occurrences of six distinct non-ASCII
// dash characters. One of them cost a real exclusion - "Omega-3 polyunsaturated
// fatty acids and inflammatory processes" (1332 citations, supporting) uses
// U+2010 HYPHEN, a different byte sequence from the U+002D HYPHEN-MINUS in the
// claim "omega-3 improves cardiovascular health".
//
// Only dashes are folded. Accents, ligatures and quotation marks are left alone:
// they were not measured to cause misses, and folding them would be a larger
// change than the evidence supports.
//
// Both readers of a work's text call this - the PICO gate and the stance
// classifier - so the two can never disagree about what the text says.
func normalizeText(s string) string {
	return dashFolder.Replace(s)
}

// dashFolder maps the dash-like runes found in the corpora to ASCII "-".
//
// Every entry below was counted in testdata/corpora_full except U+2012, which
// is included to close the Unicode dash block; adding further speculative
// entries would widen the fold without evidence.
var dashFolder = strings.NewReplacer(
	"\u2010", "-", // HYPHEN               30 occurrences
	"\u2011", "-", // NON-BREAKING HYPHEN   5
	"\u2012", "-", // FIGURE DASH           0
	"\u2013", "-", // EN DASH              46
	"\u2014", "-", // EM DASH              15
	"\u2015", "-", // HORIZONTAL BAR        2
	"\u2212", "-", // MINUS SIGN            9
)
