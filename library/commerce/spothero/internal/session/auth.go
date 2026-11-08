// Package session manages SpotHero consumer session cookies.
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	AppName      = "spothero-pp-cli"
	CookiesEnv   = "SPOTHERO_COOKIES"
	CookieDomain = "spothero.com"
)

// Session holds cookie auth material. Values are never logged.
type Session struct {
	RawCookieHeader string            `json:"cookie_header,omitempty"`
	Cookies         map[string]string `json:"cookies,omitempty"`
	Source          string            `json:"source,omitempty"`
	UpdatedAt       string            `json:"updated_at,omitempty"`
}

// HomeDir resolves the config directory.
func HomeDir(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	if v := os.Getenv("SPOTHERO_PP_HOME"); v != "" {
		return v, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, AppName), nil
}

func cookiesPath(home string) string {
	return filepath.Join(home, "cookies.json")
}

// LoadSession reads stored cookies (0600 file).
func LoadSession(home string) (*Session, error) {
	path := cookiesPath(home)
	b, err := os.ReadFile(path) // #nosec G304 -- fixed filename under the CLI's own config dir
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("parse cookies file: %w", err)
	}
	return &s, nil
}

// SaveSession writes cookies with mode 0600.
func SaveSession(home string, s *Session) error {
	if err := os.MkdirAll(home, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	path := cookiesPath(home)
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

// CookieValue returns a named cookie from the session, if present.
func (s *Session) CookieValue(name string) string {
	if s == nil {
		return ""
	}
	if s.Cookies != nil {
		if v, ok := s.Cookies[name]; ok {
			return v
		}
	}
	if s.RawCookieHeader == "" {
		return ""
	}
	prefix := name + "="
	for _, part := range strings.Split(s.RawCookieHeader, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, prefix) {
			return part[len(prefix):]
		}
	}
	return ""
}

// CSRFToken returns the csrftoken cookie for mutating session API requests.
func (s *Session) CSRFToken() string {
	return s.CookieValue("csrftoken")
}

// CookieHeader returns the Cookie header value for HTTP requests.
func (s *Session) CookieHeader() string {
	if s == nil {
		return ""
	}
	if s.RawCookieHeader != "" {
		return s.RawCookieHeader
	}
	if len(s.Cookies) == 0 {
		return ""
	}
	parts := make([]string, 0, len(s.Cookies))
	for k, v := range s.Cookies {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, "; ")
}

// ResolveSession loads file cookies, then SPOTHERO_COOKIES env.
func ResolveSession(home string) (*Session, error) {
	if v := strings.TrimSpace(os.Getenv(CookiesEnv)); v != "" {
		return ParseCookieInput(v, "env:"+CookiesEnv)
	}
	return LoadSession(home)
}
