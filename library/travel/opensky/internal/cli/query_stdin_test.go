package cli

import (
	"strings"
	"testing"
)

func TestQueryFromArgsOrStdin(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		flagQuery string
		stdin     string
		want      string
		wantOK    bool
	}{
		{name: "flag wins", flagQuery: "flights over jfk", stdin: "stdin ignored", want: "flights over jfk", wantOK: true},
		{name: "positional wins", args: []string{"flights", "over", "jfk"}, stdin: "stdin ignored", want: "flights over jfk", wantOK: true},
		{name: "stdin used when nothing inline", stdin: "what flies over JFK?", want: "what flies over JFK?", wantOK: true},
		{name: "stdin trimmed", stdin: "  \nwho owns aa07f9\n ", want: "who owns aa07f9", wantOK: true},
		{name: "no query at all", want: "", wantOK: false},
		{name: "empty stdin", stdin: "   ", want: "", wantOK: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := queryFromArgsOrStdin(tc.args, tc.flagQuery, strings.NewReader(tc.stdin))
			if got != tc.want || ok != tc.wantOK {
				t.Fatalf("queryFromArgsOrStdin(%v, %q) = %q,%v want %q,%v", tc.args, tc.flagQuery, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}
