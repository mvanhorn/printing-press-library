package scengine

import (
	"sort"
	"strings"
)

// ReconstructAbstract rebuilds plain abstract text from OpenAlex's
// abstract_inverted_index ({word: [positions...]}). Returns "" when the index
// is empty. Capped to maxWords to bound memory on pathological inputs.
func ReconstructAbstract(inv map[string][]int) string {
	return ReconstructAbstractN(inv, 6000)
}

// ReconstructAbstractN is ReconstructAbstract with an explicit word cap.
func ReconstructAbstractN(inv map[string][]int, maxWords int) string {
	if len(inv) == 0 {
		return ""
	}
	type wp struct {
		word string
		pos  int
	}
	positions := make([]wp, 0, len(inv))
	for word, posList := range inv {
		for _, p := range posList {
			positions = append(positions, wp{word: word, pos: p})
		}
	}
	sort.Slice(positions, func(i, j int) bool { return positions[i].pos < positions[j].pos })
	if maxWords > 0 && len(positions) > maxWords {
		positions = positions[:maxWords]
	}
	words := make([]string, len(positions))
	for i, p := range positions {
		words[i] = p.word
	}
	return strings.Join(words, " ")
}
