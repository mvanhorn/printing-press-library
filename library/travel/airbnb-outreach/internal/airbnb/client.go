// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.

package airbnb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/airbnb-outreach/internal/cliutil"
)

// UserAgent mimics a current desktop Chrome so requests match the fingerprint
// Airbnb's frontend uses. The API surface is not TLS-fingerprint gated (plain
// Go HTTP reaches it), but a browser-shaped UA avoids trivial UA filtering.
const UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36"

// Client talks to Airbnb's internal API. Reads use GET persisted queries;
// writes use POST mutations. All requests carry the public API key and the
// x-airbnb-* platform headers; authenticated requests add the session cookies.
type Client struct {
	http     *http.Client
	limiter  *cliutil.AdaptiveLimiter
	baseURL  string
	locale   string
	currency string
	registry *Registry
	session  *Session
	DryRun   bool
}

// maxRateLimitRetries bounds how many times a request is retried on HTTP 429
// before a *cliutil.RateLimitError is returned.
const maxRateLimitRetries = 3

// Options configure a Client.
type Options struct {
	BaseURL  string
	Locale   string
	Currency string
	Timeout  time.Duration
	Session  *Session
	Registry *Registry
}

// NewClient builds a Client, applying sensible defaults.
func NewClient(o Options) *Client {
	if o.BaseURL == "" {
		o.BaseURL = "https://www.airbnb.com"
	}
	o.BaseURL = strings.TrimRight(o.BaseURL, "/")
	if o.Locale == "" {
		o.Locale = "en"
	}
	if o.Currency == "" {
		o.Currency = "USD"
	}
	if o.Timeout == 0 {
		o.Timeout = 30 * time.Second
	}
	if o.Session == nil {
		o.Session = LoadSession()
	}
	if o.Registry == nil {
		o.Registry = LoadRegistry()
	}
	return &Client{
		http:     &http.Client{Timeout: o.Timeout},
		limiter:  cliutil.NewAdaptiveLimiter(3),
		baseURL:  o.BaseURL,
		locale:   o.Locale,
		currency: o.Currency,
		registry: o.Registry,
		session:  o.Session,
	}
}

// do sends a request through the adaptive rate limiter, retrying on HTTP 429
// with the server's Retry-After (or exponential backoff) up to
// maxRateLimitRetries. When retries are exhausted it returns a
// *cliutil.RateLimitError so throttling is never silently swallowed as empty
// data. The response body is fully read and returned.
func (c *Client) do(req *http.Request) (int, []byte, error) {
	for attempt := 0; ; attempt++ {
		c.limiter.Wait()
		resp, err := c.http.Do(req)
		if err != nil {
			return 0, nil, err
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
		resp.Body.Close()
		if readErr != nil {
			return resp.StatusCode, nil, readErr
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			c.limiter.OnRateLimit()
			if attempt >= maxRateLimitRetries {
				return resp.StatusCode, body, &cliutil.RateLimitError{
					URL:        req.URL.String(),
					RetryAfter: cliutil.RetryAfter(resp),
					Body:       truncate(string(body), 200),
				}
			}
			wait := cliutil.RetryAfter(resp)
			if wait <= 0 {
				wait = cliutil.Backoff(attempt)
			}
			time.Sleep(wait)
			if req.GetBody != nil {
				if nb, e := req.GetBody(); e == nil {
					req.Body = nb
				}
			}
			continue
		}
		c.limiter.OnSuccess()
		return resp.StatusCode, body, nil
	}
}

// Authenticated reports whether the client has a login session.
func (c *Client) Authenticated() bool { return c.session.Authenticated() }

// Registry exposes the operation registry for `ops` commands.
func (c *Client) Registry() *Registry { return c.registry }

// HTTPClient exposes the underlying client for the hash harvester.
func (c *Client) HTTPClient() *http.Client { return c.http }

// BaseURL returns the configured base URL.
func (c *Client) BaseURL() string { return c.baseURL }

// APIError carries a structured Airbnb API failure.
type APIError struct {
	Op         string
	StatusCode int
	Airlock    bool
	Message    string
	Body       string
}

func (e *APIError) Error() string {
	if e.Airlock {
		return fmt.Sprintf("airbnb %s: blocked by Airlock bot-check (HTTP %d) — your session may need re-verification in a browser", e.Op, e.StatusCode)
	}
	if e.Message != "" {
		return fmt.Sprintf("airbnb %s: %s (HTTP %d)", e.Op, e.Message, e.StatusCode)
	}
	return fmt.Sprintf("airbnb %s: HTTP %d", e.Op, e.StatusCode)
}

// ErrUnknownOperation is returned when an operation hash is not in the registry.
type ErrUnknownOperation struct{ Op string }

func (e *ErrUnknownOperation) Error() string {
	return fmt.Sprintf("unknown operation %q: run `airbnb-outreach-pp-cli ops refresh` to harvest current operation hashes", e.Op)
}

// DryRunRequest describes an outbound request when DryRun is set.
type DryRunRequest struct {
	Method    string          `json:"method"`
	URL       string          `json:"url"`
	Operation string          `json:"operation"`
	Variables json.RawMessage `json:"variables,omitempty"`
	Note      string          `json:"note,omitempty"`
}

func (c *Client) commonHeaders(req *http.Request) {
	req.Header.Set("x-airbnb-api-key", PublicAPIKey)
	req.Header.Set("x-airbnb-graphql-platform", "web")
	req.Header.Set("x-airbnb-graphql-platform-client", "minimalist-niobe")
	req.Header.Set("x-airbnb-supports-airlock-v2", "true")
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "application/json")
	if ch := c.session.CookieHeader(); ch != "" {
		req.Header.Set("Cookie", ch)
	}
}

