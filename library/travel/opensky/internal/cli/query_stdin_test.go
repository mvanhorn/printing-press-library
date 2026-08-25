package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQueryFromArgsOrStdin(t *testing.T) {
	tmp := t.TempDir()
	qf := filepath.Join(tmp, "q.txt")
	if err := os.WriteFile(qf, []byte("what flies over JFK?"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name      string
		args      []string
		flagQuery string
		queryFile string
		stdin     string
		want      string
		wantOK    bool
	}{
		{name: "flag wins", flagQuery: "flights over jfk", queryFile: qf, stdin: "stdin ignored", want: "flights over jfk", wantOK: true},
		{name: "file beats positional", args: []string{"pos", "args"}, queryFile: qf, stdin: "stdin ignored", want: "what flies over JFK?", wantOK: true},
		{name: "positional beats stdin", args: []string{"flights", "over", "jfk"}, stdin: "stdin ignored", want: "flights over jfk", wantOK: true},
		{name: "stdin used when nothing inline", stdin: "who owns aa07f9", want: "who owns aa07f9", wantOK: true},
		{name: "stdin trimmed", stdin: "  \nwho owns aa07f9\n ", want: "who owns aa07f9", wantOK: true},
		{name: "no query at all", want: "", wantOK: false},
		{name: "empty stdin", stdin: "   ", want: "", wantOK: false},
		{name: "missing query file", queryFile: filepath.Join(tmp, "nope.txt"), want: "", wantOK: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := queryFromArgsOrStdin(tc.args, tc.flagQuery, tc.queryFile, strings.NewReader(tc.stdin))
			if got != tc.want || ok != tc.wantOK {
				t.Fatalf("queryFromArgsOrStdin(%v, %q, %q) = %q,%v want %q,%v", tc.args, tc.flagQuery, tc.queryFile, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}
