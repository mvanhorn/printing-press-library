package cli

import "testing"

func TestCanonicalizeName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"General Admission ", "general admission"},     // trailing space
		{"  General   Admission ", "general admission"}, // internal collapse
		{"It's a Date", "it's a date"},                  // unicode -> ascii apostrophe
		{"GA  STANDARD", "ga standard"},                 // case-fold
		{"Early Birds: Must Enter by 11 pm", "early birds: must enter by 11 pm"},
		{"", ""},
	}
	for _, c := range cases {
		if got := canonicalizeName(c.in); got != c.want {
			t.Errorf("canonicalizeName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCanonicalizeCommonMisspellings(t *testing.T) {
	cases := map[string]string{
		"genral admission": "general admission", // generic English typo: genral -> general
		"  VIP   ":         "vip",               // already handled by trim+case-fold; no map entry needed
	}
	for in, want := range cases {
		if got := canonicalizeName(in); got != want {
			t.Errorf("canonicalizeName(%q)=%q want %q", in, got, want)
		}
	}
}
