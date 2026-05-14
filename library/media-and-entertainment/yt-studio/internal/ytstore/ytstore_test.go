package ytstore

import "testing"

func TestSignificantTokens_StripsStopWordsAndShort(t *testing.T) {
	t.Parallel()
	got := significantTokens("The Best PoE2 Build Guide of 2026")
	want := map[string]bool{"best": true, "poe2": true, "build": true, "guide": true, "2026": true}
	for _, tok := range got {
		if !want[tok] {
			t.Errorf("unexpected token %q", tok)
		}
		delete(want, tok)
	}
	if len(want) > 0 {
		t.Errorf("missing tokens: %v", want)
	}
}

func TestCoveredBy_NeedsTwoTokenOverlap(t *testing.T) {
	t.Parallel()
	corpus := []string{"Poe2 Atlas Guide"}

	// Two tokens overlap → covered.
	if !coveredBy("PoE2 Atlas Strategy Update", corpus) {
		t.Errorf("expected two-token overlap to count as covered")
	}
	// Single token overlap → not covered.
	if coveredBy("Poe2 Trade Tips", corpus) {
		t.Errorf("single-token overlap should not be covered")
	}
	// No overlap → not covered.
	if coveredBy("Last Epoch Falconer Build", corpus) {
		t.Errorf("no overlap should not be covered")
	}
}

func TestCoveredBy_EmptyTitleNotCovered(t *testing.T) {
	t.Parallel()
	if coveredBy("", []string{"anything"}) {
		t.Errorf("empty title should not be covered")
	}
}
