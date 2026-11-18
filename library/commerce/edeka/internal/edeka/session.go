// Package edeka holds the hand-written EDEKA web shop logic (maexware shops on
// OXID eShop at <market>.edeka.shop): the self-renewing cookie session, typed
// shop calls and the HTML parsers for product grids, basket, slots and checkout.
package edeka

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/edeka/internal/cliutil"
)

// Cookie is one browser cookie as exported by Playwright, CDP
// (Storage.getCookies / Network.getAllCookies) or this CLI.
type Cookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires"`
	HTTPOnly bool    `json:"httpOnly"`
	Secure   bool    `json:"secure"`
}

// Session is the persisted cookie set of one EDEKA customer plus the market
// the customer shops at and the shop's current form token.
type Session struct {
	Cookies   []Cookie  `json:"cookies"`
	Market    string    `json:"market,omitempty"`
	Stoken    string    `json:"stoken,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RememberCookie is OXID's one-year remember-me cookie. A request that carries
// it gets a fresh logged-in shop session (sid) from the server.
const RememberCookie = "oxid_1"

// ErrReloginNeeded means the remember-me cookie is gone or rejected and the
// customer has to log in to the market's shop in a browser again.
var ErrReloginNeeded = errors.New("EDEKA session expired and could not be renewed: log in to your market's shop (https://<market>.edeka.shop) in a browser again, then run `auth import`")

// ErrNoSession means no session file exists yet.
var ErrNoSession = errors.New("no EDEKA session imported yet: log in to https://<market>.edeka.shop in a browser, then run `auth import --cdp <url>` or `auth import --file cookies.json`")

// SessionPath returns the session file path. EDEKA_SESSION_FILE overrides it
// so one machine can serve several customers (one file per customer).
func SessionPath() (string, error) {
	if p := strings.TrimSpace(os.Getenv("EDEKA_SESSION_FILE")); p != "" {
		return p, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "edeka-pp-cli", "session.json"), nil
}

// LoadSession reads the session file.
func LoadSession(path string) (*Session, error) {
	b, err := os.ReadFile(filepath.Clean(path))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNoSession
	}
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("reading session %s: %w", path, err)
	}
	return &s, nil
}

// Save writes the session file with owner-only permissions. The write goes
// through a uniquely named temp file under cliutil.WithFileLock, so two CLI
// invocations saving at the same time cannot interleave or race the rename.
func (s *Session) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return cliutil.WithFileLock(path, func() error { return s.writeFile(path) })
}

// UpdateMarket stores market as the session's market and drops the cached
// form token. It re-reads the session file under the lock first, so cookies
// another CLI run rotated in the meantime are kept rather than overwritten
// with a stale copy. A missing session file returns ErrNoSession.
func UpdateMarket(path, market string) (*Session, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil, ErrNoSession
	}
	var out *Session
	err := cliutil.WithFileLock(path, func() error {
		s, err := LoadSession(path)
		if err != nil {
			return err
		}
		s.Market = market
		s.Stoken = ""
		out = s
		return s.writeFile(path)
	})
	return out, err
}

// SaveMerged re-reads the session file under the lock, applies this run's
// Set-Cookie updates to it and writes it back, so two CLI runs saving at the
// same time keep each other's rotated cookies. market and stoken overwrite
// the stored values when set.
func (s *Session) SaveMerged(path string, updates []setCookieUpdate, market, stoken string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return cliutil.WithFileLock(path, func() error {
		cur := s
		if disk, err := LoadSession(path); err == nil {
			disk.ApplySetCookies(updates)
			if market != "" {
				disk.Market = market
			}
			if stoken != "" {
				disk.Stoken = stoken
			}
			cur = disk
		}
		return cur.writeFile(path)
	})
}

// writeFile writes the session atomically (caller holds the lock).
func (s *Session) writeFile(path string) error {
	s.UpdatedAt = time.Now().UTC()
	b, err := json.MarshalIndent(s, "", " ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil && runtime.GOOS != "windows" {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// ParseCookieExport accepts a JSON array of cookies, an object with a
// "cookies" array (CDP, Playwright storage state) or a session file.
func ParseCookieExport(data []byte) ([]Cookie, error) {
	data = []byte(strings.TrimSpace(string(data)))
	var arr []Cookie
	if err := json.Unmarshal(data, &arr); err == nil {
		return FilterEDEKA(arr), nil
	}
	var obj struct {
		Cookies []Cookie `json:"cookies"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("cookie export is neither a JSON array nor an object with \"cookies\": %w", err)
	}
	return FilterEDEKA(obj.Cookies), nil
}

