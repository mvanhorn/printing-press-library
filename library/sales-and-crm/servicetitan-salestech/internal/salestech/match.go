package salestech

import (
	"sort"
	"strings"
	"unicode"
)

// normalize lowercases s and replaces every non-alphanumeric rune with a
// single space; the result is collapsed to single spaces and trimmed. Used
// before fuzzy comparison so "Well-Pump #2" and "well pump 2" match.
func normalize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	lastSpace := true
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
			lastSpace = false
		default:
			if !lastSpace {
				b.WriteByte(' ')
				lastSpace = true
			}
		}
	}
	return strings.TrimSpace(b.String())
}

// tokens splits the normalized form into space-separated words.
func tokens(s string) []string {
	n := normalize(s)
	if n == "" {
		return nil
	}
	return strings.Fields(n)
}

// similarity returns a [0,1] score on two strings via a character bigram
// Jaccard-ish coefficient. Order-insensitive enough for "1.5 hp pump" vs
// "pump 1.5 hp" without paying the cost of full edit-distance.
func similarity(a, b string) float64 {
	an, bn := normalize(a), normalize(b)
	if an == "" || bn == "" {
		return 0
	}
	if an == bn {
		return 1
	}
	ag := bigrams(an)
	bg := bigrams(bn)
	if len(ag) == 0 || len(bg) == 0 {
		return 0
	}
	common := 0
	for k := range ag {
		if bg[k] {
			common++
		}
	}
	return 2.0 * float64(common) / float64(len(ag)+len(bg))
}

func bigrams(s string) map[string]bool {
	out := map[string]bool{}
	r := []rune(s)
	for i := 0; i+1 < len(r); i++ {
		out[string(r[i:i+2])] = true
	}
	return out
}

// tokenCoverage returns the fraction of query tokens that appear (as a
// substring of some longer token) anywhere in subject. Asymmetric — good
// for "describe the part" queries where a short phrase should match a
// longer SKU name.
func tokenCoverage(query, subject string) float64 {
	qts := tokens(query)
	sts := tokens(subject)
	if len(qts) == 0 || len(sts) == 0 {
		return 0
	}
	subjJoined := " " + strings.Join(sts, " ") + " "
	hit := 0
	for _, t := range qts {
		if strings.Contains(subjJoined, " "+t) || containsToken(sts, t) {
			hit++
		}
	}
	return float64(hit) / float64(len(qts))
}

// containsToken returns true when t is a substring of any element in toks.
func containsToken(toks []string, t string) bool {
	for _, s := range toks {
		if strings.Contains(s, t) {
			return true
		}
	}
	return false
}

// sortByScoreDesc stable-sorts a slice of {score, idx} pairs in place,
// highest score first. The caller materializes the result in-order from the
// returned indices.
func sortByScoreDesc(scores []float64) []int {
	idx := make([]int, len(scores))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(i, j int) bool {
		return scores[idx[i]] > scores[idx[j]]
	})
	return idx
}
