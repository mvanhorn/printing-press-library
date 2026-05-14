// Package ytstudio implements a minimal Studio Innertube client backed by a
// stored browser session. The runtime cost is a cookie jar + one SAPISIDHASH
// per request — no resident browser, no live page-context execution.
package ytstudio

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SessionFile is the on-disk representation of a captured Studio session.
type SessionFile struct {
	CapturedAt    time.Time         `json:"captured_at"`
	Cookies       map[string]string `json:"cookies"`
	ClientVersion string            `json:"client_version,omitempty"`
	ClientName    string            `json:"client_name,omitempty"` // "62" for Studio
	OnBehalfOf    string            `json:"on_behalf_of_user,omitempty"`
}

// DefaultPath returns the default Studio session path.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".yt-studio-session.json"
	}
	return filepath.Join(home, ".openclaw", "state", "yt-studio", "studio-session.json")
}

// Load reads a SessionFile from disk; returns (nil, os.ErrNotExist) if missing.
func Load(path string) (*SessionFile, error) {
	if path == "" {
		path = DefaultPath()
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s SessionFile
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("decoding session file: %w", err)
	}
	if len(s.Cookies) == 0 {
		return nil, errors.New("session file present but contains no cookies")
	}
	return &s, nil
}

// Save writes a SessionFile to disk with mode 0600.
func Save(path string, s *SessionFile) error {
	if path == "" {
		path = DefaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// CookieHeader renders the Cookie header value from the stored cookies.
func (s *SessionFile) CookieHeader() string {
	keys := make([]string, 0, len(s.Cookies))
	for k := range s.Cookies {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(s.Cookies[k])
	}
	return b.String()
}

// SAPISID returns the SAPISID cookie value (used by the Innertube auth header).
// Returns "" if the cookie isn't present.
func (s *SessionFile) SAPISID() string {
	if v, ok := s.Cookies["SAPISID"]; ok {
		return v
	}
	if v, ok := s.Cookies["__Secure-3PAPISID"]; ok {
		return v
	}
	return ""
}

// EffectiveClientName returns the Innertube clientName, defaulting to Studio (62).
func (s *SessionFile) EffectiveClientName() string {
	if s.ClientName != "" {
		return s.ClientName
	}
	return "62"
}

// EffectiveClientVersion returns the stored clientVersion or an empty string.
// The CLI's commands should treat empty as "unknown — recapture session".
func (s *SessionFile) EffectiveClientVersion() string {
	return s.ClientVersion
}
