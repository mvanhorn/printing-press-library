// Copyright 2026 Victor Wibisono and contributors. Licensed under Apache-2.0. See LICENSE.

// Package agoda is a hand-written client for the Agoda web GraphQL and cronos
// REST surfaces discovered during generation. Agoda disables GraphQL
// introspection, so the operation documents in queries/ were captured verbatim
// from live browser traffic and are replayed as-is.
package agoda

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/agoda/internal/cliutil"
)

const (
	// DefaultBaseURL is the only host Agoda's web API is served from.
	DefaultBaseURL = "https://www.agoda.com"

	// chromeUA matches the fingerprint the capture was taken under. Agoda does
	// not gate on it today, but sending a real browser UA keeps us on the same
	// code path we verified.
	chromeUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) " +
		"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"

	// maxBodyBytes caps a single response read. citySearch returns ~800KB for a
	// 49-property page; 32MB leaves headroom for a wide sweep without letting a
	// pathological response exhaust memory.
	maxBodyBytes = 32 << 20
)

// Client talks to Agoda's web API over plain HTTP.
//
// No browser and no clearance cookie are required: the public search,
// destination, review, and property surfaces were all verified to replay
// anonymously. Cookie is optional and only affects member-priced responses.
type Client struct {
	HTTP    *http.Client
	BaseURL string

	// Cookie, when non-empty, is sent verbatim as the Cookie header. It is only
	// needed for member pricing and account surfaces.
	Cookie string

	limiter *cliutil.AdaptiveLimiter
}

// New returns a Client paced at a deliberately conservative starting rate.
//
// Agoda sits behind Akamai with documented custom rate limiting, so the limiter
// starts slow and ramps on sustained success rather than opening at full speed.
func New(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &Client{
		HTTP:    &http.Client{Timeout: timeout},
		BaseURL: DefaultBaseURL,
		limiter: cliutil.NewAdaptiveLimiterAuto(1.5),
	}
}

// Rate reports the limiter's current requests-per-second pacing.
func (c *Client) Rate() float64 {
	if c == nil || c.limiter == nil {
		return 0
	}
	return c.limiter.Rate()
}

func (c *Client) baseURL() string {
	if c != nil && c.BaseURL != "" {
		return c.BaseURL
	}
	return DefaultBaseURL
}

// do performs one paced request and normalizes throttling into a typed error.
//
// Returning a *cliutil.RateLimitError rather than an empty result is deliberate:
// callers aggregating across many properties must be able to tell "Agoda has no
// data" apart from "Agoda throttled us", or averages silently absorb the gap.
func (c *Client) do(ctx context.Context, method, url string, body []byte, extraHeaders map[string]string) ([]byte, error) {
	c.limiter.Wait()

	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", chromeUA)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Origin", c.baseURL())
	req.Header.Set("Referer", c.baseURL()+"/")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Cookie != "" {
		req.Header.Set("Cookie", c.Cookie)
	}
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		c.limiter.OnRateLimit()
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, &cliutil.RateLimitError{
			URL:        url,
			RetryAfter: cliutil.RetryAfter(resp),
			Body:       string(snippet),
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("agoda returned HTTP %d for %s: %s", resp.StatusCode, url, string(snippet))
	}

	c.limiter.OnSuccess()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("reading response from %s: %w", url, err)
	}
	return data, nil
}

// gqlError mirrors the error envelope Agoda's GraphQL gateway returns.
type gqlError struct {
	Message string `json:"message"`
}

// postGraphQL sends one operation and surfaces GraphQL-level errors as Go errors.
//
// Agoda returns HTTP 200 with a populated "errors" array on failure, so checking
// the status code alone would silently treat a failed query as an empty result.
func (c *Client) postGraphQL(ctx context.Context, path string, payload []byte, pageTypeID string) ([]byte, error) {
	url := c.baseURL() + path
	headers := map[string]string{
		"AG-LANGUAGE-LOCALE": "en-us",
	}
	if pageTypeID != "" {
		headers["AG-PAGE-TYPE-ID"] = pageTypeID
	}
	data, err := c.do(ctx, http.MethodPost, url, payload, headers)
	if err != nil {
		return nil, err
	}
	var probe struct {
		Errors []gqlError `json:"errors"`
	}
	if err := json.Unmarshal(data, &probe); err == nil && len(probe.Errors) > 0 {
		return nil, fmt.Errorf("agoda graphql %s: %s", path, probe.Errors[0].Message)
	}
	return data, nil
}