// FilterEDEKA keeps cookies of edeka.shop hosts only.
func FilterEDEKA(in []Cookie) []Cookie {
	out := make([]Cookie, 0, len(in))
	for _, c := range in {
		if IsShopHost(strings.TrimPrefix(c.Domain, ".")) {
			if c.Path == "" {
				c.Path = "/"
			}
			out = append(out, c)
		}
	}
	return out
}

// IsShopHost reports whether h is edeka.shop or one of its subdomains.
func IsShopHost(h string) bool {
	h = strings.ToLower(strings.TrimSuffix(h, "."))
	return h == "edeka.shop" || strings.HasSuffix(h, ".edeka.shop")
}

// Has reports whether a non-expired cookie with that name exists for host
// ("" matches any host).
func (s *Session) Has(name, host string) bool {
	now := float64(time.Now().Unix())
	for _, c := range s.Cookies {
		if c.Name == name && (c.Expires <= 0 || c.Expires > now) && (host == "" || domainMatches(host, c.Domain)) {
			return true
		}
	}
	return false
}

// Markets lists the market hosts the session has a remember-me cookie for.
func (s *Session) Markets() []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range s.Cookies {
		d := strings.TrimPrefix(strings.ToLower(c.Domain), ".")
		if c.Name == RememberCookie && d != "edeka.shop" && IsShopHost(d) && !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	return out
}

// RememberExpiry returns when the remember-me cookie for host expires (zero when unknown).
func (s *Session) RememberExpiry(host string) time.Time {
	for _, c := range s.Cookies {
		if c.Name == RememberCookie && domainMatches(host, c.Domain) && c.Expires > 0 {
			return time.Unix(int64(c.Expires), 0).UTC()
		}
	}
	return time.Time{}
}

// Jar builds an http.CookieJar holding the session cookies.
func (s *Session) Jar() (*cookiejar.Jar, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	byURL := map[string][]*http.Cookie{}
	now := time.Now()
	for _, c := range s.Cookies {
		if !validCookieValue(c.Value) {
			continue
		}
		if c.Expires > 0 && time.Unix(int64(c.Expires), 0).Before(now) {
			continue
		}
		host := strings.TrimPrefix(c.Domain, ".")
		path := c.Path
		if path == "" {
			path = "/"
		}
		hc := &http.Cookie{Name: c.Name, Value: c.Value, Path: path, Secure: c.Secure, HttpOnly: c.HTTPOnly} // #nosec G124 -- client-side jar entry built from the saved session; Secure/HttpOnly are copied from the source cookie and SameSite only applies to server Set-Cookie headers
		if strings.HasPrefix(c.Domain, ".") {
			hc.Domain = c.Domain
		}
		if c.Expires > 0 {
			hc.Expires = time.Unix(int64(c.Expires), 0)
		}
		u := "https://" + host + path
		byURL[u] = append(byURL[u], hc)
	}
	for raw, cs := range byURL {
		u, err := url.Parse(raw)
		if err != nil {
			continue
		}
		jar.SetCookies(u, cs)
	}
	return jar, nil
}

// HostCookies returns the session cookies that apply to host as name/value pairs.
func (s *Session) HostCookies(host string) []*http.Cookie {
	var out []*http.Cookie
	now := float64(time.Now().Unix())
	for _, c := range s.Cookies {
		if !domainMatches(host, c.Domain) || !validCookieValue(c.Value) || (c.Expires > 0 && c.Expires < now) {
			continue
		}
		out = append(out, &http.Cookie{Name: c.Name, Value: c.Value, Path: "/"}) // #nosec G124 -- request cookie for the Cookie header, not a server Set-Cookie; these attributes do not apply
	}
	return out
}

