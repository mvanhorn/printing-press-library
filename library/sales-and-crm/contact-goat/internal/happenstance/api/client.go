// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Client is a bearer-auth Go client for the Happenstance public REST API.
// It owns no persistent state besides the http.Client and the bearer key.
// All polling state for asynchronous endpoints is kept per-call.
//
// Client is safe for concurrent use; the underlying http.Client handles
// connection pooling.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	dryRun     bool
	userAgent  string
}

// Option configures a Client. The functional-option pattern matches the
// rest of the contact-goat codebase (see internal/client/cookie_auth.go for
// the canonical example).
type Option func(*Client)

// WithBaseURL overrides the default API root. Tests pass an httptest server
// URL; production code that talks to a private Happenstance instance can
// override here as well.
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = strings.TrimRight(url, "/")
	}
}

// WithHTTPClient lets callers inject a fully-configured http.Client (e.g.
// to add a custom transport for tracing or to share a connection pool).
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		if hc != nil {
			c.httpClient = hc
		}
	}
}

// WithDryRun makes the client print the request line + redacted Authorization
// header to stdout and return a synthetic success ({"dry_run": true}) without
// sending. The bearer key is replaced with the literal string defined by
// RedactedBearerLine; the real value never reaches stdout.
func WithDryRun(dry bool) Option {
	return func(c *Client) {
		c.dryRun = dry
	}
}

// WithUserAgent overrides the User-Agent header sent with every request.
// Defaults to "contact-goat-pp-cli/happenstance-api".
func WithUserAgent(ua string) Option {
	return func(c *Client) {
		if ua != "" {
			c.userAgent = ua
		}
	}
}

// NewClient constructs a Client. apiKey may be empty; the call will fail at
// request time with a friendly 401-ish error rather than at construction.
// This matches the pattern in internal/deepline/client.go where the missing-
// key surface is normalized into the same error path as a rejected key.
func NewClient(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:     apiKey,
		baseURL:    DefaultBaseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		userAgent:  "contact-goat-pp-cli/happenstance-api",
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// BaseURL returns the configured API root for this client. Useful for tests
// and for surface-selection diagnostics.
func (c *Client) BaseURL() string { return c.baseURL }

// DryRun reports whether this client is in dry-run mode.
func (c *Client) DryRun() bool { return c.dryRun }

// HasKey reports whether this client was constructed with a non-empty
// bearer token. Callers can use this to decide whether to attempt a request
// at all (vs surfacing a "not configured" warning).
func (c *Client) HasKey() bool { return c.apiKey != "" }

// Me calls GET /v1/users/me. It is a free probe — no credits are spent —
// and is the canonical way to validate that a key is live.
func (c *Client) Me(ctx context.Context) (User, error) {
	body, err := c.do(ctx, http.MethodGet, "/users/me", nil)
	if err != nil {
		return User{}, err
	}
	var u User
	if err := json.Unmarshal(body, &u); err != nil {
		return User{}, fmt.Errorf("happenstance api: decoding /users/me: %w", err)
	}
	if u.Friends == nil {
		// Defensive: even if the JSON omits "friends" entirely, downstream
		// code expects a non-nil slice it can range over.
		u.Friends = []Friend{}
	}
	return u, nil
}
