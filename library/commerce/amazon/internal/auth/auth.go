// Package auth manages Amazon session cookies for the CLI.
//
// Three import paths, mirroring the instacart pattern:
//
//	LoadFromChrome  - shells out to kooky to read the macOS / Linux Chrome
//	                  cookie store. Most ergonomic; fails when the user has
//	                  not granted Keychain access.
//	LoadFromPaste   - takes a Cookie: header string copied from DevTools.
//	                  Always works.
//	LoadFromFile    - reads a JSON file in the shape Chrome's "Cookie-Editor"
//	                  extension exports (array of {name,value,domain,...}).
//
// All three normalize to a Session, then persist it to cookies.json under the
// profile directory. Subsequent commands call Load(profile) which reads
// cookies.json — fast path, no Keychain prompt.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/browserutils/kooky"
	_ "github.com/browserutils/kooky/browser/chrome"
)

// Cookie is the persisted shape.
type Cookie struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Domain   string    `json:"domain"`
	Path     string    `json:"path"`
	Expires  time.Time `json:"expires,omitempty"`
	Secure   bool      `json:"secure,omitempty"`
	HTTPOnly bool      `json:"http_only,omitempty"`
}

// Session is the in-memory auth state.
type Session struct {
	Cookies   []Cookie  `json:"cookies"`
	UserAgent string    `json:"user_agent,omitempty"`
	SavedAt   time.Time `json:"saved_at"`
}

// LoadFromChrome reads cookies for .amazon.com from the user's Chrome profile
// via kooky. Returns a Session ready to be persisted.
func LoadFromChrome() (*Session, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	stores := kooky.FindAllCookieStores(ctx)
	if len(stores) == 0 {
		return nil, errors.New("no Chrome cookie stores found on this machine")
	}
	var cookies []Cookie
	for _, st := range stores {
		// Limit to Chromium-family browsers; ignore Firefox stores for now.
		browser := strings.ToLower(st.Browser())
		if !strings.Contains(browser, "chrome") &&
			!strings.Contains(browser, "edge") &&
			!strings.Contains(browser, "brave") &&
			!strings.Contains(browser, "arc") {
			_ = st.Close()
			continue
		}
		seq := st.TraverseCookies(kooky.Valid)
		all, err := seq.ReadAllCookies(ctx)
		_ = st.Close()
		if err != nil {
			// Keychain denial / permission error — try the next store.
			continue
		}
		for _, c := range all {
			if !strings.Contains(c.Domain, "amazon") {
				continue
			}
			cookies = append(cookies, normalizeKookyCookie(c))
		}
	}
	if len(cookies) == 0 {
		return nil, errors.New("no Amazon cookies found in any Chrome store (try `auth paste` or `auth import-file` instead)")
	}
	return &Session{Cookies: cookies, SavedAt: time.Now().UTC()}, nil
}

// LoadFromPaste parses a Cookie: header line into a Session. The line is the
// raw string copied from DevTools (`Cookie: session-id=...; session-token=...`).
func LoadFromPaste(header, domain string) (*Session, error) {
	if domain == "" {
		domain = ".amazon.com"
	}
	header = strings.TrimSpace(header)
	header = strings.TrimPrefix(header, "Cookie:")
	header = strings.TrimPrefix(header, "cookie:")
	header = strings.TrimSpace(header)
	if header == "" {
		return nil, errors.New("empty Cookie header")
	}
	parts := strings.Split(header, ";")
	var cookies []Cookie
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		eq := strings.IndexByte(p, '=')
		if eq <= 0 {
			continue
		}
		name := strings.TrimSpace(p[:eq])
		val := strings.TrimSpace(p[eq+1:])
		if name == "" {
			continue
		}
		cookies = append(cookies, Cookie{Name: name, Value: val, Domain: domain, Path: "/"})
	}
	if len(cookies) == 0 {
		return nil, errors.New("no name=value pairs parsed from header")
	}
	return &Session{Cookies: cookies, SavedAt: time.Now().UTC()}, nil
}