// setCookieUpdate is one raw Set-Cookie header seen during this run, kept
// with the host it came from.
type setCookieUpdate struct {
	Host   string
	Cookie *http.Cookie
}

// recordingTransport remembers every raw Set-Cookie header, including those of
// redirect hops http.Client follows itself, so expiries and deletions can be
// written back to the session (cookiejar.Jar has no way to read them back).
type recordingTransport struct {
	next http.RoundTripper
	mu   sync.Mutex
	seen []setCookieUpdate
}

func (t *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.next.RoundTrip(req)
	if resp != nil {
		if cs := resp.Cookies(); len(cs) > 0 {
			t.mu.Lock()
			for _, c := range cs {
				t.seen = append(t.seen, setCookieUpdate{Host: req.URL.Hostname(), Cookie: c})
			}
			t.mu.Unlock()
		}
	}
	return resp, err
}

func (t *recordingTransport) snapshot() []setCookieUpdate {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]setCookieUpdate, len(t.seen))
	copy(out, t.seen)
	return out
}

// ApplySetCookies writes Set-Cookie updates into the session: new values,
// expiries, new cookies (stored host-only for the host that set them) and
// deletions.
func (s *Session) ApplySetCookies(updates []setCookieUpdate) {
	for _, u := range updates {
		c := u.Cookie
		deleted := c.MaxAge < 0 || (c.MaxAge == 0 && !c.Expires.IsZero() && c.Expires.Before(time.Now())) || c.Value == "deleted"
		exp := setCookieExpiry(c)
		domain := u.Host
		if c.Domain != "" {
			domain = "." + strings.TrimPrefix(c.Domain, ".")
		}
		if !IsShopHost(strings.TrimPrefix(domain, ".")) {
			continue // never widen a cookie beyond edeka.shop
		}
		matched := false
		for i := 0; i < len(s.Cookies); i++ {
			sc := &s.Cookies[i]
			if sc.Name != c.Name || !sameCookieScope(sc.Domain, domain) {
				continue
			}
			matched = true
			if deleted {
				s.Cookies = append(s.Cookies[:i], s.Cookies[i+1:]...)
				i--
				continue
			}
			sc.Value = c.Value
			sc.Expires = exp
		}
		if !matched && !deleted {
			path := c.Path
			if path == "" {
				path = "/"
			}
			s.Cookies = append(s.Cookies, Cookie{Name: c.Name, Value: c.Value, Domain: domain, Path: path, Expires: exp, HTTPOnly: c.HttpOnly, Secure: true})
		}
	}
}

// setCookieExpiry resolves a Set-Cookie's expiry as a Unix timestamp; 0 means
// a session cookie. Max-Age wins over Expires (RFC 6265 5.3).
func setCookieExpiry(c *http.Cookie) float64 {
	if c.MaxAge > 0 {
		return float64(time.Now().Add(time.Duration(c.MaxAge) * time.Second).Unix())
	}
	if c.MaxAge == 0 && !c.Expires.IsZero() {
		return float64(c.Expires.Unix())
	}
	return 0
}

func sameCookieScope(a, b string) bool {
	return strings.EqualFold(strings.TrimPrefix(a, "."), strings.TrimPrefix(b, ".")) && strings.HasPrefix(a, ".") == strings.HasPrefix(b, ".")
}

// domainMatches reports whether a cookie domain applies to host.
func domainMatches(host, domain string) bool {
	d := strings.TrimPrefix(strings.ToLower(domain), ".")
	host = strings.ToLower(host)
	if !strings.HasPrefix(domain, ".") {
		return host == d
	}
	return host == d || strings.HasSuffix(host, "."+d)
}

func validCookieValue(v string) bool { return !strings.ContainsAny(v, "\"; \\,") }
