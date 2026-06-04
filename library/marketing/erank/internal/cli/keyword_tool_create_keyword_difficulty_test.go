package cli

import "testing"

func TestParseKeywordDifficultyPairsPreservesExplicitZero(t *testing.T) {
	got, err := parseKeywordDifficultyPairs([]string{"dad mug=0"})
	if err != nil {
		t.Fatalf("parseKeywordDifficultyPairs() error = %v", err)
	}
	value, ok := got["dad mug"].(int)
	if !ok || value != 0 {
		t.Fatalf("dad mug = %#v, want int(0)", got["dad mug"])
	}
}

func TestParseKeywordDifficultyPairsRejectsMalformedPair(t *testing.T) {
	if _, err := parseKeywordDifficultyPairs([]string{"dad mug"}); err == nil {
		t.Fatal("parseKeywordDifficultyPairs() error = nil, want error")
	}
}
