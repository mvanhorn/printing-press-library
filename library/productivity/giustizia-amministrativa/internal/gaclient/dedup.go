package gaclient

import (
	"fmt"
	"strings"
)

// minSnippetLen is the shortest snippet we consider meaningful enough to
// dedup on. Below this, the portal's teaser is too short to be a reliable
// fingerprint (e.g. "...OMISSIS...").
const minSnippetLen = 30

// applySnippetDedup folds near-duplicate results that share an identical,
// non-trivial snippet and records the event as a warning on the result.
//
// The portal returns one row per ruling, so near-identical snippets signal
// ricorsi gemelli — multiple parties challenging the same provvedimento with
// the same device. They add rows without adding information: four copies of
// the same obiter citation waste four result slots and, downstream, trigger
// four identical full-text fetches in massime/corpus/appeal-chain.
//
// Called once at the end of Search, so every caller (plain search, stats,
// massime, appeal-chain, corpus-build) benefits without each repeating the
// logic.
func applySnippetDedup(res *SearchResult) *SearchResult {
	if res == nil || len(res.Items) == 0 {
		return res
	}
	deduped, grouped := dedupBySnippet(res.Items)
	if grouped > 0 {
		res.Items = deduped
		res.Warnings = append(res.Warnings, fmt.Sprintf(
			"%d risultati con snippet identico (probabili ricorsi gemelli sullo stesso provvedimento) sono stati raggruppati. Il campo 'duplicati' su ciascun risultato indica quanti ne condividono lo stesso snippet",
			grouped))
	}
	return res
}

// dedupBySnippet groups results that share an identical, non-trivial snippet.
// The first item of each group (most recent, because the portal orders by
// decreasing number) is kept as the representative; the rest are dropped.
// Duplicati on the representative records the total group size so nothing
// is hidden. Returns the deduplicated slice and the number of items folded.
func dedupBySnippet(items []Provvedimento) ([]Provvedimento, int) {
	// First pass: count how many items share each non-trivial snippet.
	counts := make(map[string]int)
	for _, it := range items {
		s := normSnippet(it.Snippet)
		if len(s) >= minSnippetLen {
			counts[s]++
		}
	}

	// Second pass: keep the first occurrence of each duplicated group,
	// set Duplicati, drop the rest.
	seen := make(map[string]bool)
	var out []Provvedimento
	grouped := 0
	for _, it := range items {
		s := normSnippet(it.Snippet)
		if len(s) >= minSnippetLen && counts[s] > 1 {
			if seen[s] {
				grouped++
				continue
			}
			seen[s] = true
			it.Duplicati = counts[s]
		}
		out = append(out, it)
	}
	return out, grouped
}

// normSnippet trims and collapses whitespace so cosmetic differences in the
// portal's HTML rendering do not defeat exact matching.
func normSnippet(s string) string {
	s = strings.TrimSpace(s)
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}
