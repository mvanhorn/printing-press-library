package cli

import (
	"errors"
	"testing"
)

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

func TestRefreshBrowserCookieSourcePrefersDirectExtractor(t *testing.T) {
	liveCalled := false
	cookies, err := refreshBrowserCookieSource(
		".perplexity.ai",
		func() (cookieTool, error) { return cookieTool{name: "pycookiecheat"}, nil },
		func(tool cookieTool, domain, profileDir string) (string, error) {
			if tool.name != "pycookiecheat" || domain != ".perplexity.ai" || profileDir != "" {
				t.Fatalf("unexpected direct extraction arguments: %#v, %q, %q", tool, domain, profileDir)
			}
			return "session=direct", nil
		},
		func(string) (string, error) {
			liveCalled = true
			return "session=live", nil
		},
	)
	if err != nil {
		t.Fatalf("refreshBrowserCookieSource() error = %v", err)
	}
	if cookies != "session=direct" {
		t.Fatalf("refreshBrowserCookieSource() = %q, want direct cookie value", cookies)
	}
	if liveCalled {
		t.Fatal("refreshBrowserCookieSource() called the live extractor despite a direct extractor")
	}
}

func TestRefreshBrowserCookieSourceFallsBackToLiveWithoutDirectExtractor(t *testing.T) {
	cookies, err := refreshBrowserCookieSource(
		".perplexity.ai",
		func() (cookieTool, error) { return cookieTool{}, errors.New("no direct extractor") },
		func(cookieTool, string, string) (string, error) {
			t.Fatal("direct extraction should not run when no direct extractor is available")
			return "", nil
		},
		func(domain string) (string, error) {
			if domain != ".perplexity.ai" {
				t.Fatalf("live extractor domain = %q, want .perplexity.ai", domain)
			}
			return "session=live", nil
		},
	)
	if err != nil {
		t.Fatalf("refreshBrowserCookieSource() error = %v", err)
	}
	if cookies != "session=live" {
		t.Fatalf("refreshBrowserCookieSource() = %q, want live cookie value", cookies)
	}
}

func TestRefreshBrowserCookieSourceRejectsFailedDirectExtraction(t *testing.T) {
	liveCalled := false
	_, err := refreshBrowserCookieSource(
		".perplexity.ai",
		func() (cookieTool, error) { return cookieTool{name: "pycookiecheat"}, nil },
		func(cookieTool, string, string) (string, error) { return "", errors.New("cookie database unavailable") },
		func(string) (string, error) {
			liveCalled = true
			return "session=partial", nil
		},
	)
	if err == nil {
		t.Fatal("refreshBrowserCookieSource() error = nil, want direct extraction error")
	}
	if liveCalled {
		t.Fatal("refreshBrowserCookieSource() accepted a potentially partial live cookie value after direct extraction failed")
	}
}
