// Package omnilogic implements the Hayward OmniLogic cloud partner API
// client: two-stage auth (REST JSON login -> XML-envelope ops), XML envelope
// build/parse, token caching, and operation-specific request helpers.
//
// All operations POST to the same URL with a different <Name> in the body and
// the cached token in a Token: header. The Python reference is djtimca/omnilogic-api.
package omnilogic

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/hayward-omnilogic/internal/cliutil"
)

var _ = time.Now // ensure time import stays used

// Client is the OmniLogic client. Construct with New. Token caching is
// transparent: every operation calls ensureToken() first.
type Client struct {
	email         string
	password      string
	http          *http.Client
	authCachePath string
	limiter       *cliutil.AdaptiveLimiter

	mu    sync.Mutex
	state *AuthState
}

// New builds a Client from email + password (typically HAYWARD_USER /
// HAYWARD_PW env vars). It pre-loads the token cache so the first operation
// doesn't pay the login round-trip.
func New(email, password string, timeout time.Duration) *Client {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	c := &Client{
		email:         email,
		password:      password,
		http:          &http.Client{Timeout: timeout},
		authCachePath: defaultAuthCachePath(),
		// Hayward doesn't publish rate limits; community guidance is 30-60s
		// per call. 2 req/s gives us headroom for compound calls (status,
		// sweep) while staying well within polite usage. AdaptiveLimiter
		// will back off automatically on 429s if Hayward ever starts
		// throttling.
		limiter: cliutil.NewAdaptiveLimiter(2.0),
	}
	if s, err := loadAuthState(c.authCachePath); err == nil {
		// Only reuse if the cached email matches the current credentials —
		// otherwise we'd happily replay a previous user's token.
		if s.Email == email {
			c.state = s
		}
	}
	return c
}

// SetAuthCachePath overrides where the token cache is read from / written to.
// Used by tests and `auth login --cache-path`.
func (c *Client) SetAuthCachePath(p string) { c.authCachePath = p }

// AuthCachePath returns the resolved token cache path so doctor / status can
// report it.
func (c *Client) AuthCachePath() string { return c.authCachePath }

// Email returns the email the client was constructed with so doctor can
// surface "logged in as X" without leaking the password.
func (c *Client) Email() string { return c.email }

// HasCredentials returns true when both email and password are set on the
// Client; false signals that HAYWARD_USER / HAYWARD_PW were unset at startup
// and the caller should surface the env-var-missing error rather than
// attempting an obviously-doomed login round-trip.
func (c *Client) HasCredentials() bool { return c.email != "" && c.password != "" }

// AuthState returns a copy of the current cached auth state, or nil if there
// isn't one. Useful for `auth status`.
func (c *Client) AuthState() *AuthState {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == nil {
		return nil
	}
	cp := *c.state
	return &cp
}

// EnsureToken makes sure the client has a valid token. It tries the cached
// token, then refresh, then a fresh login. Persists the resulting state to
// the auth cache. Returns ErrMissingCredentials if email/password are unset.
func (c *Client) EnsureToken() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state != nil && c.state.Valid() {
		return nil
	}
	if c.email == "" || c.password == "" {
		return ErrMissingCredentials
	}
	var (
		ns  *AuthState
		err error
	)
	if c.state != nil && c.state.RefreshToken != "" {
		ns, err = c.refresh(c.state)
	} else {
		ns, err = c.login()
	}
	if err != nil {
		return err
	}
	c.state = ns
	_ = saveAuthState(c.authCachePath, c.state) // best-effort persistence
	return nil
}

// Logout clears the cached token from disk and memory.
func (c *Client) Logout() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state = nil
	if c.authCachePath == "" {
		return nil
	}
	if err := removeFile(c.authCachePath); err != nil && !errors.Is(err, errFileNotExist) {
		return err
	}
	return nil
}

var ErrMissingCredentials = errors.New("HAYWARD_USER and HAYWARD_PW must be set")

// callOp executes one OmniLogic operation: builds the XML envelope, attaches
// the token header (and SiteID when MspSystemID is present in params), POSTs
// to the .ashx URL, and returns the raw response body. It re-authenticates
// once on a 401-shaped response.
func (c *Client) callOp(opName string, params map[string]any) (string, error) {
	if err := c.EnsureToken(); err != nil {
		return "", err
	}
	// First attempt
	body, status, err := c.doOp(opName, params)
	if err != nil {
		return "", err
	}
	// Some failures present as "There is no information" in the response
	// body rather than as an HTTP error — but those are login-shaped misses.
	// For operations, the canonical authenticated-failure signal is HTTP 401
	// or a token-expired Status code in the XML.
	if status == 401 {
		// Force token refresh and retry once.
		c.mu.Lock()
		c.state = nil
		c.mu.Unlock()
		if err := c.EnsureToken(); err != nil {
			return "", err
		}
		body, status, err = c.doOp(opName, params)
		if err != nil {
			return "", err
		}
	}
	if status >= 400 {
		return "", &APIError{Op: opName, StatusCode: status, Body: body}
	}
	return body, nil
}