// Query executes a persisted GraphQL query (GET) and returns its `data` object.
func (c *Client) Query(op string, variables any) (json.RawMessage, error) {
	hash := c.registry.Hash(op)
	if hash == "" {
		return nil, &ErrUnknownOperation{Op: op}
	}
	varsJSON, err := json.Marshal(variables)
	if err != nil {
		return nil, err
	}
	ext, _ := json.Marshal(map[string]any{
		"persistedQuery": map[string]any{"version": 1, "sha256Hash": hash},
	})
	q := url.Values{}
	q.Set("operationName", op)
	q.Set("locale", c.locale)
	q.Set("currency", c.currency)
	q.Set("variables", string(varsJSON))
	q.Set("extensions", string(ext))
	fullURL := fmt.Sprintf("%s/api/v3/%s/%s?%s", c.baseURL, op, hash, q.Encode())

	if c.DryRun {
		return c.dryRun(http.MethodGet, fullURL, op, varsJSON)
	}

	req, err := http.NewRequest(http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}
	c.commonHeaders(req)
	return c.doGraphQL(op, req)
}

// Mutation executes a persisted GraphQL mutation (POST) and returns its `data`.
func (c *Client) Mutation(op string, variables any) (json.RawMessage, error) {
	return c.postPersisted(op, variables)
}

// QueryPost executes a persisted GraphQL *query* over POST. Airbnb serves some
// large-variable queries (e.g. StaysPdpSections, CheckoutFlowQuery) only over
// POST; GET returns a generic error for them.
func (c *Client) QueryPost(op string, variables any) (json.RawMessage, error) {
	return c.postPersisted(op, variables)
}

// postPersisted is the shared POST transport for mutations and POST-only queries.
func (c *Client) postPersisted(op string, variables any) (json.RawMessage, error) {
	hash := c.registry.Hash(op)
	if hash == "" {
		return nil, &ErrUnknownOperation{Op: op}
	}
	body := map[string]any{
		"operationName": op,
		"variables":     variables,
		"extensions": map[string]any{
			"persistedQuery": map[string]any{"version": 1, "sha256Hash": hash},
		},
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	fullURL := fmt.Sprintf("%s/api/v3/%s/%s", c.baseURL, op, hash)

	if c.DryRun {
		vj, _ := json.Marshal(variables)
		return c.dryRun(http.MethodPost, fullURL, op, vj)
	}

	req, err := http.NewRequest(http.MethodPost, fullURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, err
	}
	c.commonHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	// Airbnb's web client sends this when it has no CSRF token cookie; without
	// it, POST requests are rejected by the Airlock layer.
	req.Header.Set("x-csrf-without-token", "1")
	return c.doGraphQL(op, req)
}

// GetV2 calls a legacy REST /api/v2 endpoint (GET) and returns the raw body.
func (c *Client) GetV2(path string, params map[string]string) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("locale", c.locale)
	q.Set("currency", c.currency)
	q.Set("key", PublicAPIKey)
	for k, v := range params {
		q.Set(k, v)
	}
	fullURL := fmt.Sprintf("%s%s?%s", c.baseURL, path, q.Encode())
	if c.DryRun {
		return c.dryRun(http.MethodGet, fullURL, path, nil)
	}
	req, err := http.NewRequest(http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}
	c.commonHeaders(req)
	status, data, err := c.do(req)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, &APIError{Op: path, StatusCode: status, Body: truncate(string(data), 400)}
	}
	return json.RawMessage(data), nil
}

