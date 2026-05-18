package cli

import "testing"

func TestIsLawHubCookieDomain(t *testing.T) {
	cases := map[string]bool{
		"lawhub.org":          true,
		".lawhub.org":         true,
		"app.lawhub.org":      true,
		"auth.lawhub.org":     true,
		"evil-lawhub.org":     false,
		"lawhub.org.evil.com": false,
		"example.com":         false,
		"":                    false,
	}
	for domain, want := range cases {
		if got := isLawHubCookieDomain(domain); got != want {
			t.Fatalf("isLawHubCookieDomain(%q)=%v want %v", domain, got, want)
		}
	}
}
