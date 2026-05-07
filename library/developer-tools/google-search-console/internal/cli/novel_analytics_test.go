package cli

import (
	"math"
	"testing"
)

func TestParseWindow(t *testing.T) {
	cases := []struct {
		spec string
		want int
	}{
		{"7d", 7},
		{"12w", 84},
		{"3m", 90},
		{"", 99}, // fallback
		{"garbage", 99},
	}
	for _, c := range cases {
		if got := parseWindow(c.spec, 99); got != c.want {
			t.Errorf("parseWindow(%q)=%d, want %d", c.spec, got, c.want)
		}
	}
}

func TestLinearFit_DownwardSlope(t *testing.T) {
	// y = 100 - 5x for x in [0..9]
	xs := []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	ys := []float64{100, 95, 90, 85, 80, 75, 70, 65, 60, 55}
	slope, intercept := linearFit(xs, ys)
	if math.Abs(slope-(-5)) > 1e-9 {
		t.Errorf("slope=%v, want -5", slope)
	}
	if math.Abs(intercept-100) > 1e-9 {
		t.Errorf("intercept=%v, want 100", intercept)
	}
}

func TestLinearFit_FlatSeries(t *testing.T) {
	xs := []float64{0, 1, 2, 3, 4}
	ys := []float64{10, 10, 10, 10, 10}
	slope, intercept := linearFit(xs, ys)
	if slope != 0 {
		t.Errorf("flat slope=%v, want 0", slope)
	}
	if intercept != 10 {
		t.Errorf("flat intercept=%v, want 10", intercept)
	}
}

func TestSplitNonEmpty(t *testing.T) {
	got := splitNonEmpty("a||b|", "|")
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("splitNonEmpty=%v", got)
	}
	if splitNonEmpty("", "|") != nil {
		t.Errorf("empty input should be nil")
	}
}

func TestBuildGroupKey(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"country", "country", false},
		{"device", "device", false},
		{"country,device", "country || ':' || device", false},
		{"device,country", "country || ':' || device", false},
		{"page", "", true},
	}
	for _, c := range cases {
		got, err := buildGroupKey(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("buildGroupKey(%q) err=%v wantErr=%v", c.in, err, c.wantErr)
		}
		if !c.wantErr && got != c.want {
			t.Errorf("buildGroupKey(%q)=%q, want %q", c.in, got, c.want)
		}
	}
}

func TestSanitizeFloat(t *testing.T) {
	if sanitizeFloat(math.Inf(1)) != "+Inf" {
		t.Error("+Inf not stringified")
	}
	if sanitizeFloat(math.NaN()) != nil {
		t.Error("NaN should be nil for JSON safety")
	}
	v, ok := sanitizeFloat(1.23456).(float64)
	if !ok || v != 1.235 {
		t.Errorf("round3 wrong: got %v", sanitizeFloat(1.23456))
	}
}
