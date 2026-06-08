package client

import (
	"net/http"
	"net/url"
	"path/filepath"
	"testing"
	"time"
)

func TestPersistedCookieJar_RoundTripAndDomainMatching(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "cookies.json")
	jar, err := newPersistedCookieJar(path)
	if err != nil {
		t.Fatalf("newPersistedCookieJar: %v", err)
	}

	baseURL, _ := url.Parse("https://www.continente.pt/")
	jar.SetCookies(baseURL, []*http.Cookie{
		{
			Name:    "sid",
			Value:   "abc",
			Domain:  ".continente.pt",
			Path:    "/",
			Secure:  true,
			Expires: time.Now().Add(time.Hour),
		},
	})

	loaded, err := newPersistedCookieJar(path)
	if err != nil {
		t.Fatalf("newPersistedCookieJar reload: %v", err)
	}

	subURL, _ := url.Parse("https://login.continente.pt/auth")
	cookies := loaded.Cookies(subURL)
	if len(cookies) != 1 {
		t.Fatalf("Cookies(...) len = %d, want 1", len(cookies))
	}
	if cookies[0].Name != "sid" || cookies[0].Value != "abc" {
		t.Fatalf("Cookies(...)[0] = %#v, want sid=abc", cookies[0])
	}
}

func TestPersistedCookieJar_ExpiredCookieNotReturned(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "cookies.json")
	jar, err := newPersistedCookieJar(path)
	if err != nil {
		t.Fatalf("newPersistedCookieJar: %v", err)
	}

	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	jar.nowFunc = func() time.Time { return now }

	baseURL, _ := url.Parse("https://www.continente.pt/")
	jar.SetCookies(baseURL, []*http.Cookie{
		{
			Name:    "sid",
			Value:   "expired",
			Domain:  ".continente.pt",
			Path:    "/",
			Secure:  true,
			Expires: now.Add(-time.Minute),
		},
	})

	if got := jar.Cookies(baseURL); len(got) != 0 {
		t.Fatalf("Cookies(...) len = %d, want 0", len(got))
	}
}
