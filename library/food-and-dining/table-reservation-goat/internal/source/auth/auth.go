// Package auth manages browser-cookie sessions for the OpenTable and Tock
// consumer endpoints. There is no API key for either service; all
// authenticated reads and writes ride on the browser session cookies the
// user already has after logging in to opentable.com / exploretock.com in
// Chrome.
package auth

// PATCH: cross-network-source-clients — see .printing-press-patches.json for the change-set rationale.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	NetworkOpenTable = "opentable"
	NetworkTock      = "tock"
	// NetworkResy is the third reservation network supported by the CLI.
	// Resy's auth model is token-based (not cookie-based), so the on-disk
	// session carries a ResyAuth struct instead of a cookie slice.
	NetworkResy = "resy"

	OpenTableHost   = "www.opentable.com"
	OpenTableDomain = ".opentable.com"
	TockHost        = "www.exploretock.com"
	TockDomain      = ".exploretock.com"
	ResyHost        = "resy.com"
)

// Cookie is the on-disk shape of a stored cookie. It mirrors the subset of
// http.Cookie that round-trips through JSON cleanly. We do not persist
// HttpOnly or Secure flags because Surf will set them per-request from the
// destination origin.
type Cookie struct {
	Name    string    `json:"name"`
	Value   string    `json:"value"`
	Domain  string    `json:"domain"`
	Path    string    `json:"path"`
	Expires time.Time `json:"expires,omitempty"`
}