// LoadFromFile reads a JSON file produced by Chrome cookie-export extensions.
// Accepts two shapes:
//
//  1. Array of objects with name/value/domain/path/expires fields.
//  2. Wrapper object with a top-level "cookies" array.
func LoadFromFile(path string) (*Session, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	// Try wrapper first.
	var wrap struct {
		Cookies []Cookie `json:"cookies"`
	}
	if err := json.Unmarshal(b, &wrap); err == nil && len(wrap.Cookies) > 0 {
		return &Session{Cookies: wrap.Cookies, SavedAt: time.Now().UTC()}, nil
	}
	var raw []map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("parsing cookies JSON: %w", err)
	}
	var cookies []Cookie
	for _, r := range raw {
		c := Cookie{
			Name:   strs(r, "name"),
			Value:  strs(r, "value"),
			Domain: strs(r, "domain"),
			Path:   strs(r, "path"),
		}
		if c.Name == "" || c.Value == "" {
			continue
		}
		if c.Domain == "" {
			c.Domain = ".amazon.com"
		}
		if c.Path == "" {
			c.Path = "/"
		}
		if exp, ok := r["expirationDate"].(float64); ok {
			c.Expires = time.Unix(int64(exp), 0)
		}
		cookies = append(cookies, c)
	}
	if len(cookies) == 0 {
		return nil, errors.New("no usable cookies in file")
	}
	return &Session{Cookies: cookies, SavedAt: time.Now().UTC()}, nil
}

// Save persists s to path with restrictive perms.
func (s *Session) Save(path string) error {
	tmp := path + ".tmp"
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Load reads a previously-saved Session from path.
func Load(path string) (*Session, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	if len(s.Cookies) == 0 {
		return nil, errors.New("session has no cookies")
	}
	return &s, nil
}

// CookieHeader returns the Cookie: header value for this session.
func (s *Session) CookieHeader() string {
	parts := make([]string, 0, len(s.Cookies))
	for _, c := range s.Cookies {
		parts = append(parts, c.Name+"="+c.Value)
	}
	return strings.Join(parts, "; ")
}

// HasMarker returns true if the session contains the cookie names typically
// required for an authenticated amazon.com page load. Used by doctor.
func (s *Session) HasMarker() bool {
	required := map[string]bool{
		"at-main":       false, // primary auth token
		"sess-at-main":  false,
		"session-id":    false,
		"session-token": false,
		"ubid-main":     false,
	}
	for _, c := range s.Cookies {
		if _, ok := required[c.Name]; ok {
			required[c.Name] = true
		}
	}
	// Need at least one of the at-main/sess-at-main pair AND a session-id.
	hasAt := required["at-main"] || required["sess-at-main"]
	return hasAt && required["session-id"]
}

// ApplyToRequest sets the Cookie header on req from this session.
func (s *Session) ApplyToRequest(req *http.Request) {
	req.Header.Set("Cookie", s.CookieHeader())
	if s.UserAgent != "" {
		req.Header.Set("User-Agent", s.UserAgent)
	}
}

// CookieJar returns a net/http/cookiejar-friendly jar populated with these
// cookies for use with http.Client.Jar. The URL hint is the marketplace base.
func (s *Session) CookieJar(base string) (http.CookieJar, error) {
	u, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	jar, err := newJar()
	if err != nil {
		return nil, err
	}
	var hcookies []*http.Cookie
	for _, c := range s.Cookies {
		hcookies = append(hcookies, &http.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  c.Expires,
			Secure:   c.Secure,
			HttpOnly: c.HTTPOnly,
		})
	}
	jar.SetCookies(u, hcookies)
	return jar, nil
}

func strs(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func normalizeKookyCookie(c *kooky.Cookie) Cookie {
	return Cookie{
		Name:     c.Cookie.Name,
		Value:    c.Cookie.Value,
		Domain:   c.Cookie.Domain,
		Path:     c.Cookie.Path,
		Expires:  c.Cookie.Expires,
		Secure:   c.Cookie.Secure,
		HTTPOnly: c.Cookie.HttpOnly,
	}
}
