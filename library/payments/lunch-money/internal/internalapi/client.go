// Package internalapi wraps Lunch Money's undocumented web-UI backend at
// api.lunchmoney.app. It is fully local — never call a public registry, never
// embed user data in source. The endpoints captured here are reverse-engineered
// from the web UI (see internal/internalapi/captures/*.md) and may change at
// any time without notice.
//
// Auth model: a long-lived cookie issued by my.lunchmoney.app's login flow
// produces short-lived JWT access cookies. We persist the cookie jar to disk
// and, on a 401 with "Access token expired", call /auth/token/refresh and
// retry the request once.
package internalapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	DefaultBaseURL    = "https://api.lunchmoney.app"
	OriginHeader      = "https://my.lunchmoney.app"
	EnvInternalCookie = "LUNCHMONEY_INTERNAL_COOKIE"
)

// Client speaks the internal Lunch Money API using cookie auth.
//
// Concurrency: methods are safe for concurrent use; the refresh path is
// serialized by mu to avoid stampeding /auth/token/refresh on simultaneous
// expirations.
type Client struct {
	BaseURL    string
	HTTP       *http.Client
	Jar        http.CookieJar
	cookiePath string
	envCookie  bool

	mu         sync.Mutex
	refreshing bool
}

// New builds a Client. cookiePath is where the persistent cookie jar lives.
// Passing empty cookiePath disables persistence (useful for tests).
func New(cookiePath string) (*Client, error) {
	jar, err := cookiejar.New(&cookiejar.Options{})
	if err != nil {
		return nil, fmt.Errorf("cookiejar: %w", err)
	}
	c := &Client{
		BaseURL:    DefaultBaseURL,
		Jar:        jar,
		cookiePath: cookiePath,
		HTTP: &http.Client{
			Jar:     jar,
			Timeout: 30 * time.Second,
		},
	}
	if cookiePath != "" {
		if err := c.loadCookies(); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("loading cookies: %w", err)
		}
	}
	if cookie := strings.TrimSpace(os.Getenv(EnvInternalCookie)); cookie != "" {
		// PATCH: Allow ephemeral internal API auth from the environment for CI
		// and one-off shells without copying the session secret into the jar.
		c.setCookieString(cookie, false)
		c.envCookie = true
	}
	return c, nil
}

// DefaultCookiePath returns the canonical location for the persistent cookie jar.
func DefaultCookiePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "lunch-money-pp-cli", "internal-cookies.json")
}

// PersistentCookie is the on-disk shape — one entry per domain cookie.
type PersistentCookie struct {
	Name    string    `json:"name"`
	Value   string    `json:"value"`
	Domain  string    `json:"domain"`
	Path    string    `json:"path"`
	Expires time.Time `json:"expires,omitempty"`
	Secure  bool      `json:"secure"`
}

func (c *Client) loadCookies() error {
	data, err := os.ReadFile(c.cookiePath)
	if err != nil {
		return err
	}
	var jar []PersistentCookie
	if err := json.Unmarshal(data, &jar); err != nil {
		return fmt.Errorf("parsing cookie file: %w", err)
	}
	// Always reload onto the parent domain (lunchmoney.app) so that subsequent
	// Set-Cookie responses from refresh merge by name. Go's cookiejar.Cookies()
	// strips Domain when returning, so we can't trust the persisted Domain
	// field to be correct — force lunchmoney.app on load.
	parentURL, _ := url.Parse("https://lunchmoney.app")
	httpCookies := make([]*http.Cookie, 0, len(jar))
	for _, pc := range jar {
		domain := pc.Domain
		if domain == "" {
			domain = "lunchmoney.app"
		}
		path := pc.Path
		if path == "" {
			path = "/"
		}
		httpCookies = append(httpCookies, &http.Cookie{
			Name: pc.Name, Value: pc.Value, Domain: domain, Path: path,
			Expires: pc.Expires, Secure: true,
		})
	}
	c.Jar.SetCookies(parentURL, httpCookies)
	return nil
}

func (c *Client) saveCookies() error {
	if c.cookiePath == "" || c.envCookie {
		return nil
	}
	u, _ := url.Parse(c.BaseURL)
	cookies := c.Jar.Cookies(u)
	out := make([]PersistentCookie, 0, len(cookies))
	for _, ck := range cookies {
		out = append(out, PersistentCookie{
			Name: ck.Name, Value: ck.Value, Domain: ck.Domain, Path: ck.Path,
			Expires: ck.Expires, Secure: ck.Secure,
		})
	}
	if err := os.MkdirAll(filepath.Dir(c.cookiePath), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	tmp := c.cookiePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, c.cookiePath)
}

