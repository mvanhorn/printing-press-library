// Package rewe holds the hand-written REWE web shop logic: the self-renewing
// session, typed shop calls, the offers parser, recipes and product matching.
package rewe

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Cookie is one browser cookie as exported by Playwright, CDP
// (Storage.getCookies / Network.getAllCookies) or this CLI.
type Cookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires"`
	HTTPOnly bool    `json:"httpOnly"`
	Secure   bool    `json:"secure"`
}

// Session is the persisted cookie set of one REWE customer.
type Session struct {
	Cookies   []Cookie  `json:"cookies"`
	UpdatedAt time.Time `json:"updated_at"`
	RenewedAt time.Time `json:"renewed_at,omitempty"`
}

// ErrReloginNeeded means the single-sign-on cookies are gone or expired and a
// human has to log in to rewe.de in a browser again (captcha + 2FA).
var ErrReloginNeeded = errors.New("REWE session expired and could not be renewed: log in to rewe.de in a browser again, then run `auth import`")

// ErrNoSession means no session file exists yet.
var ErrNoSession = errors.New("no REWE session imported yet: log in to rewe.de in a browser, then run `auth import --cdp <url>` or `auth import --file cookies.json`")

// SessionPath returns the session file path. REWE_SESSION_FILE overrides it so
// one machine can serve several customers (one file per customer).
func SessionPath() (string, error) {
	if p := strings.TrimSpace(os.Getenv("REWE_SESSION_FILE")); p != "" {
		return p, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "rewe-pp-cli", "session.json"), nil
}

// LoadSession reads the session file.
func LoadSession(path string) (*Session, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNoSession
	}
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("reading session %s: %w", path, err)
	}
	return &s, nil
}

// Save writes the session file with owner-only permissions.
func (s *Session) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	s.UpdatedAt = time.Now().UTC()
	b, err := json.MarshalIndent(s, "", " ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ParseCookieExport accepts a JSON array of cookies, an object with a
// "cookies" array (CDP and Playwright storage state) or a session file.
func ParseCookieExport(data []byte) ([]Cookie, error) {
	data = []byte(strings.TrimSpace(string(data)))
	var arr []Cookie
	if err := json.Unmarshal(data, &arr); err == nil {
		return FilterREWE(arr), nil
	}
	var obj struct {
		Cookies []Cookie `json:"cookies"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("cookie export is neither a JSON array nor an object with \"cookies\": %w", err)
	}
	return FilterREWE(obj.Cookies), nil
}

// FilterREWE keeps cookies of rewe.de hosts only.
func FilterREWE(in []Cookie) []Cookie {
	out := make([]Cookie, 0, len(in))
	for _, c := range in {
		d := strings.TrimPrefix(strings.ToLower(c.Domain), ".")
		if d == "rewe.de" || strings.HasSuffix(d, ".rewe.de") {
			if c.Path == "" {
				c.Path = "/"
			}
			out = append(out, c)
		}
	}
	return out
}

// Has reports whether a cookie with that name exists (and is not expired).
func (s *Session) Has(name string) bool {
	now := float64(time.Now().Unix())
	for _, c := range s.Cookies {
		if c.Name == name && (c.Expires <= 0 || c.Expires > now) {
			return true
		}
	}
	return false
}

// SSOExpiry returns when the Keycloak single-sign-on cookie expires (zero when unknown).
func (s *Session) SSOExpiry() time.Time {
	for _, c := range s.Cookies {
		if c.Name == "KEYCLOAK_IDENTITY" && c.Expires > 0 {
			return time.Unix(int64(c.Expires), 0).UTC()
		}
	}
	return time.Time{}
}

// Jar builds an http.CookieJar holding the session cookies.
func (s *Session) Jar() (*cookiejar.Jar, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	byURL := map[string][]*http.Cookie{}
	for _, c := range s.Cookies {
		if strings.ContainsAny(c.Value, "\"; \\,") {
			// Not a valid HTTP cookie value (e.g. JSON consent blobs); the shop
			// does not need these and net/http would log and drop them anyway.
			continue
		}
		host := strings.TrimPrefix(c.Domain, ".")
		u := "https://" + host + c.Path
		hc := &http.Cookie{Name: c.Name, Value: c.Value, Path: c.Path, Secure: c.Secure, HttpOnly: c.HTTPOnly}
		if strings.HasPrefix(c.Domain, ".") {
			hc.Domain = c.Domain
		}
		if c.Expires > 0 {
			hc.Expires = time.Unix(int64(c.Expires), 0)
		}
		byURL[u] = append(byURL[u], hc)
	}
	for raw, cs := range byURL {
		u, err := url.Parse(raw)
		if err != nil {
			continue
		}
		jar.SetCookies(u, cs)
	}
	return jar, nil
}

// MergeFromJar copies cookies the server set during this run back into the
// session (values only; the jar does not expose expiry, so known expiries are
// kept and new cookies are stored as session cookies).
func (s *Session) MergeFromJar(jar http.CookieJar) {
	targets := []struct{ url, path string }{
		{"https://www.rewe.de/shop/", "/"},
		{"https://account.rewe.de/realms/sso/", "/realms/sso/"},
	}
	for _, t := range targets {
		u, _ := url.Parse(t.url)
		for _, hc := range jar.Cookies(u) {
			matched := false
			for i := range s.Cookies {
				if s.Cookies[i].Name == hc.Name && domainMatches(u.Host, s.Cookies[i].Domain) {
					s.Cookies[i].Value = hc.Value
					matched = true
				}
			}
			if !matched {
				s.Cookies = append(s.Cookies, Cookie{Name: hc.Name, Value: hc.Value, Domain: u.Host, Path: t.path, Secure: true})
			}
		}
	}
}

// domainMatches reports whether a cookie domain applies to host.
func domainMatches(host, domain string) bool {
	d := strings.TrimPrefix(strings.ToLower(domain), ".")
	host = strings.ToLower(host)
	return host == d || strings.HasSuffix(host, "."+d)
}

// ShopCookieHeader renders the cookies sent to www.rewe.de as a Cookie header
// value (used to seed the generated client's credential store).
func (s *Session) ShopCookieHeader() string {
	parts := []string{}
	seen := map[string]bool{}
	for _, c := range s.Cookies {
		d := strings.TrimPrefix(c.Domain, ".")
		if d != "rewe.de" && d != "www.rewe.de" {
			continue
		}
		if seen[c.Name] {
			continue
		}
		seen[c.Name] = true
		parts = append(parts, c.Name+"="+c.Value)
	}
	sort.Strings(parts)
	return strings.Join(parts, "; ")
}
