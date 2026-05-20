package cli

import "testing"

func TestPercentile(t *testing.T) {
	sorted := []float64{100, 120, 150, 180, 200}
	cases := []struct {
		p    float64
		want float64
	}{
		{0.0, 100},
		{0.5, 150},
		{1.0, 200},
	}
	for _, c := range cases {
		if got := percentile(sorted, c.p); got != c.want {
			t.Errorf("percentile(%v) = %v, want %v", c.p, got, c.want)
		}
	}
	if got := percentile([]float64{42}, 0.5); got != 42 {
		t.Errorf("single-element percentile = %v, want 42", got)
	}
}

func TestClassifyPrice(t *testing.T) {
	stats := htPriceStats{Low: 100, High: 200, P25: 120, Median: 150, P75: 180}
	cases := []struct {
		current float64
		want    string
	}{
		{110, "cheap"},     // below p25
		{120, "cheap"},     // at p25
		{150, "typical"},   // median
		{180, "expensive"}, // at p75
		{195, "expensive"}, // above p75
	}
	for _, c := range cases {
		if got := classifyPrice(c.current, stats); got != c.want {
			t.Errorf("classifyPrice(%v) = %q, want %q", c.current, got, c.want)
		}
	}

	// No spread (every observation equal) -> always typical, never cheap/expensive.
	flat := htPriceStats{Low: 188, High: 188, P25: 188, Median: 188, P75: 188}
	if got := classifyPrice(188, flat); got != "typical" {
		t.Errorf("classifyPrice on no-spread distribution = %q, want typical", got)
	}
}
