package traveloka

import (
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/cliutil"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const origin = "https://www.traveloka.com"
const maxSessionBytes = 16 << 20

var operations = map[string]string{
	"/api/v2/airport/search-nexus":        "flightDiscovery",
	"/api/v1/hotel/autocomplete":          "accomContent",
	"/api/v2/hotel/autocomplete/features": "accomContent",
	"/api/v2/flight/search/initial":       "flight",
	"/api/v2/flight/search/poll":          "flight",
	"/api/v2/flight/search/redirection":   "flight",
	"/api/v2/hotel/searchList":            "accomSearch",
	"/api/v2/hotel/search/rooms":          "accomRoom",
}
var importedHeaders = map[string]bool{
	"user-agent": true, "www-app-version": true, "t-a-v": true, "tv-clientsessionid": true, "tv-mcc-id": true, "x-did": true, "x-client-interface": true,
	"x-request-token": true, "x-csrf-token": true, "x-xsrf-token": true, "csrf-token": true,
	"sec-ch-ua": true, "sec-ch-ua-mobile": true, "sec-ch-ua-platform": true,
}

type scopedCookie struct {
	Name     string      `json:"name"`
	Value    string      `json:"value"`
	Domain   string      `json:"domain"`
	Path     string      `json:"path"`
	Expires  json.Number `json:"expires"`
	HTTPOnly bool        `json:"httpOnly"`
	Secure   bool        `json:"secure"`
	SameSite string      `json:"sameSite"`
}
type requestCapture struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}
type requestProfile struct {
	Headers map[string]string `json:"headers"`
	Body    map[string]any    `json:"body"`
}
type privateSession struct {
	Version    int                       `json:"version"`
	CapturedAt string                    `json:"captured_at"`
	Cookies    []scopedCookie            `json:"cookies"`
	Profiles   map[string]requestProfile `json:"profiles"`
}
type SessionInfo struct {
	Path       string `json:"path"`
	CapturedAt string `json:"captured_at"`
	Cookies    int    `json:"cookies"`
	Profiles   int    `json:"profiles"`
}

func validDomain(domain string) bool {
	d := strings.TrimPrefix(domain, ".")
	return d == "www.traveloka.com" || d == "traveloka.com"
}
func validateCookie(c scopedCookie) error {
	if !validDomain(c.Domain) {
		return apiError("INVALID_INPUT", "session contains a foreign cookie domain", 0, false)
	}
	if c.Name == "" || strings.ContainsAny(c.Name, "\r\n;= ") || strings.ContainsAny(c.Value, "\r\n;") || !strings.HasPrefix(c.Path, "/") || strings.ContainsAny(c.Path, "\r\n") {
		return apiError("INVALID_INPUT", "session contains an invalid cookie", 0, false)
	}
	if c.Expires != "" {
		n, err := strconv.ParseFloat(string(c.Expires), 64)
		if err != nil || n < -1 || (n > -1 && n < 0) || n > 253402300799 {
			return apiError("INVALID_INPUT", "session contains an invalid cookie expiry", 0, false)
		}
	}
	// Browser exports may legitimately contain JSON quotes/backslashes. Validate
	// metadata with Go, but keep the browser value byte-for-byte for replay.
	metadata := cookieHTTP(c) // #nosec G124 -- Client-side replay validates source cookie metadata; it never sets a server response cookie.
	metadata.Value = ""
	if !validBrowserCookieValue(c.Value) {
		return apiError("INVALID_INPUT", "session contains unsafe cookie value bytes", 0, false)
	}
	if err := metadata.Valid(); err != nil {
		return apiError("INVALID_INPUT", "session contains an invalid cookie value", 0, false)
	}
	switch strings.ToLower(c.SameSite) {
	case "", "none", "lax", "strict":
	default:
		return apiError("INVALID_INPUT", "session contains an invalid cookie SameSite", 0, false)
	}
	return nil
}
func validatedPath(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" || u.Host != "www.traveloka.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" {
		return "", apiError("INVALID_INPUT", "request profiles must use the pinned Traveloka HTTPS origin", 0, false)
	}
	if _, ok := operations[u.Path]; !ok {
		return "", apiError("UNSUPPORTED_OPERATION", "request profile path is not an allowed read-only operation", 0, false)
	}
	return u.Path, nil
}
func readBoundedFile(path string) ([]byte, error) {
	f, err := os.Open(path) // #nosec G304 -- The operator explicitly selects this bounded local session/import file; no remote path selects it.
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxSessionBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxSessionBytes {
		return nil, fmt.Errorf("private session input exceeds size bound")
	}
	return b, nil
}

