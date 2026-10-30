package redfin

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Cookie is one browser cookie from an export file.
type Cookie struct {
	Name   string
	Value  string
	Domain string
}

// exportedCookie covers the field names used by common JSON cookie exports:
// browser extensions use expirationDate, Playwright and browser-use use
// expires. A session cookie leaves both unset or sets them to -1.
type exportedCookie struct {
	Name           string   `json:"name"`
	Value          string   `json:"value"`
	Domain         string   `json:"domain"`
	Expires        *float64 `json:"expires"`
	ExpirationDate *float64 `json:"expirationDate"`
}

// ParseCookieExport reads a JSON cookie export (an array, or an object with a
// cookies array) or a Netscape cookies.txt file, and returns the unexpired
// cookies for redfin.com sorted by name. It fails when none are left, so a
// wrong or stale file is reported at import instead of as a later 403.
func ParseCookieExport(data []byte, now time.Time) ([]Cookie, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, errors.New("cookie file is empty")
	}

	var parsed []Cookie
	var err error
	if strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "{") {
		parsed, err = parseJSONCookies([]byte(trimmed), now)
	} else {
		parsed, err = parseNetscapeCookies(trimmed, now)
	}
	if err != nil {
		return nil, err
	}

	byName := map[string]Cookie{}
	for _, c := range parsed {
		if c.Name == "" || !appliesToRedfinHost(c.Domain) {
			continue
		}
		// A host cookie beats a parent-domain cookie of the same name, as it
		// would in the browser's request to www.redfin.com.
		if prev, ok := byName[c.Name]; ok && len(cookieHost(prev.Domain)) > len(cookieHost(c.Domain)) {
			continue
		}
		byName[c.Name] = c
	}
	if len(byName) == 0 {
		return nil, errors.New("no unexpired redfin.com cookies in file")
	}
	out := make([]Cookie, 0, len(byName))
	for _, c := range byName {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func parseJSONCookies(data []byte, now time.Time) ([]Cookie, error) {
	var list []exportedCookie
	if err := json.Unmarshal(data, &list); err != nil {
		var wrapped struct {
			Cookies []exportedCookie `json:"cookies"`
		}
		if werr := json.Unmarshal(data, &wrapped); werr != nil {
			return nil, fmt.Errorf("parsing JSON cookie export: %w", err)
		}
		list = wrapped.Cookies
	}
	out := make([]Cookie, 0, len(list))
	for _, c := range list {
		expiry := c.ExpirationDate
		if expiry == nil {
			expiry = c.Expires
		}
		if expiry != nil && *expiry > 0 && *expiry < float64(now.Unix()) {
			continue
		}
		out = append(out, Cookie{Name: c.Name, Value: c.Value, Domain: c.Domain})
	}
	return out, nil
}

// parseNetscapeCookies reads the seven tab-separated cookies.txt fields:
// domain, subdomains flag, path, secure, expiry, name, value. An expiry of 0
// marks a session cookie. Lines starting with #HttpOnly_ are real cookies.
func parseNetscapeCookies(data string, now time.Time) ([]Cookie, error) {
	var out []Cookie
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimRight(line, "\r")
		line = strings.TrimPrefix(line, "#HttpOnly_")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 7 {
			return nil, errors.New("not a recognized cookie file: expected a JSON export or Netscape cookies.txt")
		}
		expiry, err := strconv.ParseInt(fields[4], 10, 64)
		if err != nil {
			return nil, errors.New("not a recognized cookie file: bad expiry in cookies.txt")
		}
		if expiry > 0 && expiry < now.Unix() {
			continue
		}
		out = append(out, Cookie{Name: fields[5], Value: fields[6], Domain: fields[0]})
	}
	return out, nil
}

// appliesToRedfinHost reports whether a browser would send a cookie with this
// domain to www.redfin.com, the host every CLI request goes to.
func appliesToRedfinHost(domain string) bool {
	d := cookieHost(domain)
	return d == "redfin.com" || d == "www.redfin.com"
}

func cookieHost(domain string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(domain), "."))
}

// CookieHeader renders cookies as a Cookie request header value.
func CookieHeader(cookies []Cookie) string {
	parts := make([]string, 0, len(cookies))
	for _, c := range cookies {
		parts = append(parts, c.Name+"="+c.Value)
	}
	return strings.Join(parts, "; ")
}

// HasChallengeCookies reports whether the export carries the bot-manager
// cookies a browser earns by passing Redfin's JavaScript challenge.
func HasChallengeCookies(cookies []Cookie) bool {
	for _, c := range cookies {
		if c.Name == "_abck" || c.Name == "bm_sz" {
			return true
		}
	}
	return false
}
