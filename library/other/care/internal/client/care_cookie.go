package client

import (
	"os"
	"path/filepath"
	"strings"
)

// CareCookieCachePath is where `care-pp-cli auth login/refresh` caches the
// session cookie header extracted from the persistent Chrome profile.
func CareCookieCachePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".care-pp-cli", "session-cookie")
}

// SessionCookie exposes the resolved care.com session cookie header to other
// packages (e.g. the messages command, which fetches the Stream token from the
// members page and then talks to Stream Chat directly).
func SessionCookie() string { return careSessionCookie() }

// careSessionCookie resolves the care.com session cookie header.
// Precedence: CARE_SESSION_COOKIE env override, then the cached file.
func careSessionCookie() string {
	if sc := strings.TrimSpace(os.Getenv("CARE_SESSION_COOKIE")); sc != "" {
		return sc
	}
	p := CareCookieCachePath()
	if p == "" {
		return ""
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
