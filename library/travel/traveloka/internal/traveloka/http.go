package traveloka

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/cliutil"
)

const maxResponseBytes = 24 << 20
const requestTimeout = 45 * time.Second

type Client struct {
	mu          sync.Mutex
	session     *privateSession
	sessionFile string
	diskHash    [32]byte
	http        *http.Client
	jar         http.CookieJar
	limiter     *cliutil.AdaptiveLimiter
	secrets     []string
}

func NewClient(sessionFile string) (*Client, error) {
	info, err := os.Lstat(sessionFile)
	if err != nil {
		return nil, apiError("AUTH_REQUIRED", "import or capture a Traveloka-only session with auth import-session or auth capture", 0, false)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return nil, apiError("AUTH_REQUIRED", "private session must be a regular file with mode 0600", 0, false)
	}
	// Resolve parent aliases so all clients lock the same stable sidecar.
	sessionFile, err = filepath.EvalSymlinks(sessionFile)
	if err != nil {
		return nil, apiError("AUTH_REQUIRED", "cannot resolve private Traveloka session", 0, false)
	}
	b, err := readBoundedFile(sessionFile)
	if err != nil {
		return nil, apiError("AUTH_REQUIRED", "cannot read private Traveloka session", 0, false)
	}
	var session privateSession
	if err := decodeJSON(b, &session); err != nil || session.Version != 1 || len(session.Cookies) == 0 || len(session.Profiles) == 0 {
		return nil, apiError("AUTH_REQUIRED", "invalid private session; refresh using auth capture", 0, false)
	}
	if _, err := time.Parse(time.RFC3339Nano, session.CapturedAt); err != nil {
		return nil, apiError("AUTH_REQUIRED", "invalid private session capture time", 0, false)
	}
	for _, c := range session.Cookies {
		if err := validateCookie(c); err != nil {
			return nil, apiError("AUTH_REQUIRED", "invalid Traveloka cookie scope; import a fresh scoped session", 0, false)
		}
	}
	for path, p := range session.Profiles {
		if _, ok := operations[path]; !ok {
			return nil, apiError("AUTH_REQUIRED", "private session contains an unsupported operation", 0, false)
		}
		if _, ok := p.Body["data"].(map[string]any); !ok {
			return nil, apiError("AUTH_REQUIRED", "private session operation has no data template", 0, false)
		}
		for k, v := range p.Headers {
			if !importedHeaders[strings.ToLower(k)] || strings.ContainsAny(k+v, "\r\n") {
				return nil, apiError("AUTH_REQUIRED", "invalid private session header profile", 0, false)
			}
		}
	}
	jar, _ := cookiejar.New(nil)
	u, _ := url.Parse(origin)
	for _, c := range session.Cookies {
		jar.SetCookies(u, []*http.Cookie{cookieHTTP(c)})
	}
	availableCookies := false
	for path := range session.Profiles {
		operationURL, _ := url.Parse(origin + path)
		if len(jar.Cookies(operationURL)) > 0 {
			availableCookies = true
			break
		}
	}
	if !availableCookies {
		return nil, apiError("AUTH_REQUIRED", "Traveloka cookies expired; refresh using auth capture", 0, false)
	}
	c := &Client{session: &session, sessionFile: sessionFile, diskHash: sha256.Sum256(b), limiter: cliutil.NewAdaptiveLimiter(2), jar: jar}
	// Keep scope/expiry selection in the jar. Serialize values explicitly because
	// net/http AddCookie would rewrite legitimate browser-exported JSON values.
	c.http = &http.Client{Timeout: requestTimeout, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return apiError("ACCESS_BLOCKED", "Traveloka redirect refused; refresh using auth capture", 0, false)
	}}
	c.collectSecrets()
	return c, nil
}
func (c *Client) collectSecrets() {
	for _, cookie := range c.session.Cookies {
		c.secrets = append(c.secrets, cookie.Value)
	}
	for _, p := range c.session.Profiles {
		for k, v := range p.Headers {
			if secretKey(k) || strings.EqualFold(k, "tv-mcc-id") {
				c.secrets = append(c.secrets, v)
			}
		}
		collectCredentialStrings(p.Body, &c.secrets)
	}
}
func (c *Client) TemplateData(path string) (map[string]any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := operations[path]; !ok {
		return nil, apiError("UNSUPPORTED_OPERATION", "operation is not in the Traveloka read-only allowlist", 0, false)
	}
	p, ok := c.session.Profiles[path]
	if !ok {
		return nil, apiError("AUTH_REQUIRED", "session lacks this operation; refresh using auth capture", 0, false)
	}
	return copyMap(p.Body["data"].(map[string]any))
}

