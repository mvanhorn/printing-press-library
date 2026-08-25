// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package psx

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/psx/internal/cliutil"
)

// DefaultBaseURL is the PSX data portal. It publishes no API and requires no
// credentials; every endpoint here is unauthenticated plain HTTP.
const DefaultBaseURL = "https://dps.psx.com.pk"

// CorporateBaseURL is the PSX corporate site, used only for the document index.
const CorporateBaseURL = "https://www.psx.com.pk"

// defaultRate is the community-observed politeness ceiling for the portal
// (2 req/s, per psxdata/constants.go MAX_REQUESTS_PER_SECOND). PSX publishes no
// formal limit, so this is a self-imposed budget rather than a documented one.
const defaultRate = 2.0

// Client is a rate-limited HTTP client for PSX surfaces. It is a sibling of the
// generated internal/client and exists because the portal's useful endpoints
// return HTML table fragments and form-encoded POST bodies rather than JSON.
type Client struct {
	BaseURL string
	HTTP    *http.Client
	limiter *cliutil.AdaptiveLimiter
}

// NewWithRate builds a Client at an explicit request rate. A rate <= 0 falls
// back to the community-observed politeness ceiling; the CLI's --rate-limit
// flag flows through here so the advertised flag is not inert.
func NewWithRate(timeout time.Duration, ratePerSec float64) *Client {
	c := New(timeout)
	if ratePerSec > 0 {
		c.limiter = cliutil.NewAdaptiveLimiter(ratePerSec)
	}
	return c
}

// New returns a Client bound to the data portal with the portal's required
// request headers and an adaptive limiter seeded at the politeness ceiling.
func New(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Client{
		BaseURL: DefaultBaseURL,
		HTTP:    &http.Client{Timeout: timeout},
		limiter: cliutil.NewAdaptiveLimiter(defaultRate),
	}
}

// WithBaseURL returns a shallow copy bound to a different host, sharing the
// limiter so a fan-out across both PSX hosts stays inside one budget.
func (c *Client) WithBaseURL(base string) *Client {
	cp := *c
	cp.BaseURL = strings.TrimRight(base, "/")
	return &cp
}

func (c *Client) applyHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", DefaultBaseURL+"/")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
}

// do issues one paced request and converts HTTP 429 into a typed
// *cliutil.RateLimitError so throttling is never mistaken for "no data".
func (c *Client) do(req *http.Request) ([]byte, error) {
	c.limiter.Wait()
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting %s: %w", req.URL, err)
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if resp.StatusCode == http.StatusTooManyRequests {
		c.limiter.OnRateLimit()
		return nil, &cliutil.RateLimitError{
			URL:        req.URL.String(),
			RetryAfter: cliutil.RetryAfter(resp),
			Body:       truncate(string(body), 400),
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s returned HTTP %d", req.URL, resp.StatusCode)
	}
	if readErr != nil {
		return nil, fmt.Errorf("reading %s: %w", req.URL, readErr)
	}
	c.limiter.OnSuccess()
	return body, nil
}

// Get fetches a path and returns the raw body.
func (c *Client) Get(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	c.applyHeaders(req)
	return c.do(req)
}

// PostForm submits an application/x-www-form-urlencoded body. Several of the
// portal's richest surfaces (announcements, payouts, calendar, historical) are
// form POSTs rather than GETs.
func (c *Client) PostForm(ctx context.Context, path string, form url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	c.applyHeaders(req)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	return c.do(req)
}

// GetTables fetches a path and parses every HTML table in the response.
func (c *Client) GetTables(ctx context.Context, path string) ([]Table, error) {
	body, err := c.Get(ctx, path)
	if err != nil {
		return nil, err
	}
	return ParseTables(string(body))
}

// PostTables submits a form and parses every HTML table in the response.
func (c *Client) PostTables(ctx context.Context, path string, form url.Values) ([]Table, error) {
	body, err := c.PostForm(ctx, path, form)
	if err != nil {
		return nil, err
	}
	return ParseTables(string(body))
}

// Envelope is the portal's JSON wrapper: {"status":1,"message":"","data":[...]}.
type Envelope struct {
	Status  int             `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// GetEnvelope fetches a JSON endpoint and unwraps the portal envelope. A
// non-1 status carries the portal's own message rather than a generic failure.
func (c *Client) GetEnvelope(ctx context.Context, path string) (json.RawMessage, error) {
	body, err := c.Get(ctx, path)
	if err != nil {
		return nil, err
	}
	var env Envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if env.Status != 1 {
		return nil, fmt.Errorf("%s: portal status %d: %s", path, env.Status, orMessage(env.Message))
	}
	if len(env.Data) == 0 {
		return json.RawMessage("null"), nil
	}
	return env.Data, nil
}

// orMessage keeps a portal error legible when the envelope carries no text.
func orMessage(m string) string {
	if strings.TrimSpace(m) == "" {
		return "no message"
	}
	return m
}

// PostEnvelope submits a form to a JSON endpoint and unwraps the envelope.
func (c *Client) PostEnvelope(ctx context.Context, path string, form url.Values) (json.RawMessage, error) {
	body, err := c.PostForm(ctx, path, form)
	if err != nil {
		return nil, err
	}
	var env Envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if env.Status != 1 {
		return nil, fmt.Errorf("%s: portal status %d: %s", path, env.Status, orMessage(env.Message))
	}
	if len(env.Data) == 0 {
		return json.RawMessage("null"), nil
	}
	return env.Data, nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}
