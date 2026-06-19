package cli

import "testing"

func TestBrowserCookieURLHostUsesAccountHostForPerplexity(t *testing.T) {
	if got := browserCookieURLHost(".perplexity.ai"); got != "www.perplexity.ai" {
		t.Fatalf("browserCookieURLHost(.perplexity.ai) = %q, want www.perplexity.ai", got)
	}
}

func TestBrowserCookieURLHostKeepsOtherDomains(t *testing.T) {
	if got := browserCookieURLHost(".example.com"); got != "example.com" {
		t.Fatalf("browserCookieURLHost(.example.com) = %q, want example.com", got)
	}
}