// SetCookieString accepts a raw `Cookie:` header value (e.g., copied from
// the browser DevTools) and replaces the cookie jar contents.
//
// We replace (not append) and use the same Domain ("lunchmoney.app") that the
// server's Set-Cookie response uses on /auth/token/refresh. Otherwise the jar
// keeps two cookies with the same name (host-only on api.lunchmoney.app from
// the manual seed, and Domain=lunchmoney.app from refresh), the server reads
// whichever it sees first, and retries after refresh still 401.
func (c *Client) SetCookieString(s string) {
	c.envCookie = false
	c.setCookieString(s, true)
}

func (c *Client) setCookieString(s string, persist bool) {
	// Start with a fresh jar.
	jar, err := cookiejar.New(&cookiejar.Options{})
	if err == nil {
		c.Jar = jar
		c.HTTP.Jar = jar
	}
	// Use the parent domain so Set-Cookie responses (Domain=lunchmoney.app)
	// merge with these entries instead of stacking alongside them.
	parentURL, _ := url.Parse("https://lunchmoney.app")
	cookies := make([]*http.Cookie, 0)
	for _, kv := range strings.Split(normalizeCookieHeader(s), ";") {
		kv = strings.TrimSpace(kv)
		if kv == "" {
			continue
		}
		idx := strings.IndexByte(kv, '=')
		if idx == -1 {
			continue
		}
		cookies = append(cookies, &http.Cookie{
			Name:   kv[:idx],
			Value:  kv[idx+1:],
			Domain: "lunchmoney.app",
			Path:   "/",
			Secure: true,
		})
	}
	c.Jar.SetCookies(parentURL, cookies)
	if persist {
		_ = c.saveCookies()
	}
}

func normalizeCookieHeader(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 7 && strings.EqualFold(s[:7], "Cookie:") {
		return strings.TrimSpace(s[7:])
	}
	return s
}

// HasSession returns true if at least one cookie is present for the API host.
func (c *Client) HasSession() bool {
	u, _ := url.Parse(c.BaseURL)
	return len(c.Jar.Cookies(u)) > 0
}

// Do performs an HTTP request against the internal API with automatic JSON body
// encoding, origin header, and on-401 token refresh + retry. resp body is
// decoded into out unless out is nil. Returns the http status code.
func (c *Client) Do(method, path string, body any, out any) (int, error) {
	status, _, err := c.doWithBody(method, path, body, out)
	return status, err
}

// DoRaw is like Do but returns the raw response body bytes instead of decoding.
func (c *Client) DoRaw(method, path string, body any) (int, []byte, error) {
	return c.doWithBody(method, path, body, nil)
}

func (c *Client) doWithBody(method, path string, body any, out any) (int, []byte, error) {
	var bodyData []byte
	contentType := ""
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return 0, nil, fmt.Errorf("encoding body: %w", err)
		}
		bodyData = data
		contentType = "application/json"
	}
	return c.doWithEncodedBody(method, path, bodyData, contentType, out)
}

// DoMultipartRaw sends one multipart file plus optional string fields. It is
// intentionally narrow because the captured internal file endpoints only use a
// single "file" part plus small selector fields.
func (c *Client) DoMultipartRaw(method, path string, fields map[string]string, fileField, filename string, fileContent []byte, mime string) (int, []byte, error) {
	bodyData, contentType, err := encodeMultipart(fields, fileField, filename, fileContent, mime)
	if err != nil {
		return 0, nil, err
	}
	return c.doWithEncodedBody(method, path, bodyData, contentType, nil)
}

func (c *Client) DoMultipart(method, path string, fields map[string]string, fileField, filename string, fileContent []byte, mime string, out any) (int, error) {
	bodyData, contentType, err := encodeMultipart(fields, fileField, filename, fileContent, mime)
	if err != nil {
		return 0, err
	}
	status, _, err := c.doWithEncodedBody(method, path, bodyData, contentType, out)
	return status, err
}

