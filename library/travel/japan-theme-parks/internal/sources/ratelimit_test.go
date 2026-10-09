package sources

import "testing"

func TestNewHonorsLowerRateLimit(t *testing.T) {
	c := New(0, 0.1)
	for name, l := range c.limiters {
		if got := l.Rate(); got != 0.1 {
			t.Errorf("%s rate = %v, want 0.1", name, got)
		}
	}
}

func TestNewKeepsPoliteDefaults(t *testing.T) {
	for _, maxRate := range []float64{-1, 0, 50} {
		c := New(0, maxRate)
		if got := c.limiters["queue-times"].Rate(); got != 2.0 {
			t.Errorf("maxRate %v: queue-times rate = %v, want 2.0", maxRate, got)
		}
		if got := c.limiters["tdr"].Rate(); got != 1.0 {
			t.Errorf("maxRate %v: tdr rate = %v, want 1.0", maxRate, got)
		}
	}
}
