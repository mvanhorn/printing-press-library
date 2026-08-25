package cli

import (
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/giustizia-amministrativa/internal/gaclient"
)

// searchTerms extracts the terms whose occurrences are worth counting from a
// SearchOptions. The portal's testo/all/any are word lists (AND/OR), phrase is
// a single exact locution. Words shorter than 3 runes (articles, prepositions)
// are dropped: they match everywhere and dilute the signal.
func searchTerms(opts gaclient.SearchOptions) []string {
	collect := func(s string) []string {
		var out []string
		for _, w := range strings.Fields(strings.ToLower(s)) {
			if len([]rune(w)) >= 3 {
				out = append(out, w)
			}
		}
		return out
	}
	var terms []string
	terms = append(terms, collect(opts.Testo)...)
	if opts.Phrase != "" {
		terms = append(terms, strings.ToLower(strings.TrimSpace(opts.Phrase)))
	}
	terms = append(terms, collect(opts.All)...)
	terms = append(terms, collect(opts.Any)...)
	return terms
}

// countMatches counts case-insensitive occurrences of each term in text.
// Returns the total across all terms: a ruling where "appalti" appears 12
// times scores higher than one where it appears once in an obiter.
func countMatches(text string, terms []string) int {
	if len(terms) == 0 {
		return 0
	}
	lower := strings.ToLower(text)
	total := 0
	for _, term := range terms {
		total += strings.Count(lower, term)
	}
	return total
}