// ResyAuth carries the per-user Resy credentials persisted on disk. Resy's
// auth model is token-based: a shared public APIKey (the same value for
// every browser visiting resy.com) plus a per-user JWT obtained from
// POST /3/auth/password. The password itself is never persisted.
type ResyAuth struct {
	APIKey    string `json:"api_key,omitempty"`
	AuthToken string `json:"auth_token,omitempty"`
	Email     string `json:"email,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	UserID    int64  `json:"user_id,omitempty"`
}

// Session is the on-disk authentication state for all reservation networks.
// OpenTable and Tock use cookie-session auth; Resy uses a bearer-style
// token. Storing both in one file keeps `auth status` a single read.
type Session struct {
	Version          int       `json:"version"`
	UpdatedAt        time.Time `json:"updated_at"`
	OpenTableCookies []Cookie  `json:"opentable_cookies,omitempty"`
	TockCookies      []Cookie  `json:"tock_cookies,omitempty"`
	Resy             *ResyAuth `json:"resy,omitempty"`
}

// LoggedIn returns true when the network has at least one non-expired session
// cookie (OT/Tock) or a stored auth token (Resy). The caller is responsible
// for distinguishing OT's `authCke` cookie from Tock's session cookie when a
// stricter check is needed.
func (s *Session) LoggedIn(network string) bool {
	now := time.Now()
	var cookies []Cookie
	switch strings.ToLower(network) {
	case NetworkOpenTable:
		cookies = s.OpenTableCookies
		// OT's `authCke` is the auth-confirming cookie; require it.
		return hasNamedFresh(cookies, "authCke", now)
	case NetworkTock:
		cookies = s.TockCookies
		// Tock uses an opaque session cookie; any fresh non-cf_bm cookie counts.
		for _, c := range cookies {
			if c.Name == "__cf_bm" {
				continue
			}
			if c.Expires.IsZero() || c.Expires.After(now) {
				return true
			}
		}
		return false
	case NetworkResy:
		// Resy's token has no expiration field — Resy rotates it on the
		// server side. Presence of a non-empty AuthToken is the auth
		// signal; the next call's 401 surfaces a stale token.
		return s.Resy != nil && s.Resy.AuthToken != ""
	}
	return false
}

func hasNamedFresh(cookies []Cookie, name string, now time.Time) bool {
	for _, c := range cookies {
		if c.Name != name {
			continue
		}
		if c.Expires.IsZero() || c.Expires.After(now) {
			return true
		}
	}
	return false
}

// HTTPCookies returns the stored cookies for a network as net/http cookies,
// suitable for setting on a Surf cookie jar. Some real-world cookies (notably
// Cloudflare's `__cf_bm`) carry leading/trailing quote characters that Go's
// strict cookie parser rejects with "invalid byte". We strip surrounding
// quotes here so the values round-trip through net/http cleanly.
func (s *Session) HTTPCookies(network string) []*http.Cookie {
	var src []Cookie
	switch strings.ToLower(network) {
	case NetworkOpenTable:
		src = s.OpenTableCookies
	case NetworkTock:
		src = s.TockCookies
	}
	return cookiesToHTTP(src)
}

// HTTPCookiesWithRefresh returns the stored cookies for a network, with
// Akamai/Cloudflare bot-defense cookies overlaid from a fresh Chrome read.
// Long-lived auth cookies come from the saved session (so the CLI works
// when Chrome is closed or after the user signed out); short-lived bot
// cookies come from Chrome live (so Akamai's challenge-rotation doesn't
// freeze us out). When fresh is empty the result equals HTTPCookies.
func (s *Session) HTTPCookiesWithRefresh(network string, fresh []Cookie) []*http.Cookie {
	var src []Cookie
	switch strings.ToLower(network) {
	case NetworkOpenTable:
		src = s.OpenTableCookies
	case NetworkTock:
		src = s.TockCookies
	}
	if len(fresh) == 0 {
		return cookiesToHTTP(src)
	}
	// Index fresh cookies by Name+Domain+Path so saved entries with the same
	// key are replaced rather than duplicated. Akamai sometimes serves the
	// same cookie name under both `.opentable.com` and `.www.opentable.com`;
	// keying on all three avoids accidentally dropping the right one.
	type key struct{ name, domain, path string }
	freshKeys := make(map[key]bool, len(fresh))
	for _, c := range fresh {
		freshKeys[key{c.Name, c.Domain, c.Path}] = true
	}
	merged := make([]Cookie, 0, len(src)+len(fresh))
	for _, c := range src {
		if freshKeys[key{c.Name, c.Domain, c.Path}] {
			continue
		}
		merged = append(merged, c)
	}
	merged = append(merged, fresh...)
	return cookiesToHTTP(merged)
}

func cookiesToHTTP(src []Cookie) []*http.Cookie {
	out := make([]*http.Cookie, 0, len(src))
	for _, c := range src {
		v := c.Value
		if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
			v = v[1 : len(v)-1]
		}
		// Skip any cookie whose name/value still contains characters Go's
		// http.Cookie rejects; better to omit one cookie than poison the jar.
		if !validCookieValue(v) {
			continue
		}
		out = append(out, &http.Cookie{
			Name:    c.Name,
			Value:   v,
			Domain:  c.Domain,
			Path:    c.Path,
			Expires: c.Expires,
		})
	}
	return out
}

// validCookieValue mirrors the byte set net/http accepts: visible ASCII
// minus DEL, with quote/backslash/comma/semicolon/space rejected.
func validCookieValue(v string) bool {
	for i := 0; i < len(v); i++ {
		b := v[i]
		switch {
		case b < 0x20, b > 0x7E:
			return false
		case b == '"', b == '\\', b == ',', b == ';', b == ' ':
			return false
		}
	}
	return true
}

// SessionPath returns the on-disk session file path. Honors the same config
// directory the generated config package uses so users can override via
// $TABLE_RESERVATION_GOAT_CONFIG_DIR.
func SessionPath() (string, error) {
	if env := os.Getenv("TABLE_RESERVATION_GOAT_SESSION_PATH"); env != "" {
		return env, nil
	}
	if env := os.Getenv("TABLE_RESERVATION_GOAT_CONFIG_DIR"); env != "" {
		return filepath.Join(env, "session.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(home, ".config", "table-reservation-goat-pp-cli", "session.json"), nil
}

// Load reads the session from disk. Returns an empty session (not an error)
// if the file does not exist — first-run is the most common case.
func Load() (*Session, error) {
	path, err := SessionPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Session{Version: 1}, nil
		}
		return nil, fmt.Errorf("reading session: %w", err)
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing session: %w", err)
	}
	if s.Version == 0 {
		s.Version = 1
	}
	return &s, nil
}

// Save writes the session atomically (write-then-rename) and creates parent
// directories as needed. The file is mode 0600 because session cookies are
// sensitive credential material.
func (s *Session) Save() error {
	path, err := SessionPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating session directory: %w", err)
	}
	s.Version = 1
	s.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling session: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("writing session: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("renaming session: %w", err)
	}
	return nil
}

// Clear removes the on-disk session.
func Clear() error {
	path, err := SessionPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing session: %w", err)
	}
	return nil
}

// CookieURLFor returns a stable URL pointing at a network's host so callers
// can attach cookies to a `net/http/cookiejar` keyed by URL. Resy is omitted
// — token-based auth doesn't ride on a cookie jar.
func CookieURLFor(network string) (*url.URL, error) {
	switch strings.ToLower(network) {
	case NetworkOpenTable:
		return url.Parse("https://" + OpenTableHost + "/")
	case NetworkTock:
		return url.Parse("https://" + TockHost + "/")
	}
	return nil, fmt.Errorf("unknown network: %s", network)
}
