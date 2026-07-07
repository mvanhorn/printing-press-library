// internal/resolve/score.go
package resolve

// Score combines token-set Jaccard with normalized edit-distance similarity.
// Returns a value in [0,1]; 1 == identical after normalization.
func Score(a, b string) float64 {
	na, nb := Normalize(a), Normalize(b)
	if na == nb {
		return 1
	}
	jac := jaccard(Tokens(a), Tokens(b))
	lev := 1 - float64(levenshtein(na, nb))/float64(max(len(na), len(nb)))
	if lev < 0 {
		lev = 0
	}
	return 0.6*jac + 0.4*lev
}

func jaccard(a, b []string) float64 {
	set := make(map[string]bool, len(a))
	for _, t := range a {
		set[t] = true
	}
	var inter int
	seen := make(map[string]bool)
	for _, t := range b {
		if set[t] && !seen[t] {
			inter++
			seen[t] = true
		}
	}
	for _, t := range b {
		set[t] = true // grow set into the union A ∪ B
	}
	union := len(set)
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur := make([]int, len(rb)+1)
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, min(cur[j-1]+1, prev[j-1]+cost))
		}
		prev = cur
	}
	return prev[len(rb)]
}
