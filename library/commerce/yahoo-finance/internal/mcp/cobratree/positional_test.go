package cobratree

import (
	"reflect"
	"testing"
)

func TestParsePositionalArgNames(t *testing.T) {
	cases := []struct {
		name string
		use  string
		want []string
	}{
		// Normal fixed-positional commands.
		{"single positional", "sparkline <symbol>", []string{"symbol"}},
		{"two positionals", "fx <from> <to>", []string{"from", "to"}},
		{"positional with flags section", "search <query> [flags]", []string{"query"}},

		// No positionals.
		{"no positionals", "health [flags]", nil},
		{"no angle brackets", "version", nil},

		// Variadic: repeated name → nil.
		{"repeated name", "compare <symbol> <symbol>", nil},
		{"repeated name with ellipsis token", "compare <symbol> <symbol> [...]", nil},

		// Variadic: name ends in "..." → nil.
		{"ellipsis name", "gather <file...>", nil},
		{"ellipsis name mixed", "send <dest> <file...>", nil},

		// Variadic: explicit "[...]" token → nil.
		{"explicit ellipsis token", "cmd <a> <b> [...]", nil},
		{"explicit ellipsis unique names", "fetch <symbol> [...]", nil},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := parsePositionalArgNames(tc.use)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("parsePositionalArgNames(%q) = %v, want %v", tc.use, got, tc.want)
			}
		})
	}
}
