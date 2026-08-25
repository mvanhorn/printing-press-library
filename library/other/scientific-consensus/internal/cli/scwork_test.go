package cli

import "testing"

// TestRelevantToClaim pins the relevance gate (see relevantToClaim): before a
// work is classified/scored, it must share at least one non-stopword,
// non-polarity content token with the claim in its Title OR Topic. Papers
// with no overlap are dropped so they never reach stance scoring — this is
// the gate that removes the crop-genetics false positive under a
// coffee/alertness claim.
func TestRelevantToClaim(t *testing.T) {
	tests := []struct {
		name  string
		claim string
		work  scWork
		want  bool
	}{
		{
			name:  "irrelevant crop-genetics paper is dropped",
			claim: "coffee improves alertness",
			work: scWork{
				Title: "Genomic selection improved crop yield and disease resistance",
				Topic: "Plant Genetics and Breeding",
			},
			want: false,
		},
		{
			name:  "on-topic paper is kept via title overlap",
			claim: "coffee improves alertness",
			work: scWork{
				Title: "Caffeine, coffee and alertness: a randomized crossover trial",
				Topic: "Cognitive Performance",
			},
			want: true,
		},
		{
			name:  "on-topic paper is kept via topic overlap when title is generic",
			claim: "coffee improves alertness",
			work: scWork{
				Title: "A randomized crossover trial in healthy adults",
				Topic: "Coffee and caffeine pharmacology",
			},
			want: true,
		},
		{
			name:  "polarity verb alone is not enough overlap (improved != relevant)",
			claim: "coffee improves alertness",
			work: scWork{
				Title: "Fertilizer improved maize output",
				Topic: "Agronomy",
			},
			want: false,
		},
		{
			name:  "claim with no content tokens disables the gate (keep everything)",
			claim: "improves it",
			work: scWork{
				Title: "Fertilizer improved maize output",
				Topic: "Agronomy",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := relevantToClaim(tt.claim, tt.work); got != tt.want {
				t.Errorf("relevantToClaim(%q, %q/%q) = %v, want %v",
					tt.claim, tt.work.Title, tt.work.Topic, got, tt.want)
			}
		})
	}
}

// TestFilterRelevant pins that the gate excludes works before scoring and
// does not mutate its input.
func TestFilterRelevant(t *testing.T) {
	works := []scWork{
		{Title: "Caffeine, coffee and alertness: a randomized crossover trial", Topic: "Cognitive Performance"},
		{Title: "Genomic selection improved crop yield and disease resistance", Topic: "Plant Genetics and Breeding"},
	}
	kept := filterRelevant("coffee improves alertness", works)
	if len(kept) != 1 || kept[0].Title != works[0].Title {
		t.Errorf("filterRelevant kept %d works (%+v), want only the coffee trial", len(kept), kept)
	}
	if len(works) != 2 {
		t.Errorf("input slice mutated: len = %d, want 2", len(works))
	}
}
