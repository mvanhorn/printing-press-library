package cli

import (
	"math"
	"testing"
)

func TestMatchGlob(t *testing.T) {
	cases := []struct {
		pattern, s string
		want       bool
	}{
		{"*", "EURUSD.s", true},
		{"", "EURUSD.s", true},
		{"EUR*", "EURUSD.s", true},
		{"EUR*", "GBPUSD.s", false},
		{"*USD*", "EURUSD.s", true},
		{"*USD*", "EURGBP.s", false},
		{"XAU*", "XAUUSD.s", true},
		{"EURUSD.s", "EURUSD.s", true},
		{"EURUSD.s", "EURUSD", false},
		{"*.s", "EURUSD.s", true},
		{"*.s", "EURUSD", false},
	}
	for _, c := range cases {
		if got := matchGlob(c.pattern, c.s); got != c.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", c.pattern, c.s, got, c.want)
		}
	}
}

func TestSimpleSMA(t *testing.T) {
	// SMA(3) of [1,2,3,4,5,6] = [NaN, NaN, 2, 3, 4, 5]
	got := simpleSMA([]float64{1, 2, 3, 4, 5, 6}, 3)
	want := []float64{math.NaN(), math.NaN(), 2, 3, 4, 5}
	for i := range want {
		if math.IsNaN(want[i]) {
			if !math.IsNaN(got[i]) {
				t.Errorf("simpleSMA[%d] = %v, want NaN", i, got[i])
			}
			continue
		}
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Errorf("simpleSMA[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestSimpleSMAEdgeCases(t *testing.T) {
	// period > len → all NaN
	got := simpleSMA([]float64{1, 2}, 5)
	for i, v := range got {
		if !math.IsNaN(v) {
			t.Errorf("simpleSMA[%d] = %v, want NaN (period > len)", i, v)
		}
	}
	// period 0 → all NaN
	got = simpleSMA([]float64{1, 2, 3}, 0)
	for i, v := range got {
		if !math.IsNaN(v) {
			t.Errorf("simpleSMA[%d] = %v, want NaN (period=0)", i, v)
		}
	}
}

func TestWilderRSI_AllGains(t *testing.T) {
	// Strictly increasing series: avg_loss = 0, RSI = 100 from period+1 onward.
	closes := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	got := wilderRSI(closes, 14)
	if !math.IsNaN(got[13]) && math.Abs(got[14]-100) > 1e-9 {
		t.Errorf("wilderRSI with all gains: got[14]=%v, want 100", got[14])
	}
	if got[14] != 100 {
		t.Errorf("wilderRSI[14] = %v, want 100", got[14])
	}
}

func TestWilderATR_KnownValue(t *testing.T) {
	// Build synthetic OHLC where TR is constant = 1.0, so ATR converges to 1.
	n := 20
	h := make([]float64, n)
	l := make([]float64, n)
	c := make([]float64, n)
	for i := 0; i < n; i++ {
		c[i] = float64(i) * 0.1
		h[i] = c[i] + 0.5
		l[i] = c[i] - 0.5
	}
	atr := wilderATR(h, l, c, 14)
	// After enough bars beyond period, ATR should be near 1.0 (the constant TR width).
	if math.IsNaN(atr[15]) {
		t.Fatal("ATR[15] is NaN")
	}
	if math.Abs(atr[19]-1.0) > 0.2 {
		t.Errorf("ATR did not converge: ATR[19] = %v, want ~1.0", atr[19])
	}
}

func TestRollingStd(t *testing.T) {
	// Constant series → std 0.
	got := rollingStd([]float64{5, 5, 5, 5, 5}, 3)
	for i := 2; i < 5; i++ {
		if got[i] != 0 {
			t.Errorf("rollingStd const series [%d] = %v, want 0", i, got[i])
		}
	}
	// First (window-1) values must be NaN.
	for i := 0; i < 2; i++ {
		if !math.IsNaN(got[i]) {
			t.Errorf("rollingStd warmup [%d] = %v, want NaN", i, got[i])
		}
	}
}

func TestParseSpeed(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		inf  bool
		bad  bool
	}{
		{"real", 1.0, false, false},
		{"", 1.0, false, false},
		{"max", 0, true, false},
		{"10x", 10, false, false},
		{"0.5x", 0.5, false, false},
		{"100x", 100, false, false},
		{"abc", 0, false, true},
		{"-1x", 0, false, true},
	}
	for _, c := range cases {
		got, err := parseSpeed(c.in)
		if c.bad {
			if err == nil {
				t.Errorf("parseSpeed(%q): expected error, got %v", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseSpeed(%q): unexpected error %v", c.in, err)
			continue
		}
		if c.inf {
			if !math.IsInf(got, +1) {
				t.Errorf("parseSpeed(%q) = %v, want +Inf", c.in, got)
			}
			continue
		}
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("parseSpeed(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestNullIfNaN(t *testing.T) {
	if nullIfNaN(math.NaN()) != nil {
		t.Error("nullIfNaN(NaN) should be nil")
	}
	if nullIfNaN(math.Inf(+1)) != nil {
		t.Error("nullIfNaN(+Inf) should be nil")
	}
	if v := nullIfNaN(1.5); v != 1.5 {
		t.Errorf("nullIfNaN(1.5) = %v, want 1.5", v)
	}
}

func TestSanitizeFilename(t *testing.T) {
	cases := map[string]string{
		"EURUSD.s": "EURUSD.s",
		"foo/bar":  "foo_bar",
		"foo\\bar": "foo_bar",
		"a:b*c?d":  "a_b_c_d",
		"<x>|y\"z": "_x__y_z",
	}
	for in, want := range cases {
		if got := sanitizeFilename(in); got != want {
			t.Errorf("sanitize(%q) = %q, want %q", in, got, want)
		}
	}
}
