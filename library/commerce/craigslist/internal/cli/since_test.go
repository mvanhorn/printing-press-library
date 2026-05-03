package cli

import "testing"

func TestPIDFromURL_validHTML(t *testing.T) {
	u := "https://sfbay.craigslist.org/sfc/apa/d/some-slug-here/7915891289.html"
	if got := pidFromURL(u); got != 7915891289 {
		t.Errorf("pidFromURL(%q): got %d want 7915891289", u, got)
	}
}

func TestPIDFromURL_missing(t *testing.T) {
	cases := []string{"", "https://example.com/", "not-a-url"}
	for _, c := range cases {
		if got := pidFromURL(c); got != 0 {
			t.Errorf("pidFromURL(%q): got %d want 0", c, got)
		}
	}
}

func TestPIDFromURL_noHTMLSuffix(t *testing.T) {
	if got := pidFromURL("https://x/123"); got != 123 {
		t.Errorf("trailing PID without .html should still parse, got %d", got)
	}
}
