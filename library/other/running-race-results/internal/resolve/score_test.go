// internal/resolve/score_test.go
package resolve

import "testing"

func TestScoreExactIsOne(t *testing.T) {
	if s := Score("Berlin Marathon", "berlin marathon"); s < 0.999 {
		t.Fatalf("exact match scored %f", s)
	}
}

func TestScorePartialBeatsUnrelated(t *testing.T) {
	partial := Score("berlin", "BMW Berlin Marathon")
	unrelated := Score("berlin", "TCS New York City Marathon")
	if partial <= unrelated {
		t.Fatalf("partial %f should beat unrelated %f", partial, unrelated)
	}
}
