// Copyright 2026 github-actionsbot and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: see .printing-press-patches/2026-08-30-nextauth-cookie-surface.md.

package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempCookiesFile(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "cookies.json")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("writing temp cookies file: %v", err)
	}
	return path
}

func TestLoadSessionCookiesFromFile_PlaywrightStorageStateShape(t *testing.T) {
	t.Parallel()

	path := writeTempCookiesFile(t, `{
		"cookies": [
			{"name": "__Secure-next-auth.session-token", "value": "tok1", "domain": "labs.google", "path": "/"},
			{"name": "email", "value": "user@example.com", "domain": "labs.google", "path": "/"}
		],
		"origins": []
	}`)

	cookies, err := loadSessionCookiesFromFile(path, "labs.google", nextAuthCookieNames)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cookies) != 1 {
		t.Fatalf("expected exactly 1 cookie (session-token only, email filtered out), got %d: %v", len(cookies), cookies)
	}
	if cookies[0].Name != "__Secure-next-auth.session-token" || cookies[0].Value != "tok1" {
		t.Fatalf("unexpected cookie: %+v", cookies[0])
	}
}

func TestLoadSessionCookiesFromFile_BareArrayShape(t *testing.T) {
	t.Parallel()

	// browser-use `cookies export`, Puppeteer's page.cookies(), and most
	// DevTools cookie-export extensions all emit a bare top-level array,
	// not Playwright's {"cookies":[...]} wrapper.
	path := writeTempCookiesFile(t, `[
		{"name": "__Secure-next-auth.session-token", "value": "tok2", "domain": "labs.google", "path": "/"},
		{"name": "_ga", "value": "GA1.1.xxx", "domain": ".labs.google", "path": "/"}
	]`)

	cookies, err := loadSessionCookiesFromFile(path, "labs.google", nextAuthCookieNames)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cookies) != 1 {
		t.Fatalf("expected exactly 1 cookie (session-token only, _ga filtered out), got %d: %v", len(cookies), cookies)
	}
	if cookies[0].Name != "__Secure-next-auth.session-token" || cookies[0].Value != "tok2" {
		t.Fatalf("unexpected cookie: %+v", cookies[0])
	}
}

func TestLoadSessionCookiesFromFile_WrongDomainExcluded(t *testing.T) {
	t.Parallel()

	path := writeTempCookiesFile(t, `[
		{"name": "__Secure-next-auth.session-token", "value": "tok3", "domain": "aisandbox-pa.googleapis.com", "path": "/"}
	]`)

	_, err := loadSessionCookiesFromFile(path, "labs.google", nextAuthCookieNames)
	if err == nil {
		t.Fatalf("expected an error when no cookies match the requested domain")
	}
}

func TestLoadSessionCookiesFromFile_RawCookieHeader(t *testing.T) {
	t.Parallel()

	path := writeTempCookiesFile(t, "Cookie: __Secure-next-auth.session-token=tok4; email=user@example.com")

	cookies, err := loadSessionCookiesFromFile(path, "labs.google", nextAuthCookieNames)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cookies) != 1 {
		t.Fatalf("expected exactly 1 cookie (session-token only, email filtered out), got %d: %v", len(cookies), cookies)
	}
	if cookies[0].Name != "__Secure-next-auth.session-token" || cookies[0].Value != "tok4" {
		t.Fatalf("unexpected cookie: %+v", cookies[0])
	}
}

func TestLoadSessionCookiesFromFile_EmptyFile(t *testing.T) {
	t.Parallel()

	path := writeTempCookiesFile(t, "")

	_, err := loadSessionCookiesFromFile(path, "labs.google", nextAuthCookieNames)
	if err == nil {
		t.Fatalf("expected an error for an empty cookies file")
	}
}

func TestLoadSessionCookiesFromFile_EmptyCookiesArray(t *testing.T) {
	t.Parallel()

	path := writeTempCookiesFile(t, `{"cookies": [], "origins": []}`)

	_, err := loadSessionCookiesFromFile(path, "labs.google", nextAuthCookieNames)
	if err == nil {
		t.Fatalf("expected an error for an explicitly empty cookies array")
	}
}

func TestSessionCookieNameAllowed(t *testing.T) {
	t.Parallel()

	if !sessionCookieNameAllowed("__Secure-next-auth.session-token", nextAuthCookieNames) {
		t.Fatalf("expected the real next-auth session cookie name to be allowed")
	}
	if sessionCookieNameAllowed("email", nextAuthCookieNames) {
		t.Fatalf("expected an unrelated app cookie name to be excluded")
	}
	if !sessionCookieNameAllowed("anything", nil) {
		t.Fatalf("expected a nil allowlist to allow everything (no filter requested)")
	}
}
