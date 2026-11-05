package auth

import "testing"

func TestNormalizeHost(t *testing.T) {
	ok := map[string]string{
		"":                          DefaultHost,
		"www.instacart.ca":          "www.instacart.ca",
		"https://www.instacart.ca/": "www.instacart.ca",
		" WWW.Instacart.COM ":       "www.instacart.com",
		"instacart.ca":              "instacart.ca",
	}
	for in, want := range ok {
		got, err := NormalizeHost(in)
		if err != nil || got != want {
			t.Errorf("NormalizeHost(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"evil.com", "www.instacart.ca.evil.com", "instacart.ca/graphql", "user@www.instacart.ca", "www.instacart.ca:8443", "instacart", "www.instacart.co.uk", "shop.instacart.ca", "instacart.evil"} {
		if got, err := NormalizeHost(bad); err == nil {
			t.Errorf("NormalizeHost(%q) = %q, want error", bad, got)
		}
	}
}

func TestResolveHostPrecedence(t *testing.T) {
	t.Setenv(HostEnv, "")
	if got, _ := ResolveHost(""); got != DefaultHost {
		t.Errorf("default: got %q", got)
	}
	if got, _ := ResolveHost("www.instacart.ca"); got != "www.instacart.ca" {
		t.Errorf("config: got %q", got)
	}
	t.Setenv(HostEnv, "https://www.instacart.ca")
	if got, _ := ResolveHost("www.instacart.com"); got != "www.instacart.ca" {
		t.Errorf("env should override config: got %q", got)
	}
}

func TestHostDerivedValues(t *testing.T) {
	defer SetHost(DefaultHost)
	if BaseURL() != "https://www.instacart.com" || CookieDomain() != "instacart.com" {
		t.Fatalf("US default changed: %s %s", BaseURL(), CookieDomain())
	}
	SetHost("www.instacart.ca")
	if BaseURL() != "https://www.instacart.ca" || CookieDomain() != "instacart.ca" {
		t.Fatalf("got %s %s", BaseURL(), CookieDomain())
	}
	for d, want := range map[string]bool{".instacart.ca": true, "www.instacart.ca": true, "instacart.ca": true, ".instacart.com": false, "notinstacart.ca": false} {
		if got := cookieDomainMatches(d); got != want {
			t.Errorf("cookieDomainMatches(%q) = %v, want %v", d, got, want)
		}
	}
}
