package client

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type persistedCookieJar struct {
	path    string
	nowFunc func() time.Time

	mu      sync.Mutex
	cookies []http.Cookie
}

type persistedCookie struct {
	Name       string    `json:"name"`
	Value      string    `json:"value"`
	Domain     string    `json:"domain,omitempty"`
	Path       string    `json:"path,omitempty"`
	Expires    time.Time `json:"expires,omitempty"`
	Secure     bool      `json:"secure,omitempty"`
	HttpOnly   bool      `json:"http_only,omitempty"`
	Persistent bool      `json:"persistent,omitempty"`
	HostOnly   bool      `json:"host_only,omitempty"`
}

func newPersistedCookieJar(path string) (*persistedCookieJar, error) {
	jar := &persistedCookieJar{
		path:    path,
		nowFunc: time.Now,
	}
	if err := jar.load(); err != nil {
		return nil, err
	}
	return jar, nil
}

func (j *persistedCookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	if u == nil || len(cookies) == 0 {
		return
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	now := j.nowFunc()
	for _, cookie := range cookies {
		if cookie == nil || cookie.Name == "" {
			continue
		}
		normalized := normalizeCookie(u, cookie, now)
		j.deleteCookieLocked(normalized)
		if !cookieExpired(normalized, now) {
			j.cookies = append(j.cookies, normalized)
		}
	}

	j.compactLocked(now)
	_ = j.saveLocked()
}

func (j *persistedCookieJar) Cookies(u *url.URL) []*http.Cookie {
	if u == nil {
		return nil
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	now := j.nowFunc()
	j.compactLocked(now)

	var out []*http.Cookie
	for _, cookie := range j.cookies {
		if !cookieMatchesURL(cookie, u, now) {
			continue
		}
		copyCookie := cookie
		out = append(out, &copyCookie)
	}
	return out
}

func (j *persistedCookieJar) ClearDomainSuffix(domainSuffix string) error {
	domainSuffix = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(domainSuffix, ".")))
	if domainSuffix == "" {
		return nil
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	filtered := j.cookies[:0]
	for _, cookie := range j.cookies {
		domain := strings.ToLower(strings.TrimPrefix(cookie.Domain, "."))
		if domain == domainSuffix || strings.HasSuffix(domain, "."+domainSuffix) {
			continue
		}
		filtered = append(filtered, cookie)
	}
	j.cookies = filtered
	return j.saveLocked()
}

func (j *persistedCookieJar) load() error {
	if j.path == "" {
		return nil
	}
	data, err := os.ReadFile(j.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var stored []persistedCookie
	if err := json.Unmarshal(data, &stored); err != nil {
		return err
	}
	now := j.nowFunc()
	j.cookies = j.cookies[:0]
	for _, item := range stored {
		cookie := http.Cookie{
			Name:     item.Name,
			Value:    item.Value,
			Domain:   item.Domain,
			Path:     item.Path,
			Expires:  item.Expires,
			Secure:   item.Secure,
			HttpOnly: item.HttpOnly,
		}
		if !cookieExpired(cookie, now) {
			j.cookies = append(j.cookies, cookie)
		}
	}
	return nil
}

func (j *persistedCookieJar) saveLocked() error {
	if j.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(j.path), 0o700); err != nil {
		return err
	}
	stored := make([]persistedCookie, 0, len(j.cookies))
	for _, cookie := range j.cookies {
		stored = append(stored, persistedCookie{
			Name:       cookie.Name,
			Value:      cookie.Value,
			Domain:     cookie.Domain,
			Path:       cookie.Path,
			Expires:    cookie.Expires,
			Secure:     cookie.Secure,
			HttpOnly:   cookie.HttpOnly,
			Persistent: !cookie.Expires.IsZero(),
			HostOnly:   !strings.HasPrefix(cookie.Domain, "."),
		})
	}
	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(j.path, data, 0o600)
}

func (j *persistedCookieJar) deleteCookieLocked(target http.Cookie) {
	filtered := j.cookies[:0]
	for _, cookie := range j.cookies {
		if sameCookie(cookie, target) {
			continue
		}
		filtered = append(filtered, cookie)
	}
	j.cookies = filtered
}

func (j *persistedCookieJar) compactLocked(now time.Time) {
	filtered := j.cookies[:0]
	for _, cookie := range j.cookies {
		if cookieExpired(cookie, now) {
			continue
		}
		filtered = append(filtered, cookie)
	}
	j.cookies = filtered
}

func normalizeCookie(u *url.URL, cookie *http.Cookie, now time.Time) http.Cookie {
	out := *cookie
	if out.Domain == "" {
		out.Domain = strings.ToLower(u.Hostname())
	} else {
		out.Domain = strings.ToLower(strings.TrimPrefix(out.Domain, "."))
	}
	if out.Path == "" {
		out.Path = "/"
	}
	if out.MaxAge < 0 {
		out.Expires = now.Add(-time.Second)
	}
	return out
}

func sameCookie(a, b http.Cookie) bool {
	return strings.EqualFold(a.Name, b.Name) &&
		strings.EqualFold(strings.TrimPrefix(a.Domain, "."), strings.TrimPrefix(b.Domain, ".")) &&
		a.Path == b.Path
}

func cookieExpired(cookie http.Cookie, now time.Time) bool {
	return !cookie.Expires.IsZero() && !cookie.Expires.After(now)
}

func cookieMatchesURL(cookie http.Cookie, u *url.URL, now time.Time) bool {
	if cookieExpired(cookie, now) {
		return false
	}
	host := strings.ToLower(u.Hostname())
	domain := strings.ToLower(strings.TrimPrefix(cookie.Domain, "."))
	if domain == "" {
		return false
	}
	if host != domain && !strings.HasSuffix(host, "."+domain) {
		return false
	}
	if cookie.Secure && u.Scheme != "https" {
		return false
	}
	path := cookie.Path
	if path == "" {
		path = "/"
	}
	requestPath := u.EscapedPath()
	if requestPath == "" {
		requestPath = "/"
	}
	if !strings.HasPrefix(requestPath, path) {
		return false
	}
	return true
}
