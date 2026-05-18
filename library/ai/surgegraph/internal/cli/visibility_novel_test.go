package cli

import (
	"encoding/json"
	"testing"
)

// TestCitationJSONContainsURL pins PATCH(greptile-9): the citation join in
// visibility traffic-citations must use exact-URL match, not raw substring
// match. Specifically:
//   - a short page URL must not match a longer citation URL that contains
//     it as a prefix (the most common false positive);
//   - a page URL embedded in surrounding text (search results, snippets)
//     must not match — only standalone URL string values count;
//   - trailing slashes and casing must be tolerated so trivially-different
//     spellings of the same URL still match.
func TestCitationJSONContainsURL(t *testing.T) {
	target := normalizeCitationURL("https://example.com/blog")
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "exact-match-in-citation-array", raw: `{"citations":[{"url":"https://example.com/blog"}]}`, want: true},
		{name: "prefix-of-longer-citation-url-rejected", raw: `{"citations":[{"url":"https://example.com/blog/post-a"}]}`, want: false},
		{name: "embedded-in-text-rejected", raw: `{"text":"see https://example.com/blog for details"}`, want: false},
		{name: "trailing-slash-tolerated", raw: `{"citations":[{"url":"https://example.com/blog/"}]}`, want: true},
		{name: "casing-tolerated", raw: `{"citations":[{"url":"HTTPS://EXAMPLE.COM/blog"}]}`, want: true},
		{name: "deeply-nested-match", raw: `{"data":{"results":[{"meta":{"canonical":"https://example.com/blog"}}]}}`, want: true},
		{name: "empty-raw-returns-false", raw: ``, want: false},
		{name: "invalid-json-returns-false", raw: `{not json`, want: false},
		{name: "unrelated-url-returns-false", raw: `{"citations":[{"url":"https://other.com/something"}]}`, want: false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := citationJSONContainsURL(json.RawMessage(tc.raw), target)
			if got != tc.want {
				t.Errorf("want %v, got %v (raw=%s)", tc.want, got, tc.raw)
			}
		})
	}
}

func TestCitationJSONContainsURLEdgeCases(t *testing.T) {
	// Empty target short-circuits to false even if raw contains URLs.
	if got := citationJSONContainsURL(json.RawMessage(`{"url":"https://example.com"}`), ""); got {
		t.Error("empty target should return false")
	}
	// Nil raw is treated as empty input.
	if got := citationJSONContainsURL(nil, "https://example.com"); got {
		t.Error("nil raw should return false")
	}
}

func TestNormalizeCitationURLLocal(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"  ", ""},
		{"https://example.com/a", "https://example.com/a"},
		{"https://example.com/a/", "https://example.com/a"},
		{"  https://Example.COM/A  ", "https://example.com/a"},
	}
	for _, tc := range cases {
		got := normalizeCitationURL(tc.in)
		if got != tc.want {
			t.Errorf("normalizeCitationURL(%q): want %q, got %q", tc.in, tc.want, got)
		}
	}
}
