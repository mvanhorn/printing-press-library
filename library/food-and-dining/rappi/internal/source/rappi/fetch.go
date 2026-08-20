// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
package rappi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/rappi/internal/cliutil"
)

// DefaultUserAgent is the real-Chrome User-Agent that browser-sniff
// confirmed gives 200 OK across Rappi's SSR surface. Plain Go-http-client
// returns 200 OK from datacenter IPs but a real-Chrome UA both improves
// cache hit rate and is what every commercial Rappi scraper uses.
const DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// DefaultBaseURL is rappi.com.mx, the only host this CLI targets.
const DefaultBaseURL = "https://www.rappi.com.mx"

// DefaultRateLimit caps outbound requests to Rappi's SSR surface at a
// conservative 2 req/sec. AdaptiveLimiter ramps up after consecutive
// successes and halves on a 429, so the steady-state rate self-tunes.
const DefaultRateLimit = 2.0

// Client wraps an HTTP client with the headers Rappi expects. Always
// resilient to nil HTTPClient (falls back to http.DefaultClient).
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	UserAgent  string
	Limiter    *cliutil.AdaptiveLimiter
}

// NewClient builds a Client with sensible defaults: 30-second timeout,
// real-Chrome UA, Spanish Accept-Language, and a shared adaptive rate
// limiter starting at DefaultRateLimit req/sec.
func NewClient() *Client {
	return &Client{
		BaseURL:   DefaultBaseURL,
		UserAgent: DefaultUserAgent,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		Limiter: cliutil.NewAdaptiveLimiter(DefaultRateLimit),
	}
}

// FetchHTML retrieves the page at relPath (or absolute URL) with the
// browser-sniffed headers and returns the raw HTML body. Status >= 400
// produces an error including the response code; rate-limit 429s are
// surfaced explicitly so callers can back off. The adaptive limiter
// paces requests, ramps up on success, and halves on 429.
func (c *Client) FetchHTML(ctx context.Context, relPath string) ([]byte, error) {
	u := relPath
	if len(u) == 0 || u[0] == '/' {
		u = c.BaseURL + u
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("build request for %s: %w", u, err)
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "es-MX,es;q=0.9,en;q=0.8")
	req.Header.Set("Accept-Encoding", "identity")
	hc := c.HTTPClient
	if hc == nil {
		hc = http.DefaultClient
	}
	// Pace the request through the adaptive limiter (no-op if nil).
	c.Limiter.Wait()
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", u, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", u, err)
	}
	if resp.StatusCode == 429 {
		c.Limiter.OnRateLimit()
		return body, &cliutil.RateLimitError{
			URL:        u,
			RetryAfter: cliutil.RetryAfter(resp),
			Body:       string(body),
		}
	}
	if resp.StatusCode >= 400 {
		return body, fmt.Errorf("rappi HTTP %d at %s", resp.StatusCode, u)
	}
	c.Limiter.OnSuccess()
	return body, nil
}
