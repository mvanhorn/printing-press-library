// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

// Package pexels is a hand-authored typed client for the Pexels API
// (https://api.pexels.com/v1). It wraps an AdaptiveLimiter for outbound
// pacing and surfaces *cliutil.RateLimitError on HTTP 429 so callers never
// confuse throttling with an empty result set.
package pexels

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/pexels/internal/cliutil"
)

const (
	baseURL   = "https://api.pexels.com/v1"
	userAgent = "pexels-pp-cli/0.1 (+https://github.com/mvanhorn/printing-press-library)"
)

// RateInfo carries the X-Ratelimit-* values parsed from a 2xx response.
// Known is false when the upstream omitted the headers.
type RateInfo struct {
	Limit     int64
	Remaining int64
	Reset     int64
	Known     bool
}

// Client is a typed Pexels API client. The zero value is not usable; call New.
type Client struct {
	http      *http.Client
	apiKey    string
	userAgent string
	baseURL   string
	limiter   *cliutil.AdaptiveLimiter
}

// New returns a Client. apiKey may be empty: Pexels read endpoints work
// without a key, in which case no Authorization header is sent.
func New(apiKey string) *Client {
	return &Client{
		http: &http.Client{
			// Per-request deadlines come from the caller's context, not a
			// fixed client timeout, so a generous --timeout can govern.
			Transport: http.DefaultTransport,
		},
		apiKey:    apiKey,
		userAgent: userAgent,
		baseURL:   baseURL,
		limiter:   cliutil.NewAdaptiveLimiter(3.0),
	}
}

// HTTPClient exposes the underlying *http.Client so callers (e.g. the download
// command) can fetch media bytes through the same transport.
func (c *Client) HTTPClient() *http.Client { return c.http }

// UserAgent returns the User-Agent string the client sends.
func (c *Client) UserAgent() string { return c.userAgent }

// APIKey returns the configured API key (may be empty).
func (c *Client) APIKey() string { return c.apiKey }

// Get performs a GET against baseURL+path with the given query params.
// On 429 it returns a *cliutil.RateLimitError. On other non-2xx it returns
// the body plus a descriptive error. On 2xx it returns the body, parsed
// RateInfo, and persists the snapshot to the rate ledger (best-effort).
func (c *Client) Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, RateInfo, error) {
	c.limiter.Wait()

	u := c.baseURL + path
	if len(params) > 0 {
		q := url.Values{}
		for k, v := range params {
			q.Set(k, v)
		}
		u += "?" + q.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, RateInfo{}, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		// Pexels uses the raw key — NO "Bearer" prefix.
		req.Header.Set("Authorization", c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, RateInfo{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusTooManyRequests {
		c.limiter.OnRateLimit()
		return nil, RateInfo{}, &cliutil.RateLimitError{
			URL:        u,
			RetryAfter: cliutil.RetryAfter(resp),
			Body:       snippet(body, 200),
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return json.RawMessage(body), RateInfo{}, fmt.Errorf("pexels GET %s: HTTP %d: %s", path, resp.StatusCode, snippet(body, 200))
	}

	c.limiter.OnSuccess()
	ri := parseRateInfo(resp.Header)
	// Persist the latest rate snapshot for `quota forecast`. Best-effort.
	_ = SaveRate(ri)
	return json.RawMessage(body), ri, nil
}

func parseRateInfo(h http.Header) RateInfo {
	var ri RateInfo
	limitStr := h.Get("X-Ratelimit-Limit")
	if limitStr == "" {
		return ri
	}
	ri.Known = true
	ri.Limit = parseInt64(limitStr)
	ri.Remaining = parseInt64(h.Get("X-Ratelimit-Remaining"))
	ri.Reset = parseInt64(h.Get("X-Ratelimit-Reset"))
	return ri
}

func parseInt64(s string) int64 {
	if s == "" {
		return 0
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func snippet(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n])
	}
	return string(b)
}
