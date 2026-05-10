// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package opentable

import "testing"

func TestReadThrottleRate_Default(t *testing.T) {
	t.Setenv("TRG_OT_THROTTLE_RATE", "")
	if r := readThrottleRate(); r != throttleRateDefault {
		t.Errorf("expected default %.2f, got %.4f", throttleRateDefault, r)
	}
}

func TestReadThrottleRate_ValidOverride(t *testing.T) {
	cases := map[string]float64{
		"0.1":  0.1,
		"1.0":  1.0,
		"2.5":  2.5,
		"0.01": 0.01,
		"5.0":  5.0,
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			t.Setenv("TRG_OT_THROTTLE_RATE", in)
			if r := readThrottleRate(); r != want {
				t.Errorf("for %q expected %f, got %f", in, want, r)
			}
		})
	}
}

func TestReadThrottleRate_OutOfRangeFallsBack(t *testing.T) {
	cases := []string{"0", "-1", "10", "0.001", "abc", "Inf"}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			t.Setenv("TRG_OT_THROTTLE_RATE", c)
			if r := readThrottleRate(); r != throttleRateDefault {
				t.Errorf("expected default for %q, got %.4f", c, r)
			}
		})
	}
}
