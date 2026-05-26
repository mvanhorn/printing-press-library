package safari

import "testing"

func TestEscapeLike(t *testing.T) {
	cases := map[string]string{
		"github.com": "github.com",
		"100%done":   `100\%done`,
		"a_b":        `a\_b`,
		`back\slash`: `back\\slash`,
		"%_\\":       `\%\_\\`,
	}
	for in, want := range cases {
		if got := escapeLike(in); got != want {
			t.Errorf("escapeLike(%q) = %q, want %q", in, got, want)
		}
	}
}
