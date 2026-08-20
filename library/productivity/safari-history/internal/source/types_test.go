package source

import "testing"

func TestNormalizeDomain(t *testing.T) {
	cases := map[string]string{
		"www.google.com":      "google.com",
		"mail.google.com":     "google.com",
		"example.com":         "example.com",
		"www.bbc.co.uk":       "bbc.co.uk",
		"news.bbc.co.uk":      "bbc.co.uk",
		"shop.abc.net.au":     "abc.net.au",
		"foo.bar.co.jp":       "bar.co.jp",
		"loja.exemplo.com.br": "exemplo.com.br",
		"localhost":           "localhost",
	}
	for in, want := range cases {
		if got := normalizeDomain(in); got != want {
			t.Errorf("normalizeDomain(%q) = %q, want %q", in, got, want)
		}
	}
}
