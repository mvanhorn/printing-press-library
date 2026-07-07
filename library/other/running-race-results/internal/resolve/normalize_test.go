// internal/resolve/normalize_test.go
package resolve

import (
	"reflect"
	"testing"
)

func TestNormalizeStripsSponsorAndPunct(t *testing.T) {
	cases := map[string]string{
		"TCS New York City Marathon": "new york city marathon",
		"BMW BERLIN-MARATHON 2025":   "berlin marathon 2025",
		"  Boston   Marathon!! ":     "boston marathon",
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Fatalf("Normalize(%q)=%q want %q", in, got, want)
		}
	}
}

func TestTokens(t *testing.T) {
	got := Tokens("Berlin Marathon")
	want := []string{"berlin", "marathon"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Tokens=%v want %v", got, want)
	}
}
