// Copyright 2026 wayne-lai. Licensed under Apache-2.0. See LICENSE.

package fjud

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"judgementtw-pp-cli/internal/cliutil"
)

// BaseURL is the FJUD root.
const BaseURL = "https://judgment.judicial.gov.tw"

// UserAgent is the Chrome-style UA used for every outbound request. The site
// rejects empty/Go-default UA with anti-automation HTML.
const UserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// Client is a thin HTTP wrapper that maintains an ASP.NET session cookie jar
// across calls. All requests are stdlib net/http with adaptive rate-limiting.
type Client struct {
	httpClient *http.Client
	limiter    *cliutil.AdaptiveLimiter
}

// New builds a Client with a fresh cookie jar and the given per-second rate
// limit. Pass 0 to disable rate limiting.
func New(rate float64) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Jar:     jar,
		},
		limiter: cliutil.NewAdaptiveLimiter(rate),
	}
}

// SetHTTPClient lets tests inject a custom transport.
func (c *Client) SetHTTPClient(h *http.Client) {
	if h.Jar == nil {
		h.Jar = c.httpClient.Jar
	}
	c.httpClient = h
}

// fetch performs a GET or POST with the rate limiter and returns the response
// body bytes. The cookie jar is reused across calls so ASP.NET session cookies
// persist between the form-fetch and the form-submit.
func (c *Client) fetch(ctx context.Context, method, urlStr string, body io.Reader, contentType string, referer string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.limiter.Wait()
	req, err := http.NewRequestWithContext(ctx, method, urlStr, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-TW,zh;q=0.9,en;q=0.7")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP %s %s: %w", method, urlStr, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		c.limiter.OnRateLimit()
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, &cliutil.RateLimitError{
			URL:        urlStr,
			RetryAfter: cliutil.RetryAfter(resp),
			Body:       string(bodyBytes),
		}
	}
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("HTTP %s %s: %d %s — %s", method, urlStr, resp.StatusCode, resp.Status, string(bodyBytes))
	}
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}
	c.limiter.OnSuccess()
	return out, nil
}

// Get fetches a URL with no body.
func (c *Client) Get(ctx context.Context, urlStr string) ([]byte, error) {
	return c.fetch(ctx, http.MethodGet, urlStr, nil, "", "")
}

// PostForm submits a urlencoded form. The Referer header is set to the form's
// origin URL so ASP.NET ViewState validation passes.
func (c *Client) PostForm(ctx context.Context, urlStr string, form url.Values, referer string) ([]byte, error) {
	body := strings.NewReader(form.Encode())
	return c.fetch(ctx, http.MethodPost, urlStr, body, "application/x-www-form-urlencoded", referer)
}

// FetchPDF downloads a PDF attachment to memory. Used by `judgments get
// --with-pdf`.
func (c *Client) FetchPDF(ctx context.Context, pdfURL string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.limiter.Wait()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pdfURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "application/pdf,*/*;q=0.8")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP GET %s: %w", pdfURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP GET %s: %d %s", pdfURL, resp.StatusCode, resp.Status)
	}
	c.limiter.OnSuccess()
	return io.ReadAll(resp.Body)
}
