// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.

package airbnb

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Session holds the Airbnb cookies that authenticate private API calls. It is
// persisted to ~/.config/airbnb-outreach-pp-cli/session.json (0600). Cookie values are
// session credentials — the file is user-only and never logged.
type Session struct {
	Cookies    map[string]string `json:"cookies"`
	Source     string            `json:"source"`      // "chrome", "manual", "env"
	ImportedAt time.Time         `json:"imported_at"`
	Domain     string            `json:"domain"`      // e.g. ".airbnb.com"
}

func sessionPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "airbnb-outreach-pp-cli", "session.json")
}

// LoadSession reads the persisted session. A missing file returns an empty
// session with no cookies (public-only mode), not an error.
func LoadSession() *Session {
	s := &Session{Cookies: map[string]string{}}
	data, err := os.ReadFile(sessionPath())
	if err != nil {
		return s
	}
	_ = json.Unmarshal(data, s)
	if s.Cookies == nil {
		s.Cookies = map[string]string{}
	}
	return s
}

// Save writes the session file with 0600 permissions.
func (s *Session) Save() error {
	path := sessionPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// Clear removes the session file (logout).
func ClearSession() error {
	err := os.Remove(sessionPath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Authenticated reports whether the session carries a plausible Airbnb login
// cookie. Airbnb's httpOnly session cookie is `_aat` (auth token); `_airbed_session_id`
// is also present on logged-in sessions.
func (s *Session) Authenticated() bool {
	if s == nil {
		return false
	}
	for _, k := range []string{"_aat", "_airbed_session_id"} {
		if v := s.Cookies[k]; v != "" {
			return true
		}
	}
	return false
}

// CookieHeader renders the cookies as a single Cookie request-header value.
func (s *Session) CookieHeader() string {
	if s == nil || len(s.Cookies) == 0 {
		return ""
	}
	names := make([]string, 0, len(s.Cookies))
	for k := range s.Cookies {
		names = append(names, k)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, k := range names {
		parts = append(parts, k+"="+s.Cookies[k])
	}
	return strings.Join(parts, "; ")
}

// ImportFromCookieString parses a "k=v; k2=v2" Cookie header (e.g. copied from
// DevTools) into the session. Returns the number of cookies parsed.
func (s *Session) ImportFromCookieString(raw string) int {
	if s.Cookies == nil {
		s.Cookies = map[string]string{}
	}
	n := 0
	for _, pair := range strings.Split(raw, ";") {
		pair = strings.TrimSpace(pair)
		k, v, ok := strings.Cut(pair, "=")
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if !ok || k == "" || v == "" {
			continue
		}
		s.Cookies[k] = v
		n++
	}
	return n
}

// ImportFromChrome reads Airbnb cookies directly from the local Chrome profile,
// decrypting them with the OS credential store. profile is the Chrome profile
// directory name (e.g. "Default", "Profile 1"); empty means "Default". The
// implementation is OS-specific (see chrome_windows.go / chrome_other.go).
func (s *Session) ImportFromChrome(profile string) (int, error) {
	if s.Cookies == nil {
		s.Cookies = map[string]string{}
	}
	cookies, err := importChromeCookies(profile)
	if err != nil {
		return 0, err
	}
	n := 0
	for k, v := range cookies {
		if v == "" {
			continue
		}
		s.Cookies[k] = v
		n++
	}
	if n == 0 {
		return 0, fmt.Errorf("no Airbnb cookies found in Chrome profile %q (are you logged in to airbnb.com in Chrome?)", chromeProfileOrDefault(profile))
	}
	s.Source = "chrome"
	s.ImportedAt = time.Now()
	s.Domain = ".airbnb.com"
	return n, nil
}

func chromeProfileOrDefault(p string) string {
	if p == "" {
		return "Default"
	}
	return p
}
