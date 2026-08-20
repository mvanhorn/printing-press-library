package cli

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestFoldICSLineKeepsUTF8Valid(t *testing.T) {
	cases := []string{
		"SUMMARY:" + strings.Repeat("é", 90),
		"SUMMARY:" + strings.Repeat("🧠", 60),
		"SUMMARY:" + strings.Repeat("a", 40) + strings.Repeat("🎯", 30),
		"SUMMARY:" + strings.Repeat("x", 200),
		"SUMMARY:short",
	}
	for _, in := range cases {
		out := foldICSLine(in)
		if !utf8.ValidString(out) {
			t.Fatalf("invalid UTF-8 produced for input starting %q", in[:20])
		}
		// Unfold per RFC 5545 and confirm round-trip.
		un := strings.ReplaceAll(out, "\r\n ", "")
		un = strings.TrimSuffix(un, "\r\n")
		if un != in {
			t.Fatalf("round-trip mismatch\n got: %q\nwant: %q", un, in)
		}
		for _, seg := range strings.Split(strings.TrimSuffix(out, "\r\n"), "\r\n") {
			if len(seg) > 75 {
				t.Fatalf("segment exceeds 75 octets: %d", len(seg))
			}
		}
	}
}
