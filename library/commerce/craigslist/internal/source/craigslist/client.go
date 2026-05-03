// Package craigslist wraps the undocumented JSON endpoints used by Craigslist's own
// mobile app (sapi.craigslist.org, rapi.craigslist.org, reference.craigslist.org)
// plus the public sitemap surfaces. It is not the generated client.Client — that one
// targets sapi only and lacks cookie reuse and per-host routing.
//
// All requests go through cliutil.AdaptiveLimiter so 429/403 responses ramp the rate
// down and hCaptcha pages surface as typed errors instead of raw HTML.
package craigslist

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/craigslist/internal/cliutil"
)

// Default User-Agent matches a current Chrome on macOS so Craigslist's anti-bot
// system doesn't fingerprint us as a bot library on identity alone. The cl_b
// cookie that Craigslist auto-issues handles the rest.
const DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36"

// Hosts for the three Craigslist JSON surfaces and the city sitemap surface.
const (
	HostSAPI      = "https://sapi.craigslist.org/web/v8"
	HostRAPI      = "https://rapi.craigslist.org/web/v8"
	HostReference = "https://reference.craigslist.org"
)

// ErrBlocked is returned when Craigslist's anti-bot system serves a 403 with the
// "Your request has been blocked" body. Callers should back off — retrying the
// same URL within seconds will not unblock.
var ErrBlocked = errors.New("craigslist anti-bot block (403); back off and retry later")

// ErrChallenge is returned when the response contains an hCaptcha gate.
var ErrChallenge = errors.New("craigslist served an hCaptcha challenge; cannot proceed unattended")

// Client is the typed Craigslist HTTP client. It serializes a cl_b cookie across
// every request and runs through an AdaptiveLimiter for polite pacing.
type Client struct {
	HTTP      *http.Client
	UserAgent string
	limiter   *cliutil.AdaptiveLimiter
}

// New returns a fresh Client. ratePerSec=0 disables rate limiting; the polite
// default is 1.0 (1 req/s, ramped up automatically by AdaptiveLimiter on success).
func New(ratePerSec float64) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		HTTP: &http.Client{
			Timeout: 20 * time.Second,
			Jar:     jar,
		},
		UserAgent: DefaultUserAgent,
		limiter:   cliutil.NewAdaptiveLimiter(ratePerSec),
	}
}

// RawGet does a single GET, parsing path+params, returning the body bytes and
// a typed error on non-2xx. It does NOT decode JSON — callers do that against
// the typed schemas in sapi.go / rapi.go / reference.go. Auto-retries up to 3
// times on 429 + AdaptiveLimiter ramp-down. The 403 anti-bot and hCaptcha
// branches are mapped to *cliutil.RateLimitError so commands surface them as
// typed throttling rather than empty results.
func (c *Client) RawGet(ctx context.Context, base, path string, params url.Values) ([]byte, error) {
	if base == "" {
		return nil, fmt.Errorf("craigslist: empty base URL")
	}
	full := strings.TrimRight(base, "/") + path
	if len(params) > 0 {
		full += "?" + params.Encode()
	}
	const maxAttempts = 3
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		c.limiter.Wait()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", c.UserAgent)
		req.Header.Set("Accept", "application/json, text/xml, */*")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		resp, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = err
			c.limiter.OnRateLimit()
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		switch {
		case resp.StatusCode >= 200 && resp.StatusCode < 300:
			c.limiter.OnSuccess()
			return body, nil
		case resp.StatusCode == http.StatusTooManyRequests:
			c.limiter.OnRateLimit()
			lastErr = fmt.Errorf("craigslist: HTTP 429 from %s (attempt %d)", full, attempt)
			continue
		case resp.StatusCode == http.StatusForbidden:
			// Distinguish blocked (anti-bot) from generic 403.
			if strings.Contains(strings.ToLower(string(body)), "request has been blocked") {
				return body, &cliutil.RateLimitError{URL: full, Body: ErrBlocked.Error()}
			}
			if strings.Contains(strings.ToLower(string(body)), "hcaptcha") {
				return body, &cliutil.RateLimitError{URL: full, Body: ErrChallenge.Error()}
			}
			return body, fmt.Errorf("craigslist: HTTP 403 from %s: %s", full, snippet(body))
		case resp.StatusCode == http.StatusServiceUnavailable, resp.StatusCode == http.StatusBadGateway, resp.StatusCode == http.StatusGatewayTimeout:
			c.limiter.OnRateLimit()
			lastErr = fmt.Errorf("craigslist: HTTP %d from %s (attempt %d)", resp.StatusCode, full, attempt)
			continue
		default:
			return body, fmt.Errorf("craigslist: HTTP %d from %s: %s", resp.StatusCode, full, snippet(body))
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("craigslist: exhausted %d attempts to %s", maxAttempts, full)
	}
	return nil, &cliutil.RateLimitError{URL: full, Body: lastErr.Error()}
}

func snippet(body []byte) string {
	const max = 200
	if len(body) <= max {
		return string(body)
	}
	return string(body[:max]) + "..."
}
