package cli

import "testing"

func TestParseTotal(t *testing.T) {
	for in, want := range map[string]int{"597": 597, "5,97": 597, "5.97": 597, "5,97 €": 597, "1.234,50": 123450} {
		if got, ok := parseTotal(in); !ok || got != want {
			t.Fatalf("parseTotal(%q) = %d, %v", in, got, ok)
		}
	}
	for _, bad := range []string{"", "abc", "5x"} {
		if _, ok := parseTotal(bad); ok {
			t.Fatalf("parseTotal(%q) should fail", bad)
		}
	}
}