// PATCH: Support captured internal multipart endpoints that the generated
// public client cannot reach because they use cookie auth on api.lunchmoney.app.
func encodeMultipart(fields map[string]string, fileField, filename string, fileContent []byte, mime string) ([]byte, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			_ = w.Close()
			return nil, "", fmt.Errorf("writing multipart field %q: %w", k, err)
		}
	}
	if fileField != "" {
		if filename == "" {
			filename = "upload"
		}
		if mime == "" {
			mime = http.DetectContentType(fileContent)
		}
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, escapeMultipartQuotes(fileField), escapeMultipartQuotes(filename)))
		h.Set("Content-Type", mime)
		part, err := w.CreatePart(h)
		if err != nil {
			_ = w.Close()
			return nil, "", fmt.Errorf("creating multipart file field %q: %w", fileField, err)
		}
		if _, err := part.Write(fileContent); err != nil {
			_ = w.Close()
			return nil, "", fmt.Errorf("writing multipart file field %q: %w", fileField, err)
		}
	}
	if err := w.Close(); err != nil {
		return nil, "", fmt.Errorf("finalizing multipart body: %w", err)
	}
	return buf.Bytes(), w.FormDataContentType(), nil
}

func escapeMultipartQuotes(s string) string {
	return strings.NewReplacer("\\", "\\\\", `"`, "\\\"").Replace(s)
}

func (c *Client) doWithEncodedBody(method, path string, bodyData []byte, contentType string, out any) (int, []byte, error) {
	status, raw, err := c.doEncoded(method, path, bodyData, contentType)
	if err != nil {
		return status, raw, err
	}
	if status == 401 && tryRefresh(raw) {
		if err := c.refresh(); err != nil {
			return status, raw, fmt.Errorf("token refresh: %w", err)
		}
		// PATCH(retry-non-idempotent-internalapi): mirror the public-client
		// 5xx gate. Most 401s fire before body processing so a POST/PATCH
		// retry is usually safe, but if a server ever processes-then-401s
		// (e.g. token expires mid-handler) the retry would double-commit.
		// Limit auto-retry to idempotent methods; non-idempotent callers
		// surface the 401 and let the user re-run if appropriate.
		if !isIdempotentMethodInternal(method) {
			return status, raw, &APIError{Status: status, Body: raw, Path: path, Method: method}
		}
		status, raw, err = c.doEncoded(method, path, bodyData, contentType)
		if err != nil {
			return status, raw, err
		}
	}
	if status >= 400 {
		return status, raw, &APIError{Status: status, Body: raw, Path: path, Method: method}
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return status, raw, fmt.Errorf("decoding %s %s: %w", method, path, err)
		}
	}
	return status, raw, nil
}

func (c *Client) doEncoded(method, path string, bodyData []byte, contentType string) (int, []byte, error) {
	var bodyReader io.Reader
	if len(bodyData) > 0 {
		bodyReader = bytes.NewReader(bodyData)
	}
	url := c.BaseURL + path
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Origin", OriginHeader)
	req.Header.Set("Referer", OriginHeader+"/")
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	_ = c.saveCookies()
	return resp.StatusCode, raw, nil
}

// isIdempotentMethodInternal mirrors the public client's isIdempotentMethod;
// kept local to this package to avoid a cross-package import for one switch.
func isIdempotentMethodInternal(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}

func tryRefresh(raw []byte) bool {
	// Server returns either "Access token expired." or "Access token does not exist."
	// depending on whether the JWT is past exp or the access cookie is missing
	// entirely (e.g., user cleared cookies but refresh cookie still valid).
	return bytes.Contains(raw, []byte("Access token expired")) ||
		bytes.Contains(raw, []byte("Access token does not exist"))
}

func (c *Client) refresh() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.refreshing {
		return errors.New("refresh already in progress")
	}
	c.refreshing = true
	defer func() { c.refreshing = false }()
	status, _, err := c.doEncoded("POST", "/auth/token/refresh", []byte("{}"), "application/json")
	if err != nil {
		return err
	}
	if status >= 400 {
		return fmt.Errorf("refresh returned %d — re-run `lunch-money-pp-cli internal auth set-cookie` with a fresh browser cookie", status)
	}
	return nil
}

// APIError is the structured error returned for >=400 responses.
type APIError struct {
	Method string
	Path   string
	Status int
	Body   []byte
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s %s -> HTTP %d: %s", e.Method, e.Path, e.Status, strings.TrimSpace(string(e.Body)))
}
