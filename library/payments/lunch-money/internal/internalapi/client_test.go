package internalapi

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

// PATCH: Allow internal API cookie auth to come from an env var without writing
// the session secret into the persisted cookie jar.
func TestNewUsesInternalCookieEnvWithoutPersisting(t *testing.T) {
	t.Setenv(EnvInternalCookie, "Cookie: lm_session=env-session; lm_access_token=env-access")
	cookiePath := filepath.Join(t.TempDir(), "internal-cookies.json")

	c, err := New(cookiePath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !c.HasSession() {
		t.Fatal("HasSession = false, want true from env cookie")
	}
	if _, err := os.Stat(cookiePath); !os.IsNotExist(err) {
		t.Fatalf("cookie file exists or stat failed: %v", err)
	}
	if err := c.saveCookies(); err != nil {
		t.Fatalf("saveCookies: %v", err)
	}
	if _, err := os.Stat(cookiePath); !os.IsNotExist(err) {
		t.Fatalf("cookie file exists after saveCookies with env cookie: %v", err)
	}

	apiURL, _ := url.Parse(DefaultBaseURL)
	cookies := c.Jar.Cookies(apiURL)
	if cookieValue(cookies, "lm_session") != "env-session" {
		t.Fatalf("lm_session cookie not loaded from env: %#v", cookies)
	}
	if cookieValue(cookies, "Cookie: lm_session") != "" {
		t.Fatalf("Cookie: prefix was parsed as a cookie name: %#v", cookies)
	}
}

func TestSetCookieStringStripsCookieHeaderPrefixAndPersists(t *testing.T) {
	cookiePath := filepath.Join(t.TempDir(), "internal-cookies.json")
	c, err := New(cookiePath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	c.SetCookieString("Cookie: lm_session=file-session; lm_access_token=file-access")

	data, err := os.ReadFile(cookiePath)
	if err != nil {
		t.Fatalf("read cookie file: %v", err)
	}
	var persisted []PersistentCookie
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("decode cookie file: %v", err)
	}
	if persistentCookieValue(persisted, "lm_session") != "file-session" {
		t.Fatalf("persisted cookies = %#v", persisted)
	}
	if persistentCookieValue(persisted, "Cookie: lm_session") != "" {
		t.Fatalf("Cookie: prefix was persisted as a cookie name: %#v", persisted)
	}
}

func cookieValue(cookies []*http.Cookie, name string) string {
	for _, ck := range cookies {
		if ck.Name == name {
			return ck.Value
		}
	}
	return ""
}

func persistentCookieValue(cookies []PersistentCookie, name string) string {
	for _, ck := range cookies {
		if ck.Name == name {
			return ck.Value
		}
	}
	return ""
}
