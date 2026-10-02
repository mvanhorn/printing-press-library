package traveloka

import (
	"math"
	"testing"
)

func TestSimulatedExplicitRateLimitOptions(t *testing.T) {
	for _, tt := range []struct {
		name     string
		rate     float64
		bad      bool
		disabled bool
	}{{"auto", -1, false, false}, {"disabled", 0, false, true}, {"ceiling", 0.2, false, false}, {"invalid negative", -0.5, true, false}, {"NaN", math.NaN(), true, false}, {"infinite", math.Inf(1), true, false}} {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{}
			e := c.SetRateLimit(tt.rate)
			if (e != nil) != tt.bad {
				t.Fatalf("invalid rate not distinguished: %v", e)
			}
			if e == nil && (c.limiter == nil) != tt.disabled {
				t.Fatal("pacing enable/disable was ignored")
			}
		})
	}
}
