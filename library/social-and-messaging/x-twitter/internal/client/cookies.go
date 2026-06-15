// Browser session cookie auth (Source B) for X Articles GraphQL endpoints.
//
// X Articles is a browser-only authoring surface served from x.com. It does
// not accept OAuth 2.0 user tokens; it requires the same auth_token + ct0
// session cookies that the x.com web app uses, plus a hardcoded Bearer that
// the web app embeds. Captured one-time via DevTools (Application → Cookies)
// and stored at ~/.config/x-twitter-pp-cli/cookies.json.
//
// Refresh the file when the session expires (typically when X invalidates
// the session or you log out and back in).

package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type cookieAuth struct {
	AuthToken  string `json:"auth_token"`
	CT0        string `json:"ct0"`
	WebBearer  string `json:"web_bearer"`
	UserID     string `json:"user_id"`
	TWID       string `json:"twid"`
	CapturedAt string `json:"captured_at"`
}

// LoadCookieAuth reads ~/.config/x-twitter-pp-cli/cookies.json. Returns an actionable
// error if the file is missing or fields are empty so the user knows to run
// the cookie-capture flow documented in SKILL.md.
func LoadCookieAuth() (*cookieAuth, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, ".config", "x-twitter-pp-cli", "cookies.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w (capture cookies via DevTools → Application → Cookies, see x-twitter-pp-cli's SKILL.md)", path, err)
	}
	var c cookieAuth
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse cookies file %s: %w", path, err)
	}
	if c.AuthToken == "" || c.CT0 == "" || c.WebBearer == "" {
		return nil, fmt.Errorf("cookies file %s is missing one of auth_token, ct0, web_bearer", path)
	}
	return &c, nil
}

func (c *cookieAuth) ArticleUserID() string {
	if c == nil {
		return ""
	}
	if c.UserID != "" {
		return c.UserID
	}
	return userIDFromTWID(c.TWID)
}

func userIDFromTWID(twid string) string {
	twid = strings.TrimSpace(twid)
	if twid == "" {
		return ""
	}
	if decoded, err := url.QueryUnescape(twid); err == nil {
		twid = decoded
	}
	twid = strings.Trim(twid, `"`)
	twid = strings.TrimPrefix(twid, "u=")
	twid = strings.TrimPrefix(twid, "u%3D")
	if twid == "" {
		return ""
	}
	for _, r := range twid {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return twid
}

// applyCookieAuth attaches Source B auth headers to req. Used for hosts
// x.com and upload.x.com (the Articles editor + media upload endpoints).
func (c *cookieAuth) apply(req *http.Request) {
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: c.AuthToken})
	req.AddCookie(&http.Cookie{Name: "ct0", Value: c.CT0})
	req.Header.Set("x-csrf-token", c.CT0)
	req.Header.Set("x-twitter-active-user", "yes")
	req.Header.Set("x-twitter-auth-type", "OAuth2Session")
	req.Header.Set("x-twitter-client-language", "en")
	req.Header.Set("Authorization", "Bearer "+c.WebBearer)
	// Many sniffed endpoints require an Origin/Referer that matches x.com.
	if req.Header.Get("Origin") == "" {
		req.Header.Set("Origin", "https://x.com")
	}
	if req.Header.Get("Referer") == "" {
		req.Header.Set("Referer", "https://x.com/")
	}
}

// hostUsesCookieAuth reports whether the given URL host should be authenticated
// via Source B (browser session cookies) rather than Source A (OAuth 2.0).
func hostUsesCookieAuth(host string) bool {
	switch host {
	case "x.com", "twitter.com", "upload.x.com", "upload.twitter.com":
		return true
	}
	return false
}

// isAllowedAbsoluteHost is the allowlist for absolute URLs the client will dial.
// This guards against token exfiltration if a caller path becomes user-controllable
// — without an allowlist, any "https://attacker.com/..." would pass through and
// would receive the OAuth bearer (because Source A applies to anything not in
// hostUsesCookieAuth).
func isAllowedAbsoluteHost(host string) bool {
	switch host {
	case "api.x.com", "api.twitter.com",
		"x.com", "twitter.com",
		"upload.x.com", "upload.twitter.com":
		return true
	}
	return false
}

// IsAllowedAbsoluteHost reports whether an absolute raw/API URL host is on the
// X/Twitter allowlist. CLI-side validators use this to fail before building a
// request.
func IsAllowedAbsoluteHost(host string) bool {
	return isAllowedAbsoluteHost(host)
}

// hostFromURL extracts the hostname from a possibly-absolute URL. Empty if not parseable
// or the URL is relative.
func hostFromURL(u string) string {
	parsed, err := url.Parse(u)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}
