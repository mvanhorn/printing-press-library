package cli

import (
	"math"
	"testing"
)

func TestJaroWinkler(t *testing.T) {
	cases := []struct {
		a, b string
		min  float64 // expected >= min
		max  float64 // expected <= max (0 means no upper bound checked)
	}{
		{"final release", "final release", 0.999, 0}, // identical
		{"final release", "fina release", 0.90, 0},   // single typo
		{"last chance tickets", "last chance ticket", 0.90, 0},
		{"general admission", "roof admission", 0.0, 0.85}, // different prefix, low — must stay below clustering threshold
	}
	for _, c := range cases {
		got := jaroWinkler(c.a, c.b)
		if got < c.min || math.IsNaN(got) {
			t.Errorf("jaroWinkler(%q,%q) = %.3f, want >= %.3f", c.a, c.b, got, c.min)
		}
		if c.max > 0 && got > c.max {
			t.Errorf("jaroWinkler(%q,%q) = %.3f, want <= %.3f (should be below clustering threshold)", c.a, c.b, got, c.max)
		}
	}
}

func TestClusterNames(t *testing.T) {
	in := []string{"final release", "fina release", "general admission", "general admision"}
	clusters := clusterNames(in, 0.92)
	if len(clusters) != 2 {
		t.Fatalf("want 2 clusters, got %d: %v", len(clusters), clusters)
	}
}
