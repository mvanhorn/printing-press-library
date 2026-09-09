// Copyright 2026 Paul Siola and contributors. Licensed under Apache-2.0. See LICENSE.

package seekparse

import (
	"math"
	"testing"
)

func TestParseSalary(t *testing.T) {
	cases := []struct {
		in             string
		wantOk         bool
		wantMinAtLeast float64
		wantMaxAtMost  float64
	}{
		{"$120,000 – $140,000", true, 110000, 150000},
		{"$120k - $140k", true, 110000, 150000},
		{"120000 - 140000", true, 110000, 150000},
		{"Up to $90,000 + super", true, 85000, 95000},
		{"$55 per hour", true, 100000, 120000},
		{"$800 per day", true, 190000, 220000},
		{"$9,000 per month", true, 100000, 115000},
		{"Competitive salary", false, 0, 0},
		{"Attractive package + car", false, 0, 0},
		{"", false, 0, 0},
	}
	for _, c := range cases {
		got := ParseSalary(c.in)
		if got.Ok != c.wantOk {
			t.Errorf("ParseSalary(%q).Ok = %v, want %v (%+v)", c.in, got.Ok, c.wantOk, got)
			continue
		}
		if !c.wantOk {
			continue
		}
		if got.Min < c.wantMinAtLeast || got.Max > c.wantMaxAtMost {
			t.Errorf("ParseSalary(%q) = min %.0f max %.0f, want min>=%.0f max<=%.0f",
				c.in, got.Min, got.Max, c.wantMinAtLeast, c.wantMaxAtMost)
		}
		if got.Period != "year" {
			t.Errorf("ParseSalary(%q).Period = %q, want annualised", c.in, got.Period)
		}
	}
}

func TestPercentile(t *testing.T) {
	vals := []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	if got := Percentile(vals, 50); math.Abs(got-55) > 0.001 {
		t.Errorf("Percentile p50 = %v, want 55", got)
	}
	if got := Percentile(vals, 0); got != 10 {
		t.Errorf("Percentile p0 = %v, want 10", got)
	}
	if got := Percentile(vals, 100); got != 100 {
		t.Errorf("Percentile p100 = %v, want 100", got)
	}
	if got := Percentile(nil, 50); got != 0 {
		t.Errorf("Percentile(nil) = %v, want 0", got)
	}
}

func TestHistogram(t *testing.T) {
	bounds, counts := Histogram([]float64{0, 10, 20, 30, 40}, 5)
	if len(bounds) != 5 || len(counts) != 5 {
		t.Fatalf("Histogram len = %d/%d, want 5/5", len(bounds), len(counts))
	}
	total := 0
	for _, c := range counts {
		total += c
	}
	if total != 5 {
		t.Errorf("Histogram counts sum = %d, want 5", total)
	}
}

func TestWinsorize(t *testing.T) {
	// 20 tight values plus two extremes; p10/p90 clip must pull the extremes in.
	in := []float64{1}
	for i := 0; i < 20; i++ {
		in = append(in, 40+float64(i))
	}
	in = append(in, 100000)
	out := Winsorize(in, 10, 90)
	if len(out) != len(in) {
		t.Fatalf("Winsorize changed length: %d != %d", len(out), len(in))
	}
	max, min := out[0], out[0]
	for _, v := range out {
		if v > max {
			max = v
		}
		if v < min {
			min = v
		}
	}
	if max > 100 || min < 30 {
		t.Errorf("Winsorize left an extreme: min %v max %v", min, max)
	}
	// small samples pass through untouched
	small := []float64{1, 999}
	if got := Winsorize(small, 5, 95); len(got) != 2 || got[1] != 999 {
		t.Errorf("Winsorize mangled a small sample: %v", got)
	}
}

func TestParseSavedSearchQuery(t *testing.T) {
	cases := []struct {
		in        string
		wantKw    string
		wantWhere string
	}{
		{"?keywords=registered nurse&where=Melbourne VIC", "registered nurse", "Melbourne VIC"},
		{"keywords=developer&where=Sydney&classification=6281", "developer", "Sydney"},
		{"/software-engineer-jobs/in-Sydney-NSW", "software engineer", "Sydney NSW"},
		{"", "", ""},
		{"totally unknown shape", "", ""},
	}
	for _, c := range cases {
		got := ParseSavedSearchQuery(c.in)
		if got.Keywords != c.wantKw || got.Where != c.wantWhere {
			t.Errorf("ParseSavedSearchQuery(%q) = kw %q where %q, want kw %q where %q",
				c.in, got.Keywords, got.Where, c.wantKw, c.wantWhere)
		}
	}
	if got := ParseSavedSearchQuery("keywords=x&classification=6281&worktype=242"); got.Classification != "6281" || got.Worktype != "242" {
		t.Errorf("ParseSavedSearchQuery lost filters: %+v", got)
	}
}
