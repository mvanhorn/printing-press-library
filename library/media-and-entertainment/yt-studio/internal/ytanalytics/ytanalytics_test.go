package ytanalytics

import (
	"encoding/json"
	"testing"
)

func TestToInt64_HandlesAllNumericShapes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   interface{}
		want int64
	}{
		{float64(42), 42},
		{int64(42), 42},
		{int(42), 42},
		{json.Number("42"), 42},
		{"not a number", 0},
	}
	for _, c := range cases {
		got := toInt64(c.in)
		if got != c.want {
			t.Errorf("toInt64(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestToFloat_HandlesAllNumericShapes(t *testing.T) {
	t.Parallel()
	if v, ok := toFloat(float64(0.5)); !ok || v != 0.5 {
		t.Errorf("float64: %v %v", v, ok)
	}
	if v, ok := toFloat(int64(2)); !ok || v != 2.0 {
		t.Errorf("int64: %v %v", v, ok)
	}
	if _, ok := toFloat("nope"); ok {
		t.Errorf("string should fail")
	}
	jn := json.Number("0.75")
	if v, ok := toFloat(jn); !ok || v != 0.75 {
		t.Errorf("json.Number: %v %v", v, ok)
	}
}

func TestEnsureScopes_IncludesBoth(t *testing.T) {
	t.Parallel()
	got := EnsureScopes()
	want := []string{"yt-analytics.readonly", "youtube.readonly"}
	for _, w := range want {
		if !contains(got, w) {
			t.Errorf("missing scope %q in %q", w, got)
		}
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

func TestTruncate_LeavesShortAlone(t *testing.T) {
	t.Parallel()
	if got := truncate("short", 100); got != "short" {
		t.Errorf("expected unchanged, got %q", got)
	}
	if got := truncate("aaaaaaaaaa", 5); got != "aaaaa…" {
		t.Errorf("expected truncation with ellipsis, got %q", got)
	}
}