// Post returns the complete sanitized response envelope, including data and meta.
func (c *Client) Post(ctx context.Context, path string, data map[string]any, shop Shopper) (map[string]any, error) {
	return c.PostWithEnvelope(ctx, path, data, shop, nil)
}

// PostWithEnvelope preserves the private captured envelope while accepting only
// the public projection and observed desktop-interface request fields.
// Hold the process-shared lock through loading the latest credentials, sending
// the request and persisting refreshes. Atomic rename alone cannot protect a
// read/refresh/write transaction from another CLI or MCP process.
func (c *Client) PostWithEnvelope(ctx context.Context, path string, data map[string]any, shop Shopper, overrides map[string]any) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	for !c.mu.TryLock() {
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, sourceHTTPError(ctx, ctx.Err(), 0, "Traveloka session lock wait canceled", nil)
		case <-timer.C:
		}
	}
	defer c.mu.Unlock()
	var result map[string]any
	err := cliutil.WithFileLockContext(ctx, c.sessionFile, func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		b, err := readBoundedFile(c.sessionFile)
		if err != nil {
			return apiError("AUTH_REQUIRED", "cannot read private Traveloka session", 0, false)
		}
		if sha256.Sum256(b) != c.diskHash {
			latest, err := NewClient(c.sessionFile)
			if err != nil {
				return err
			}
			c.session, c.jar, c.diskHash = latest.session, latest.jar, latest.diskHash
			c.secrets = append(c.secrets, latest.secrets...)
		}
		result, err = c.postWithEnvelopeLocked(ctx, path, data, shop, overrides)
		return err
	})
	if ctx.Err() != nil {
		return nil, sourceHTTPError(ctx, ctx.Err(), 0, "Traveloka session request canceled", c.secrets)
	}
	return result, err
}

func (c *Client) persistSessionLocked() error {
	if err := writePrivateSessionUnlocked(c.sessionFile, c.session); err != nil {
		return err
	}
	b, err := json.Marshal(c.session)
	if err != nil {
		return err
	}
	c.diskHash = sha256.Sum256(b)
	return nil
}

