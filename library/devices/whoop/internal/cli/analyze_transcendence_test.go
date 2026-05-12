// Happy-path tests for the transcendence layer. We test pure helpers
// directly. The DB-backed RunE paths get exercised in dogfood/verify; here
// we just want to lock in that the math and parsing are correct so future
// regenerations don't silently break the analytics.

package cli

import (
	"math"
	"testing"
	"time"
)

func TestParseWindow(t *testing.T) {
	cases := map[string]time.Duration{
		"30d": 30 * 24 * time.Hour,
		"90d": 90 * 24 * time.Hour,
		"12w": 12 * 7 * 24 * time.Hour,
		"24h": 24 * time.Hour,
	}
	for in, want := range cases {
		got, err := parseWindow(in)
		if err != nil {
			t.Fatalf("parseWindow(%q) errored: %v", in, err)
		}
		if got != want {
			t.Errorf("parseWindow(%q) = %v, want %v", in, got, want)
		}
	}
	if _, err := parseWindow("xx"); err == nil {
		t.Error("expected error for bogus window 'xx'")
	}
}

func TestMeanStd(t *testing.T) {
	mean, std := meanStd([]float64{1, 2, 3, 4, 5})
	if math.Abs(mean-3) > 1e-9 {
		t.Errorf("mean = %v want 3", mean)
	}
	// sample stdev of {1,2,3,4,5} = sqrt(2.5) ≈ 1.5811
	if math.Abs(std-math.Sqrt(2.5)) > 1e-6 {
		t.Errorf("std = %v want %v", std, math.Sqrt(2.5))
	}
	m2, s2 := meanStd(nil)
	if m2 != 0 || s2 != 0 {
		t.Errorf("empty slice should return zeros, got %v %v", m2, s2)
	}
}

func TestPearsonPerfectPositive(t *testing.T) {
	xs := []float64{1, 2, 3, 4, 5}
	ys := []float64{2, 4, 6, 8, 10}
	r, n := pearson(xs, ys)
	if n != 5 {
		t.Errorf("n = %d want 5", n)
	}
	if math.Abs(r-1.0) > 1e-9 {
		t.Errorf("r = %v want 1.0", r)
	}
}

func TestPearsonPerfectNegative(t *testing.T) {
	xs := []float64{1, 2, 3, 4, 5}
	ys := []float64{5, 4, 3, 2, 1}
	r, _ := pearson(xs, ys)
	if math.Abs(r-(-1.0)) > 1e-9 {
		t.Errorf("r = %v want -1.0", r)
	}
}

func TestInterpretPearson(t *testing.T) {
	cases := []struct {
		r    float64
		want string
	}{
		{0.05, "no meaningful linear relationship"},
		{-0.4, "moderate negative correlation"},
		{0.85, "very strong positive correlation"},
	}
	for _, c := range cases {
		if got := interpretPearson(c.r); got != c.want {
			t.Errorf("interpretPearson(%v) = %q want %q", c.r, got, c.want)
		}
	}
}

func TestIsReadOnlyQuery(t *testing.T) {
	allowed := []string{
		"SELECT * FROM cycle",
		"  select 1",
		"WITH x AS (SELECT 1) SELECT * FROM x",
		"-- comment\nSELECT 1",
	}
	for _, q := range allowed {
		if !isReadOnlyQuery(q) {
			t.Errorf("expected allowed: %q", q)
		}
	}
	rejected := []string{
		"INSERT INTO cycle VALUES (1)",
		"UPDATE cycle SET strain = 10",
		"DROP TABLE cycle",
		"PRAGMA writable_schema = 1",
		"WITH x AS (SELECT 1) INSERT INTO cycle VALUES (1)",
		"DELETE FROM cycle",
	}
	for _, q := range rejected {
		if isReadOnlyQuery(q) {
			t.Errorf("expected rejected: %q", q)
		}
	}
}

func TestSameDayUTC(t *testing.T) {
	a := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	b := time.Date(2026, 5, 11, 23, 59, 0, 0, time.UTC)
	c := time.Date(2026, 5, 12, 0, 1, 0, 0, time.UTC)
	if !sameDayUTC(a, b) {
		t.Error("a and b should be same day UTC")
	}
	if sameDayUTC(a, c) {
		t.Error("a and c are different days")
	}
}

func TestLinRegSlope(t *testing.T) {
	xs := []float64{0, 1, 2, 3, 4}
	ys := []float64{1, 3, 5, 7, 9} // y = 2x + 1
	slope := linRegSlope(xs, ys)
	if math.Abs(slope-2) > 1e-9 {
		t.Errorf("slope = %v want 2.0", slope)
	}
}

func TestAnyToFloatInt64(t *testing.T) {
	if v := anyToFloat(float64(3.14)); v != 3.14 {
		t.Errorf("anyToFloat float64 = %v", v)
	}
	if v := anyToFloat("2.5"); v != 2.5 {
		t.Errorf("anyToFloat string = %v", v)
	}
	if v := anyToInt64(int64(42)); v != 42 {
		t.Errorf("anyToInt64 int64 = %v", v)
	}
	if v := anyToInt64("17"); v != 17 {
		t.Errorf("anyToInt64 string = %v", v)
	}
}