// GetHTML fetches a same-origin HTML page (with the session cookies) and
// returns the body. Used for pages whose data is server-rendered rather than
// exposed through a client GraphQL query (e.g. the trips page).
func (c *Client) GetHTML(path string) (string, error) {
	fullURL := c.baseURL + path
	req, err := http.NewRequest(http.MethodGet, fullURL, nil)
	if err != nil {
		return "", err
	}
	c.commonHeaders(req)
	req.Header.Set("Accept", "text/html")
	status, body, err := c.do(req)
	if err != nil {
		return "", err
	}
	if status >= 400 {
		return "", &APIError{Op: path, StatusCode: status}
	}
	return string(body), nil
}

// putBytes uploads raw bytes with a PUT through the rate limiter (used for the
// signed-URL image upload step). Returns the HTTP status.
func (c *Client) putBytes(rawURL, contentType string, data []byte) (int, error) {
	req, err := http.NewRequest(http.MethodPut, rawURL, bytes.NewReader(data))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", contentType)
	status, _, err := c.do(req)
	return status, err
}

// fetchText fetches a URL (e.g. a JS bundle on the CDN) through the rate
// limiter and returns the body. Used by the operation-hash harvester.
func (c *Client) fetchText(rawURL string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "text/html,application/javascript,*/*")
	status, body, err := c.do(req)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("GET %s: HTTP %d", rawURL, status)
	}
	return string(body), nil
}

func (c *Client) doGraphQL(op string, req *http.Request) (json.RawMessage, error) {
	status, data, err := c.do(req)
	if err != nil {
		return nil, err
	}

	// Airlock challenge shape: {"success":false,"redirect":...} usually with 401.
	if looksLikeAirlock(data) {
		return nil, &APIError{Op: op, StatusCode: status, Airlock: true, Body: truncate(string(data), 400)}
	}
	if status >= 400 {
		msg := extractGraphQLError(data)
		return nil, &APIError{Op: op, StatusCode: status, Message: msg, Body: truncate(string(data), 400)}
	}

	var env struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, &APIError{Op: op, StatusCode: status, Message: "unparseable response", Body: truncate(string(data), 400)}
	}
	if len(env.Errors) > 0 {
		return env.Data, &APIError{Op: op, StatusCode: status, Message: env.Errors[0].Message, Body: truncate(string(data), 400)}
	}
	return env.Data, nil
}

func (c *Client) dryRun(method, fullURL, op string, vars json.RawMessage) (json.RawMessage, error) {
	dr := DryRunRequest{Method: method, URL: fullURL, Operation: op, Variables: vars, Note: "dry-run: request not sent"}
	return json.Marshal(dr)
}

func looksLikeAirlock(data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return false
	}
	var probe struct {
		Success  *bool           `json:"success"`
		Redirect json.RawMessage `json:"redirect"`
	}
	if err := json.Unmarshal(trimmed, &probe); err != nil {
		return false
	}
	return probe.Success != nil && !*probe.Success && probe.Redirect != nil
}

func extractGraphQLError(data []byte) string {
	var env struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
		ErrorMessage string `json:"error_message"`
	}
	if json.Unmarshal(data, &env) == nil {
		if len(env.Errors) > 0 && env.Errors[0].Message != "" {
			return env.Errors[0].Message
		}
		if env.ErrorMessage != "" {
			return env.ErrorMessage
		}
	}
	return ""
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
