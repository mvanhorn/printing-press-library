package pricebook

import (
	"strings"
	"unicode"
)

// Normalize lower-cases a string and reduces every run of non-alphanumeric
// characters to a single space, then trims. It is the shared canonical form
// for fuzzy matching part numbers, SKU codes, and display names — so
// "F-1921/000", "f1921 000", and "F1921000" all compare equal-ish.
func Normalize(s string) string {
	var b strings.Builder
	lastSpace := true // suppress leading space
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastSpace = false
		} else if !lastSpace {
			b.WriteByte(' ')
			lastSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

// NormalizeTight is Normalize with all spaces removed — the form used to
// compare part numbers, where internal punctuation is noise but token
// boundaries are not meaningful ("F1921000" vs "F-1921-000").
func NormalizeTight(s string) string {
	return strings.ReplaceAll(Normalize(s), " ", "")
}

// Tokens returns the unique normalized word tokens of s.
func Tokens(s string) []string {
	fields := strings.Fields(Normalize(s))
	seen := make(map[string]struct{}, len(fields))
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if _, ok := seen[f]; ok {
			continue
		}
		seen[f] = struct{}{}
		out = append(out, f)
	}
	return out
}

// TokenCoverage is the asymmetric overlap of query against target in [0,1]:
// the fraction of query's tokens that also appear in target. Unlike Jaccard
// it does not penalize target for having extra words — "softener 30k" fully
// covers against "30k Grain Softener, Master Water". This is the right
// scorer for the natural-language part finder, where the query is a short
// description and the candidate SKU name is longer. An empty query covers
// nothing (returns 0) so it never inflates a find score.
func TokenCoverage(query, target string) float64 {
	tq, tt := Tokens(query), Tokens(target)
	if len(tq) == 0 {
		return 0
	}
	set := make(map[string]struct{}, len(tt))
	for _, t := range tt {
		set[t] = struct{}{}
	}
	hit := 0
	for _, t := range tq {
		if _, ok := set[t]; ok {
			hit++
		}
	}
	return float64(hit) / float64(len(tq))
}

// Jaccard returns the token-set Jaccard similarity of a and b in [0,1]:
// |intersection| / |union|. Two empty strings are defined as similarity 1
// (both describe "nothing"); one empty and one non-empty is 0.
func Jaccard(a, b string) float64 {
	ta, tb := Tokens(a), Tokens(b)
	if len(ta) == 0 && len(tb) == 0 {
		return 1
	}
	if len(ta) == 0 || len(tb) == 0 {
		return 0
	}
	set := make(map[string]struct{}, len(ta))
	for _, t := range ta {
		set[t] = struct{}{}
	}
	inter := 0
	for _, t := range tb {
		if _, ok := set[t]; ok {
			inter++
		}
	}
	union := len(ta) + len(tb) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// Levenshtein returns the edit distance between a and b.
func Levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	if len(ra) == 0 {
		return len(rb)
	}
	if len(rb) == 0 {
		return len(ra)
	}
	prev := make([]int, len(rb)+1)
	curr := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		curr[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			curr[j] = min3(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[len(rb)]
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

// LevenshteinRatio returns 1 - dist/maxlen in [0,1]: 1.0 for identical
// strings, 0.0 for total mismatch. Both empty is 1.
func LevenshteinRatio(a, b string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1
	}
	maxLen := len([]rune(a))
	if l := len([]rune(b)); l > maxLen {
		maxLen = l
	}
	if maxLen == 0 {
		return 1
	}
	return 1 - float64(Levenshtein(a, b))/float64(maxLen)
}

// Similarity is the general-purpose fuzzy score in [0,1] for comparing two
// SKU-ish strings (codes, names, descriptions). It takes the best of:
//   - token-set Jaccard (handles word reordering and extra words)
//   - Levenshtein ratio of the normalized forms (handles typos)
//   - a containment boost (one normalized string fully inside the other)
//
// so "Softener 30k" scores high against "30k Grain Softener, Master Water".
func Similarity(a, b string) float64 {
	na, nb := Normalize(a), Normalize(b)
	if na == "" && nb == "" {
		return 1
	}
	if na == "" || nb == "" {
		return 0
	}
	score := Jaccard(a, b)
	if lr := LevenshteinRatio(na, nb); lr > score {
		score = lr
	}
	if na != nb && (strings.Contains(na, nb) || strings.Contains(nb, na)) {
		if 0.9 > score {
			score = 0.9
		}
	}
	return score
}

// PartMatch reports whether two vendor part numbers refer to the same part.
// Vendor part numbers are punctuation-noisy ("MPCLR30TE1" vs "MPCLR30T-E1"),
// so the comparison is on the tight-normalized form. Empty strings never
// match anything — a missing part number is not a match, it is a gap.
func PartMatch(a, b string) bool {
	ta, tb := NormalizeTight(a), NormalizeTight(b)
	if ta == "" || tb == "" {
		return false
	}
	return ta == tb
}
