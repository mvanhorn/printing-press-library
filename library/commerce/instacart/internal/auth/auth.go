package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/browserutils/kooky"
	_ "github.com/browserutils/kooky/browser/chrome"

	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/config"
)

// InstacartDomain and InstacartHost are the US defaults. Request-building
// code uses Host()/BaseURL() so a regional site can be selected (host.go).
const (
	InstacartDomain = DefaultHost
	InstacartHost   = "https://" + DefaultHost
)

var RequiredCookieNames = []string{
	"__Host-instacart_sid",
	"_instacart_session_id",
	"device_uuid",
}

var UsefulCookieNames = []string{
	"__Host-instacart_sid",
	"_instacart_session_id",
	"device_uuid",
	"build_sha",
	"known_visitor",
	"X-IC-bcx",
	"forterToken",
	"ahoy_visit",
	"ahoy_visitor",
}

type Session struct {
	Cookies   []Cookie  `json:"cookies"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
}

type Cookie struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Domain   string    `json:"domain"`
	Path     string    `json:"path"`
	Expires  time.Time `json:"expires,omitempty"`
	HTTPOnly bool      `json:"http_only"`
	Secure   bool      `json:"secure"`
}

func sessionPath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "session.json"), nil
}

func LoadSession() (*Session, error) {
	path, err := sessionPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("not logged in (run `instacart auth login`)")
	}
	if err != nil {
		return nil, fmt.Errorf("read session: %w", err)
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse session: %w", err)
	}
	return &s, nil
}

func (s *Session) Save() error {
	path, err := sessionPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func ClearSession() error {
	path, err := sessionPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// CookieHeader builds a Cookie header string for use in HTTP requests.
func (s *Session) CookieHeader() string {
	var parts []string
	for _, c := range s.Cookies {
		parts = append(parts, c.Name+"="+c.Value)
	}
	return strings.Join(parts, "; ")
}

// ApplyToRequest adds cookies to an http.Request.
func (s *Session) ApplyToRequest(req *http.Request) {
	for _, c := range s.Cookies {
		req.AddCookie(&http.Cookie{Name: c.Name, Value: c.Value})
	}
}

// ImportFromChrome reads the Instacart cookies from the user's Chrome cookie
// store via kooky and returns a Session containing the ones we care about.
func ImportFromChrome() (*Session, error) {
	stores := kooky.FindAllCookieStores()
	if len(stores) == 0 {
		return nil, errors.New("no browser cookie stores found by kooky; make sure Chrome is installed")
	}

	u, _ := url.Parse(BaseURL())
	wanted := make(map[string]bool, len(UsefulCookieNames))
	for _, n := range UsefulCookieNames {
		wanted[n] = true
	}

	found := make(map[string]Cookie)
	var lastErr error

	for _, st := range stores {
		if st == nil {
			continue
		}
		// Prefer Chrome stores; kooky will still try others if Chrome is locked.
		browser := strings.ToLower(st.Browser())
		if browser != "chrome" && browser != "chromium" && browser != "brave" && browser != "edge" {
			continue
		}
		cookies, err := st.ReadCookies(kooky.DomainHasSuffix(CookieDomain()))
		if err != nil {
			lastErr = err
			continue
		}
		for _, c := range cookies {
			if !wanted[c.Name] {
				continue
			}
			// Keep the most recently seen one.
			found[c.Name] = Cookie{
				Name:     c.Name,
				Value:    c.Value,
				Domain:   c.Domain,
				Path:     c.Path,
				Expires:  c.Expires,
				HTTPOnly: c.HttpOnly,
				Secure:   c.Secure,
			}
		}
	}
	_ = u

	if len(found) == 0 {
		if lastErr != nil {
			return nil, fmt.Errorf("kooky could not read Chrome cookies: %w (quit Chrome and retry?)", lastErr)
		}
		return nil, fmt.Errorf("no Instacart cookies found in Chrome (are you logged in at %s?)", CookieDomain())
	}

	// Require at least one of the session cookies.
	hasSession := false
	for _, name := range RequiredCookieNames {
		if _, ok := found[name]; ok {
			hasSession = true
			break
		}
	}
	if !hasSession {
		return nil, fmt.Errorf("found Instacart cookies but no session cookie (%s) -- please log in at %s in Chrome, then re-run `instacart auth login`", strings.Join(RequiredCookieNames, ", "), BaseURL())
	}

	s := &Session{Source: "chrome", CreatedAt: time.Now().UTC()}
	for _, c := range found {
		s.Cookies = append(s.Cookies, c)
	}
	return s, nil
}

// ImportFromFile reads a cookies.json file in the browser-use export format
// (an array of {name, value, domain, ...}) or a wrapper {cookies: [...]}
// and extracts the Instacart cookies we care about.
func ImportFromFile(path string) (*Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	type rawCookie struct {
		Name     string  `json:"name"`
		Value    string  `json:"value"`
		Domain   string  `json:"domain"`
		Path     string  `json:"path"`
		Expires  float64 `json:"expires"`
		HTTPOnly bool    `json:"httpOnly"`
		Secure   bool    `json:"secure"`
	}

	// Try both shapes: bare array or wrapped.
	var cookies []rawCookie
	if err := json.Unmarshal(data, &cookies); err != nil {
		var wrapped struct {
			Cookies []rawCookie `json:"cookies"`
		}
		if err2 := json.Unmarshal(data, &wrapped); err2 != nil {
			return nil, fmt.Errorf("parse %s: not a cookie array or {cookies:[]}: %w", path, err)
		}
		cookies = wrapped.Cookies
	}

	wanted := make(map[string]bool, len(UsefulCookieNames))
	for _, n := range UsefulCookieNames {
		wanted[n] = true
	}

	s := &Session{Source: "file", CreatedAt: time.Now().UTC()}
	for _, c := range cookies {
		if !cookieDomainMatches(c.Domain) {
			continue
		}
		if !wanted[c.Name] {
			continue
		}
		var exp time.Time
		if c.Expires > 0 {
			exp = time.Unix(int64(c.Expires), 0)
		}
		s.Cookies = append(s.Cookies, Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  exp,
			HTTPOnly: c.HTTPOnly,
			Secure:   c.Secure,
		})
	}
	if len(s.Cookies) == 0 {
		return nil, fmt.Errorf("no recognized Instacart cookies found in %s", path)
	}
	hasSession := false
	for _, c := range s.Cookies {
		for _, n := range RequiredCookieNames {
			if c.Name == n {
				hasSession = true
			}
		}
	}
	if !hasSession {
		return nil, fmt.Errorf("file %s missing session cookie (one of %s)", path, strings.Join(RequiredCookieNames, ", "))
	}
	return s, nil
}

// ImportFromHeader parses a raw "Cookie: foo=bar; baz=qux" header string
// (the user can copy this from devtools). This is the fallback for users
// whose browser kooky cannot read.
func ImportFromHeader(header string) (*Session, error) {
	header = strings.TrimSpace(header)
	header = strings.TrimPrefix(header, "Cookie:")
	header = strings.TrimPrefix(header, "cookie:")
	header = strings.TrimSpace(header)
	if header == "" {
		return nil, errors.New("empty cookie header")
	}

	wanted := make(map[string]bool, len(UsefulCookieNames))
	for _, n := range UsefulCookieNames {
		wanted[n] = true
	}

	s := &Session{Source: "paste", CreatedAt: time.Now().UTC()}
	pairs := strings.Split(header, ";")
	for _, p := range pairs {
		p = strings.TrimSpace(p)
		eq := strings.Index(p, "=")
		if eq < 1 {
			continue
		}
		name := strings.TrimSpace(p[:eq])
		val := strings.TrimSpace(p[eq+1:])
		if !wanted[name] {
			continue
		}
		s.Cookies = append(s.Cookies, Cookie{
			Name:     name,
			Value:    val,
			Domain:   Host(),
			Path:     "/",
			Secure:   true,
			HTTPOnly: strings.HasPrefix(name, "__Host-") || strings.HasPrefix(name, "_instacart"),
		})
	}
	if len(s.Cookies) == 0 {
		return nil, errors.New("no recognized Instacart cookies in header")
	}
	hasSession := false
	for _, c := range s.Cookies {
		for _, n := range RequiredCookieNames {
			if c.Name == n {
				hasSession = true
			}
		}
	}
	if !hasSession {
		return nil, fmt.Errorf("cookie header missing session cookie (one of %s)", strings.Join(RequiredCookieNames, ", "))
	}
	return s, nil
}