// ImportSession accepts only Traveloka-scoped cookies and eight exact read-only POST profiles.
func ImportSession(cookiesFile, requestsFile, outputFile string) (*SessionInfo, error) {
	cb, err := readBoundedFile(cookiesFile)
	if err != nil {
		return nil, apiError("INVALID_INPUT", "cannot read cookie input file", 0, false)
	}
	rb, err := readBoundedFile(requestsFile)
	if err != nil {
		return nil, apiError("INVALID_INPUT", "cannot read request input file", 0, false)
	}
	var cookies []scopedCookie
	var requests []requestCapture
	if err := decodeJSON(cb, &cookies); err != nil || cookies == nil {
		return nil, apiError("INVALID_INPUT", "cookies must be a JSON array", 0, false)
	}
	if err := decodeJSON(rb, &requests); err != nil || requests == nil {
		return nil, apiError("INVALID_INPUT", "requests must be a JSON array", 0, false)
	}
	if len(cookies) == 0 || len(requests) == 0 {
		return nil, apiError("INVALID_INPUT", "session requires scoped cookies and captured request profiles", 0, false)
	}
	for _, c := range cookies {
		if err := validateCookie(c); err != nil {
			return nil, err
		}
	}
	session := privateSession{Version: 1, CapturedAt: time.Now().UTC().Format(time.RFC3339Nano), Cookies: cookies, Profiles: map[string]requestProfile{}}
	for _, r := range requests {
		if r.Method != "POST" {
			return nil, apiError("UNSUPPORTED_OPERATION", "session profiles must be captured POST requests", 0, false)
		}
		path, err := validatedPath(r.URL)
		if err != nil {
			return nil, err
		}
		var body map[string]any
		if err := decodeJSON([]byte(r.Body), &body); err != nil || body == nil {
			return nil, apiError("INVALID_INPUT", "captured request body must be a JSON object", 0, false)
		}
		if _, ok := body["data"].(map[string]any); !ok {
			return nil, apiError("INVALID_INPUT", "captured request body must contain a data object", 0, false)
		}
		headers := map[string]string{}
		for k, v := range r.Headers {
			if strings.ContainsAny(k+v, "\r\n") {
				return nil, apiError("INVALID_INPUT", "invalid captured request headers", 0, false)
			}
			if importedHeaders[strings.ToLower(k)] {
				headers[http.CanonicalHeaderKey(k)] = v
			}
		}
		session.Profiles[path] = requestProfile{headers, body}
	}
	if err := writePrivateSession(outputFile, &session); err != nil {
		return nil, apiError("INVALID_INPUT", "cannot write private session file", 0, false)
	}
	return &SessionInfo{outputFile, session.CapturedAt, len(cookies), len(session.Profiles)}, nil
}
func writePrivateSession(path string, session *privateSession) error {
	if path == "" {
		return fmt.Errorf("private session path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return err
	}
	path = filepath.Join(parent, filepath.Base(path))
	return cliutil.WithFileLock(path, func() error { return writePrivateSessionUnlocked(path, session) })
}
func writePrivateSessionUnlocked(path string, session *privateSession) error {
	if path == "" {
		return fmt.Errorf("private session path is required")
	}
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("private session must be a regular file")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	b, err := json.Marshal(session)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".traveloka-session-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err != nil {
		_ = f.Close() // Preserve the earlier permission or write failure.
		return err
	}
	if _, err = f.Write(b); err != nil {
		_ = f.Close() // Preserve the earlier permission or write failure.
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
func cookieHTTP(c scopedCookie) *http.Cookie {
	out := &http.Cookie{Name: c.Name, Value: c.Value, Domain: c.Domain, Path: c.Path, HttpOnly: c.HTTPOnly, Secure: c.Secure} // #nosec G124 -- Preserve imported browser attributes for pinned HTTPS client replay; this is not a Set-Cookie response.
	if n, err := strconv.ParseFloat(string(c.Expires), 64); err == nil && n >= 0 {
		out.Expires = time.Unix(int64(n), 0)
	}
	switch strings.ToLower(c.SameSite) {
	case "strict":
		out.SameSite = http.SameSiteStrictMode
	case "lax":
		out.SameSite = http.SameSiteLaxMode
	case "none":
		out.SameSite = http.SameSiteNoneMode
	}
	return out
}

func validBrowserCookieValue(value string) bool {
	for i := 0; i < len(value); i++ {
		b := value[i]
		if b < 0x20 || b >= 0x7f || b == ';' {
			return false
		}
	}
	return true
}

// browserCookieHeader preserves imported browser values; net/http.Cookie.String
// would strip JSON quotes/backslashes or wrap commas, changing legitimate cookies.
func browserCookieHeader(cookies []*http.Cookie) (string, error) {
	parts := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		metadata := *cookie // #nosec G124 -- Validate metadata for a client request header, preserving the source browser attributes.
		metadata.Value = ""
		if err := metadata.Valid(); err != nil || !validBrowserCookieValue(cookie.Value) {
			return "", apiError("AUTH_REQUIRED", "private session contains unsafe cookie bytes; refresh with auth capture", 0, false)
		}
		parts = append(parts, cookie.Name+"="+cookie.Value)
	}
	return strings.Join(parts, "; "), nil
}

// parseBrowserSetCookies delegates attribute parsing to Go while retaining the
// source browser-compatible value. The fixed placeholder is never sent upstream.
func parseBrowserSetCookies(header http.Header) ([]*http.Cookie, error) {
	cookies := []*http.Cookie{}
	for _, line := range header.Values("Set-Cookie") {
		first, attributes, hasAttributes := strings.Cut(line, ";")
		name, value, ok := strings.Cut(strings.TrimSpace(first), "=")
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		if !ok || !validBrowserCookieValue(value) {
			return nil, apiError("MALFORMED_RESPONSE", "Traveloka returned unsafe cookie bytes", 0, false)
		}
		metadata := name + "=browser-cookie-placeholder"
		if hasAttributes {
			metadata += ";" + attributes
		}
		cookie, err := http.ParseSetCookie(metadata)
		if err != nil {
			return nil, apiError("MALFORMED_RESPONSE", "Traveloka returned malformed cookie metadata", 0, false)
		}
		cookie.Value = value
		cookie.Raw = ""
		cookie.Quoted = false
		cookies = append(cookies, cookie)
	}
	return cookies, nil
}
