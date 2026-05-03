package cli

import "testing"

func TestPercentile_empty(t *testing.T) {
	if got := percentile(nil, 0.5); got != 0 {
		t.Errorf("empty input should return 0, got %d", got)
	}
}

func TestPercentile_p50(t *testing.T) {
	values := []int{10, 20, 30, 40, 50}
	if got := percentile(values, 0.5); got != 30 {
		t.Errorf("p50 of {10..50}: got %d want 30", got)
	}
}

func TestPercentile_p25_p75(t *testing.T) {
	values := []int{1, 2, 3, 4, 5, 6, 7, 8}
	if got := percentile(values, 0.25); got != 2 {
		t.Errorf("p25: got %d want 2", got)
	}
	if got := percentile(values, 0.75); got != 6 {
		t.Errorf("p75: got %d want 6", got)
	}
}

func TestPercentile_unsortedInput(t *testing.T) {
	values := []int{50, 10, 40, 20, 30}
	if got := percentile(values, 0.5); got != 30 {
		t.Errorf("median of unsorted: got %d want 30", got)
	}
	// Caller's input slice should not be re-ordered (we copy in percentile).
	if values[0] != 50 {
		t.Errorf("percentile must not mutate caller's slice; got %v", values)
	}
}

func TestPercentile_singleValue(t *testing.T) {
	if got := percentile([]int{42}, 0.5); got != 42 {
		t.Errorf("single-value p50: got %d want 42", got)
	}
}
