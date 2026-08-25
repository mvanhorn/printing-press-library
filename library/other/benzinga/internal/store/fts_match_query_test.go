// Copyright 2026 waveriderai and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import "testing"

// TestFTSMatchQueryTokenization is the MCP/CLI search-schema contract:
// ftsMatchQuery keeps [\pL\pN_]+ tokens, quotes each one, and joins them
// with spaces (FTS5 implicit AND). Operators and punctuation are not
// syntax — they are stripped or treated as literal tokens.
func TestFTSMatchQueryTokenization(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{name: "single token", in: "earnings", want: `"earnings"`},
		{name: "implicit AND", in: "earnings surprise", want: `"earnings" "surprise"`},
		{name: "NOT is a literal", in: "NOT earnings", want: `"NOT" "earnings"`},
		{name: "OR is a literal", in: "AAPL OR NVDA", want: `"AAPL" "OR" "NVDA"`},
		{name: "AND is a literal", in: "AAPL AND NVDA", want: `"AAPL" "AND" "NVDA"`},
		{name: "quoted phrase splits", in: `"earnings surprise"`, want: `"earnings" "surprise"`},
		{name: "hyphen splits", in: "foo-bar baz", want: `"foo" "bar" "baz"`},
		{name: "underscore kept", in: "ticker_id", want: `"ticker_id"`},
		{name: "punctuation only", in: "!!!", want: ""},
		{name: "whitespace only", in: "   ", want: ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ftsMatchQuery(c.in); got != c.want {
				t.Fatalf("ftsMatchQuery(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
