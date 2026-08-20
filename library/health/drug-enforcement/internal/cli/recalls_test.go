package cli

import "testing"

func TestPhrase(t *testing.T) {
	tests := []struct {
		field, term, want string
	}{
		{"product_description", "ibuprofen", `product_description:"ibuprofen"`},
		{"recalling_firm", "Teva Pharma", `recalling_firm:"Teva Pharma"`},
		{"product_description", `bad"quote`, `product_description:"badquote"`}, // embedded quotes stripped
		{"recalling_firm", "  spaced  ", `recalling_firm:"spaced"`},            // trimmed
	}
	for _, tc := range tests {
		if got := phrase(tc.field, tc.term); got != tc.want {
			t.Errorf("phrase(%q,%q)=%q want %q", tc.field, tc.term, got, tc.want)
		}
	}
}

func TestNormalizeRecallDate(t *testing.T) {
	tests := []struct{ in, want string }{
		{"20260115", "2026-01-15"},
		{"2026-01-15", "2026-01-15"}, // already ISO → passthrough
		{"", ""},
		{"bad", "bad"},
		{"20261301", "20261301"}, // invalid month → passthrough
	}
	for _, tc := range tests {
		if got := normalizeRecallDate(tc.in); got != tc.want {
			t.Errorf("normalizeRecallDate(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestClassToLabel(t *testing.T) {
	for n, want := range map[int]string{1: "Class I", 2: "Class II", 3: "Class III"} {
		if got := classToLabel[n]; got != want {
			t.Errorf("classToLabel[%d]=%q want %q", n, got, want)
		}
	}
	if _, ok := classToLabel[4]; ok {
		t.Error("class 4 should not be valid")
	}
}
