// Copyright 2026 paul-bockewitz. Licensed under Apache-2.0. See LICENSE.

package svc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/education/ankiweb/internal/cliutil"
)

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// Client is a thin HTTP wrapper for AnkiWeb's /svc/ protobuf endpoints. It
// sends a browser User-Agent, attaches a session cookie when configured, and
// paces requests through an adaptive limiter.
type Client struct {
	baseURL string
	cookies string
	http    *http.Client
	limiter *cliutil.AdaptiveLimiter
}

// New returns a Client for baseURL. cookies, when non-empty, is sent verbatim
// as the Cookie header on every request. ratePerSec <= 0 disables pacing.
func New(baseURL, cookies string, timeout time.Duration, ratePerSec float64) *Client {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		cookies: cookies,
		http:    &http.Client{Timeout: timeout},
		limiter: cliutil.NewAdaptiveLimiter(ratePerSec),
	}
}

// GetBytes performs a GET against path with the given query and returns the
// raw response body, HTTP status, and any transport error. A 429 after a
// single retry surfaces as *cliutil.RateLimitError.
func (c *Client) GetBytes(ctx context.Context, path string, query url.Values) ([]byte, int, error) {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	return c.do(ctx, http.MethodGet, u, nil)
}

// PostBytes performs a POST against path with an octet-stream body and returns
// the raw response body, HTTP status, and any transport error.
func (c *Client) PostBytes(ctx context.Context, path string, body []byte) ([]byte, int, error) {
	return c.do(ctx, http.MethodPost, c.baseURL+path, body)
}

func (c *Client) do(ctx context.Context, method, u string, body []byte) ([]byte, int, error) {
	// One retry on 429: back off per Retry-After, then surface a structured
	// rate-limit error so callers never mistake throttling for empty data.
	for attempt := 0; ; attempt++ {
		c.limiter.Wait()
		var rdr io.Reader
		if body != nil {
			rdr = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, u, rdr)
		if err != nil {
			return nil, 0, err
		}
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Accept", "application/octet-stream, */*")
		if body != nil {
			req.Header.Set("Content-Type", "application/octet-stream")
		}
		if c.cookies != "" {
			req.Header.Set("Cookie", c.cookies)
		}

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, 0, err
		}
		data, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, resp.StatusCode, readErr
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			c.limiter.OnRateLimit()
			if attempt < 1 {
				wait := cliutil.RetryAfter(resp)
				select {
				case <-ctx.Done():
					return nil, resp.StatusCode, ctx.Err()
				case <-time.After(wait):
				}
				continue
			}
			return nil, resp.StatusCode, &cliutil.RateLimitError{
				URL:        u,
				RetryAfter: cliutil.RetryAfter(resp),
				Body:       string(data),
			}
		}

		c.limiter.OnSuccess()
		if resp.StatusCode >= 400 {
			return data, resp.StatusCode, fmt.Errorf("HTTP %d for %s", resp.StatusCode, u)
		}
		return data, resp.StatusCode, nil
	}
}