func (c *Client) postWithEnvelopeLocked(ctx context.Context, path string, data map[string]any, shop Shopper, overrides map[string]any) (map[string]any, error) {
	if _, ok := operations[path]; !ok {
		return nil, apiError("UNSUPPORTED_OPERATION", "operation is not in the Traveloka read-only allowlist", 0, false)
	}
	if err := ValidateShopper(shop); err != nil {
		return nil, err
	}
	if data == nil {
		return nil, apiError("INVALID_INPUT", "request data must be an object", 0, false)
	}
	if path == "/api/v2/flight/search/redirection" {
		prefetch, ok := data["isPrefetch"].(bool)
		if !ok || !prefetch {
			return nil, apiError("UNSUPPORTED_OPERATION", "flight redirection requires explicit isPrefetch=true for read-only quote preparation", 0, false)
		}
	}
	public, err := validateEnvelopeOverrides(overrides)
	if err != nil {
		return nil, err
	}
	profile, ok := c.session.Profiles[path]
	if !ok {
		return nil, apiError("AUTH_REQUIRED", "session lacks this operation; refresh using auth capture", 0, false)
	}
	payload, err := copyMap(profile.Body)
	if err != nil {
		return nil, apiError("INVALID_INPUT", "invalid captured request template", 0, false)
	}
	d, err := copyMap(data)
	if err != nil {
		return nil, apiError("INVALID_INPUT", "request data cannot be encoded", 0, false)
	}
	if _, ok := d["currency"]; ok {
		d["currency"] = shop.Currency
	}
	if _, ok := d["locale"]; ok {
		d["locale"] = strings.ReplaceAll(shop.Locale, "-", "_")
	}
	payload["data"] = d
	for key, value := range public {
		payload[key] = value
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, apiError("INVALID_INPUT", "request cannot be encoded", 0, false)
	}
	bounded, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	if err := c.limiter.Wait(bounded); err != nil {
		return nil, sourceHTTPError(bounded, err, 0, "Traveloka request pacing failed", c.secrets)
	}
	req, err := http.NewRequestWithContext(bounded, http.MethodPost, origin+path, bytes.NewReader(body))
	if err != nil {
		return nil, apiError("INVALID_INPUT", "cannot construct request", 0, false)
	}
	for k, v := range profile.Headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Origin", origin)
	req.Header.Set("Referer", origin+"/")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Domain", operations[path])
	req.Header.Set("Tv-Country", shop.Market)
	req.Header.Set("Tv-Language", strings.ReplaceAll(shop.Locale, "-", "_"))
	req.Header.Set("Tv-Currency", shop.Currency)
	req.Header.Set("X-Route-Prefix", strings.ToLower(shop.Locale))
	if id, ok := d["searchId"].(string); ok && id != "" {
		req.Header.Set("Fpr-Search-Id", id)
	}
	cookieHeader, err := browserCookieHeader(c.jar.Cookies(req.URL))
	if err != nil {
		return nil, err
	}
	if cookieHeader != "" {
		req.Header.Set("Cookie", cookieHeader)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, sourceHTTPError(bounded, err, 0, "Traveloka HTTP request failed: "+err.Error(), c.secrets)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, sourceHTTPError(bounded, err, resp.StatusCode, "cannot read Traveloka response", c.secrets)
	}
	if len(raw) > maxResponseBytes {
		return nil, apiError("MALFORMED_RESPONSE", "Traveloka response exceeds size bound", resp.StatusCode, false)
	}
	responseCookies, err := parseBrowserSetCookies(resp.Header)
	if err != nil {
		return nil, err
	}
	if err := c.collectResponseCookies(responseCookies, req.URL); err != nil {
		return nil, apiError("UPSTREAM_ERROR", "cannot persist refreshed private cookies", resp.StatusCode, false)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		c.limiter.OnRateLimit()
		return nil, &cliutil.RateLimitError{URL: origin + path, RetryAfter: retryAfter(resp.Header.Get("Retry-After")), Body: "Traveloka throttled this read-only request", Cause: apiError("RATE_LIMITED", "Traveloka returned HTTP 429", 429, true)}
	}
	if resp.StatusCode == http.StatusAccepted {
		return nil, apiError("ACCESS_BLOCKED", "Traveloka protection response; refresh the scoped session using auth capture", 202, false)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, apiError("AUTH_REQUIRED", "Traveloka session expired; refresh using auth capture", 401, false)
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, apiError("ACCESS_BLOCKED", "Traveloka access blocked; refresh the scoped session using auth capture", 403, false)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, apiError("UPSTREAM_ERROR", "Traveloka returned HTTP "+strconv.Itoa(resp.StatusCode), resp.StatusCode, resp.StatusCode >= 500)
	}
	var result map[string]any
	if err := decodeJSON(raw, &result); err != nil || result == nil {
		return nil, apiError("MALFORMED_RESPONSE", "Traveloka returned malformed or empty JSON", resp.StatusCode, false)
	}
	collectCredentialStrings(result, &c.secrets)
	// Response sentinel objects can be partial (observed token-only responses).
	// Preserve the captured request schema, including signals, instead of
	// transplanting the response object as the next request sentinel.
	if update, ok := result["sentinel"]; ok {
		merged, changed, err := mergeCapturedSentinel(profile.Body["sentinel"], update)
		if err != nil {
			return nil, apiError("MALFORMED_RESPONSE", "cannot preserve private request sentinel shape", resp.StatusCode, false)
		}
		if changed {
			profile.Body["sentinel"] = merged
			c.session.Profiles[path] = profile
			if err := c.persistSessionLocked(); err != nil {
				return nil, apiError("UPSTREAM_ERROR", "cannot persist refreshed private session", resp.StatusCode, false)
			}
		}
	}

	c.limiter.OnSuccess()
	return sanitizeValue(result, c.secrets).(map[string]any), nil
}
func (c *Client) collectResponseCookies(cookies []*http.Cookie, requestURL *url.URL) error {
	if len(cookies) == 0 {
		return nil
	}
	changed := false
	for _, cookie := range cookies {
		domain := cookie.Domain
		if domain == "" {
			domain = "www.traveloka.com"
		}
		if !validDomain(domain) {
			continue
		}
		path := cookie.Path
		if path == "" {
			path = "/"
			if last := strings.LastIndex(requestURL.Path, "/"); last > 0 {
				path = requestURL.Path[:last]
			}
		}
		expiry := json.Number("-1")
		if !cookie.Expires.IsZero() {
			expiry = json.Number(strconv.FormatInt(cookie.Expires.Unix(), 10))
		}
		if cookie.MaxAge > 0 {
			expiry = json.Number(strconv.FormatInt(time.Now().Add(time.Duration(cookie.MaxAge)*time.Second).Unix(), 10))
		}
		if cookie.MaxAge < 0 {
			expiry = "1"
		}
		sameSite := ""
		switch cookie.SameSite {
		case http.SameSiteLaxMode:
			sameSite = "Lax"
		case http.SameSiteStrictMode:
			sameSite = "Strict"
		case http.SameSiteNoneMode:
			sameSite = "None"
		}
		scoped := scopedCookie{Name: cookie.Name, Value: cookie.Value, Domain: domain, Path: path, Expires: expiry, HTTPOnly: cookie.HttpOnly, Secure: cookie.Secure, SameSite: sameSite}
		if err := validateCookie(scoped); err != nil {
			return err
		}
		cookie.Path = path
		c.jar.SetCookies(requestURL, []*http.Cookie{cookie})
		c.secrets = append(c.secrets, cookie.Value)
		replaced := false
		for i, old := range c.session.Cookies {
			if old.Name == scoped.Name && old.Domain == scoped.Domain && old.Path == scoped.Path {
				c.session.Cookies[i] = scoped
				replaced = true
				break
			}
		}
		if !replaced {
			c.session.Cookies = append(c.session.Cookies, scoped)
		}
		changed = true
	}
	if changed {
		return c.persistSessionLocked()
	}
	return nil
}
func retryAfter(raw string) time.Duration {
	if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 86400 {
		return time.Duration(n) * time.Second
	}
	if t, err := http.ParseTime(raw); err == nil {
		d := time.Until(t)
		if d > 0 && d <= 24*time.Hour {
			return d
		}
	}
	return 0
}

