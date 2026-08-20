package airbnb

import (
	"testing"
	"time"
)

func TestJitterBounds(t *testing.T) {
	cases := []time.Duration{
		1 * time.Second,
		5 * time.Second,
		30 * time.Second,
	}
	for _, base := range cases {
		for i := 0; i < 1000; i++ {
			j := jitter(base)
			if j < 0 || j >= base/4 {
				t.Fatalf("jitter(%v) = %v, want [0, %v)", base, j, base/4)
			}
		}
	}
}

func TestJitterZeroBase(t *testing.T) {
	if j := jitter(0); j != 0 {
		t.Errorf("jitter(0) = %v, want 0", j)
	}
	if j := jitter(2 * time.Nanosecond); j != 0 {
		t.Errorf("jitter(2ns) = %v, want 0", j)
	}
}