// callOpOrdered is the ordered-param variant of callOp. Set* operations that
// hit the .ashx endpoint with deterministic parameter ordering (Hayward's
// .NET handler is order-sensitive on SetUIEquipmentCmd and SetUI*Cmd
// variants) should use this path.
func (c *Client) callOpOrdered(opName string, ordered []orderedParam, mspSystemID int) (string, error) {
	if err := c.EnsureToken(); err != nil {
		return "", err
	}
	body, status, err := c.doOpOrdered(opName, ordered, mspSystemID)
	if err != nil {
		return "", err
	}
	if status == 401 {
		c.mu.Lock()
		c.state = nil
		c.mu.Unlock()
		if err := c.EnsureToken(); err != nil {
			return "", err
		}
		body, status, err = c.doOpOrdered(opName, ordered, mspSystemID)
		if err != nil {
			return "", err
		}
	}
	if status >= 400 {
		return "", &APIError{Op: opName, StatusCode: status, Body: body}
	}
	return body, nil
}

func (c *Client) doOpOrdered(opName string, ordered []orderedParam, mspSystemID int) (string, int, error) {
	payload, err := buildOrderedRequest(opName, ordered)
	if err != nil {
		return "", 0, err
	}
	return c.sendOpRequest(opName, payload, mspSystemID)
}

func (c *Client) doOp(opName string, params map[string]any) (string, int, error) {
	var payload string
	if opName == "SetCHLORParams" {
		payload = buildChlorRequest(params)
	} else {
		p, err := buildRequest(opName, params)
		if err != nil {
			return "", 0, err
		}
		payload = p
	}
	msp := 0
	if v, ok := params["MspSystemID"]; ok {
		msp = asInt(v)
	}
	return c.sendOpRequest(opName, payload, msp)
}

// sendOpRequest is the shared HTTP-and-response leg used by both the
// map-based doOp and the ordered-slice doOpOrdered. Sets Content-Type,
// Token header (from cached auth state), SiteID header (when msp > 0),
// limiter integration, and 429 mapping to cliutil.RateLimitError.
func (c *Client) sendOpRequest(opName, payload string, mspSystemID int) (string, int, error) {
	if os.Getenv("HAYWARD_DEBUG") != "" {
		// Opt-in payload dump for protocol debugging against Hayward's
		// .NET-shaped error responses. Set HAYWARD_DEBUG=1 to enable.
		fmt.Fprintf(os.Stderr, "[debug] %s payload:\n%s\n", opName, payload)
	}
	req, err := http.NewRequest("POST", OpsURL, bytes.NewReader([]byte(payload)))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "text/xml")
	req.Header.Set("Cache-Control", "no-cache")
	c.mu.Lock()
	if c.state != nil && c.state.Token != "" {
		req.Header.Set("Token", c.state.Token)
	}
	c.mu.Unlock()
	if mspSystemID > 0 {
		req.Header.Set("SiteID", strconv.Itoa(mspSystemID))
	}
	if c.limiter != nil {
		c.limiter.Wait()
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("HTTP %s: %w", opName, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 429 {
		if c.limiter != nil {
			c.limiter.OnRateLimit()
		}
		retryAfter := cliutil.RetryAfter(resp)
		return string(body), resp.StatusCode, &cliutil.RateLimitError{
			URL:        OpsURL,
			RetryAfter: retryAfter,
			Body:       string(body),
		}
	}
	if c.limiter != nil && resp.StatusCode < 400 {
		c.limiter.OnSuccess()
	}
	return string(body), resp.StatusCode, nil
}

func asInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int32:
		return int(x)
	case int64:
		return int(x)
	case string:
		n, _ := strconv.Atoi(x)
		return n
	}
	return 0
}

func asString(v any) string {
	switch x := v.(type) {
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case string:
		return x
	default:
		return fmt.Sprintf("%v", v)
	}
}

// APIError carries a non-2xx response from the .ashx endpoint.
type APIError struct {
	Op         string
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("hayward %s failed (HTTP %d): %s", e.Op, e.StatusCode, truncate(e.Body, 200))
}

func truncate(s string, n int) string {
	return Truncate(s, n)
}

// Truncate is the exported wrapper for the package's truncation helper so
// CLI-level error classifiers can summarize OmniLogic error bodies without
// pulling in their own string helpers.
func Truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