// SetHTTPTransport supplies the generated Chrome-compatible transport without changing the pinned origin or jar.
func (c *Client) SetHTTPTransport(transport http.RoundTripper) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if transport == nil {
		transport = http.DefaultTransport
	}
	c.http.Transport = transport
}

// mergeCapturedSentinel accepts partial object refreshes only when the operation
// has a captured object sentinel. It never invents a request shape from a response.
func mergeCapturedSentinel(captured, update any) (map[string]any, bool, error) {
	base, baseOK := captured.(map[string]any)
	patch, patchOK := update.(map[string]any)
	if !baseOK || !patchOK {
		return nil, false, nil
	}
	merged, err := copyMap(base)
	if err != nil {
		return nil, false, err
	}
	copiedPatch, err := copyMap(patch)
	if err != nil {
		return nil, false, err
	}
	mergeSentinelFields(merged, copiedPatch)
	return merged, true, nil
}
func mergeSentinelFields(base, patch map[string]any) {
	for key, value := range patch {
		existing, baseOK := base[key].(map[string]any)
		incoming, patchOK := value.(map[string]any)
		if baseOK && patchOK {
			mergeSentinelFields(existing, incoming)
		} else {
			base[key] = value
		}
	}
}

// Validate only caller-owned public fields, never captured credential material.
func validateEnvelopeOverrides(overrides map[string]any) (map[string]any, error) {
	public := make(map[string]any, len(overrides))
	for key, value := range overrides {
		switch key {
		case "clientInterface":
			iface, ok := value.(string)
			if !ok {
				return nil, apiError("INVALID_INPUT", "clientInterface must be a string", 0, false)
			}
			if iface != "desktop" {
				return nil, apiError("UNSUPPORTED_OPERATION", "only the observed desktop clientInterface is supported", 0, false)
			}
			public[key] = iface
		case "fields":
			fields := []string{}
			switch values := value.(type) {
			case []string:
				fields = append(fields, values...)
			case []any:
				for _, raw := range values {
					field, ok := raw.(string)
					if !ok {
						return nil, apiError("INVALID_INPUT", "fields must be an array of projection-name strings", 0, false)
					}
					fields = append(fields, field)
				}
			default:
				return nil, apiError("INVALID_INPUT", "fields must be an array of projection-name strings", 0, false)
			}
			if len(fields) > 100 {
				return nil, apiError("INVALID_INPUT", "fields accepts at most 100 projection names", 0, false)
			}
			for _, field := range fields {
				if strings.TrimSpace(field) == "" || len(field) > 256 || strings.ContainsAny(field, "\r\n") {
					return nil, apiError("INVALID_INPUT", "projection names must be nonempty and at most 256 bytes", 0, false)
				}
			}
			public[key] = fields
		default:
			return nil, apiError("INVALID_INPUT", "only fields and clientInterface may override the captured request envelope", 0, false)
		}
	}
	return public, nil
}

// Preserve only safe context causes while presenting sanitized source failures.
func sourceHTTPError(ctx context.Context, err error, status int, message string, secrets []string) error {
	cause := err
	if ctx.Err() != nil {
		cause = ctx.Err()
	}
	if errors.Is(cause, context.DeadlineExceeded) {
		return &APIError{Code: "TIMEOUT", Message: "Traveloka request deadline exceeded", Status: status, Retryable: true, Cause: context.DeadlineExceeded}
	}
	if errors.Is(cause, context.Canceled) {
		return &APIError{Code: "CANCELED", Message: "Traveloka request canceled", Status: status, Cause: context.Canceled}
	}
	var rate *cliutil.RateLimitError
	if errors.As(err, &rate) {
		return rate
	}
	var source *APIError
	if errors.As(err, &source) {
		return source
	}
	return apiError("UPSTREAM_ERROR", redactText(message, secrets), status, true)
}
