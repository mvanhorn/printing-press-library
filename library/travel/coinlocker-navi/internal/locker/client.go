// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

// Package locker reads walk-up coin locker facts from コインロッカーなび
// (www.coinlocker-navi.com) and live reservable-empty counts from the public
// Multi Ekicube location API. All reads are on demand; nothing is mirrored.
package locker

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/net/publicsuffix"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/coinlocker-navi/internal/cliutil"
)

// Default origins. Tests override them on the Client.
const (
	DefaultNaviBase    = "https://www.coinlocker-navi.com"
	DefaultEkicubeBase = "https://api.multiecube.com"
	userAgent          = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36"
	maxBody            = 4 << 20
)

// ErrNotFound is returned when the source answers 404.
var ErrNotFound = errors.New("not found at source")

// HTTPError is a non-2xx answer other than 404 and 429.
type HTTPError struct {
	URL    string
	Status int
}

func (e *HTTPError) Error() string { return fmt.Sprintf("HTTP %d for %s", e.Status, e.URL) }

// Client is a paced, read-only client for both sources. Each host has its own
// limiter at about one request per second.
type Client struct {
	NaviBase    string
	EkicubeBase string
	HTTP        *http.Client

	mu       sync.Mutex
	rate     float64 // requests per second per host; 0 means 1.0 (tests raise it)
	limiters map[string]*cliutil.AdaptiveLimiter
	requests int
	urls     []string
}

// New returns a client with a cookie jar (the GPS search needs the CSRF cookie).
func New(timeout time.Duration) *Client {
	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &Client{
		NaviBase:    DefaultNaviBase,
		EkicubeBase: DefaultEkicubeBase,
		HTTP:        &http.Client{Jar: jar, Timeout: timeout, CheckRedirect: sameHostRedirect},
		limiters:    map[string]*cliutil.AdaptiveLimiter{},
	}
}

// Requests reports how many HTTP requests were sent.
func (c *Client) Requests() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.requests
}

// SourceURLs reports the distinct URLs read, in order.
func (c *Client) SourceURLs() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.urls))
	copy(out, c.urls)
	return out
}

func (c *Client) limiter(host string) *cliutil.AdaptiveLimiter {
	c.mu.Lock()
	defer c.mu.Unlock()
	l, ok := c.limiters[host]
	if !ok {
		rate := c.rate
		if rate <= 0 {
			rate = 1.0
		}
		l = cliutil.NewAdaptiveLimiter(rate)
		c.limiters[host] = l
	}
	return l
}

func (c *Client) record(u string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requests++
	for _, x := range c.urls {
		if x == u {
			return
		}
	}
	c.urls = append(c.urls, u)
}

func (c *Client) do(ctx context.Context, req *http.Request) ([]byte, error) {
	l := c.limiter(req.URL.Host)
	if l != nil {
		if err := l.Wait(ctx); err != nil {
			return nil, err
		}
	}
	req.Header.Set("User-Agent", userAgent)
	if req.Header.Get("Accept-Language") == "" {
		req.Header.Set("Accept-Language", "ja,en;q=0.8")
	}
	c.record(req.URL.String())
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxBody {
		return nil, fmt.Errorf("%s: response larger than %d bytes; refusing a cut page", req.URL.Host, maxBody)
	}
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		if l != nil {
			l.OnRateLimit()
		}
		return nil, &cliutil.RateLimitError{URL: req.URL.String(), RetryAfter: cliutil.RetryAfter(resp)}
	case resp.StatusCode == http.StatusNotFound:
		return nil, ErrNotFound
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		return nil, &HTTPError{URL: req.URL.String(), Status: resp.StatusCode}
	}
	if l != nil {
		l.OnSuccess()
	}
	return body, nil
}

// fetch fetches a URL.
func (c *Client) fetch(ctx context.Context, rawURL string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.do(ctx, req)
}

// postForm sends a urlencoded form (read-only search endpoints only).
func (c *Client) postForm(ctx context.Context, rawURL string, form url.Values, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.do(ctx, req)
}

// sameHostRedirect follows at most 5 redirects, and only to the same host
// over HTTPS (or plain HTTP when the first request was plain HTTP, as in
// tests). A redirect to another host is an error, not a silent follow.
func sameHostRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return fmt.Errorf("stopped after %d redirects", len(via))
	}
	first := via[0].URL
	if req.URL.Host != first.Host {
		return fmt.Errorf("refusing redirect from %s to another host %s", first.Host, req.URL.Host)
	}
	if first.Scheme == "https" && req.URL.Scheme != "https" {
		return fmt.Errorf("refusing redirect from https to %s", req.URL.Scheme)
	}
	return nil
}
