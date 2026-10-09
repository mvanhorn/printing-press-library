// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

// Package sources fetches the three public, keyless sources used by
// japan-theme-parks-pp-cli: Queue-Times JSON, Tokyo Disney Resort ticket
// calendar pages (Chrome TLS fingerprint required) and the ThemeParks.wiki
// schedule API. Every call is read-only, paced and bounded.
package sources

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/enetx/surf"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/internal/cliutil"
)

const (
	userAgent     = "japan-theme-parks-pp-cli/0.1.0 (read-only trip planning CLI)"
	maxBodyBytes  = 8 << 20
	maxRedirects  = 5
	maxRetryAfter = 10 * time.Second // longest 429 wait honoured in-process

	QueueTimesBase = "https://queue-times.com"
	TDRBase        = "https://www.tokyodisneyresort.jp"
	ThemeParksBase = "https://api.themeparks.wiki/v1"
)

// Fetch is one completed source response.
type Fetch struct {
	URL       string
	FetchedAt time.Time
	Body      []byte
}

// Client holds one paced HTTP client per source.
type Client struct {
	std        *http.Client
	chromeOnce sync.Once
	chrome     *http.Client
	chromeErr  error
	limiters   map[string]*cliutil.AdaptiveLimiter
	timeout    time.Duration
}

// New returns a client with a per-request timeout. Each source keeps its own
// polite default rate; a positive maxRate (the --rate-limit flag) lowers it
// further, but never raises it. Zero or a negative value (auto) keeps the
// defaults: these are third-party sites, so pacing is not switched off.
func New(timeout time.Duration, maxRate float64) *Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	rate := func(def float64) float64 {
		if maxRate > 0 && maxRate < def {
			return maxRate
		}
		return def
	}
	return &Client{
		std:     &http.Client{Timeout: timeout, CheckRedirect: sameHostRedirects},
		timeout: timeout,
		limiters: map[string]*cliutil.AdaptiveLimiter{
			"queue-times": cliutil.NewAdaptiveLimiter(rate(2.0)),
			"tdr":         cliutil.NewAdaptiveLimiter(rate(1.0)),
			"themeparks":  cliutil.NewAdaptiveLimiter(rate(1.0)),
		},
	}
}

func (c *Client) chromeClient() (*http.Client, error) {
	c.chromeOnce.Do(func() {
		// surf skips certificate verification unless SecureTLS is set.
		sc, err := surf.NewClient().Builder().Impersonate().Chrome().SecureTLS().Timeout(c.timeout).Build().Result()
		if err != nil {
			c.chromeErr = fmt.Errorf("building Chrome-compatible transport: %w", err)
			return
		}
		std := sc.Std()
		std.Timeout = c.timeout
		std.Jar = nil
		std.CheckRedirect = sameHostRedirects
		c.chrome = std
	})
	return c.chrome, c.chromeErr
}

// QueueTimes fetches /parks/{id}/queue_times.json.
func (c *Client) QueueTimes(ctx context.Context, parkID int) (Fetch, error) {
	return c.get(ctx, "queue-times", c.std, fmt.Sprintf("%s/parks/%d/queue_times.json", QueueTimesBase, parkID), true)
}

// TDRMonth fetches a ticket calendar month page. lang is "ja" or "en".
func (c *Client) TDRMonth(ctx context.Context, yyyymm, lang string) (Fetch, error) {
	hc, err := c.chromeClient()
	if err != nil {
		return Fetch{}, err
	}
	return c.get(ctx, "tdr", hc, tdrMonthPage(yyyymm, lang), false)
}

// tdrMonthPage is the calendar page fetched for a month.
func tdrMonthPage(yyyymm, lang string) string {
	prefix := ""
	if lang == "en" {
		prefix = "/en"
	}
	return fmt.Sprintf("%s%s/ticket/index/%s/", TDRBase, prefix, yyyymm)
}

// TDRMonthURL is the human calendar URL for a month.
func TDRMonthURL(yyyymm, lang string) string {
	return tdrMonthPage(yyyymm, lang) + "#search-date"
}

// ThemeParksSchedule fetches /entity/{id}/schedule.
func (c *Client) ThemeParksSchedule(ctx context.Context, entityID string) (Fetch, error) {
	return c.get(ctx, "themeparks", c.std, fmt.Sprintf("%s/entity/%s/schedule", ThemeParksBase, entityID), true)
}

func (c *Client) get(ctx context.Context, source string, hc *http.Client, url string, jsonAPI bool) (Fetch, error) {
	fail := func(status int, err error) (Fetch, error) {
		return Fetch{}, &FetchError{Source: source, URL: url, Status: status, Err: err}
	}
	lim := c.limiters[source]
	var lastErr *FetchError
	for attempt := 0; attempt < 2; attempt++ {
		if err := lim.Wait(ctx); err != nil {
			return fail(0, err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return fail(0, err)
		}
		if jsonAPI {
			req.Header.Set("User-Agent", userAgent)
			req.Header.Set("Accept", "application/json")
		}
		resp, err := hc.Do(req)
		if err != nil {
			lastErr = &FetchError{Source: source, URL: url, Err: fmt.Errorf("GET %s: %w", url, err)}
			if ctx.Err() != nil || errors.Is(err, errCrossHostRedirect) || attempt == 1 || sleepCtx(ctx, cliutil.Backoff(attempt)) != nil {
				return Fetch{}, lastErr
			}
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
		_ = resp.Body.Close()
		switch {
		case resp.StatusCode == http.StatusTooManyRequests:
			lim.OnRateLimit()
			wait := cliutil.RetryAfter(resp)
			lastErr = &FetchError{Source: source, URL: url, Status: resp.StatusCode, Err: &cliutil.RateLimitError{URL: url, RetryAfter: wait}}
			if attempt == 0 && wait <= maxRetryAfter && sleepCtx(ctx, wait) == nil {
				continue
			}
			return Fetch{}, lastErr
		case readErr != nil:
			return fail(resp.StatusCode, fmt.Errorf("reading %s: %w", url, readErr))
		case len(body) > maxBodyBytes:
			return fail(resp.StatusCode, fmt.Errorf("response from %s exceeds %d bytes", url, maxBodyBytes))
		case resp.StatusCode >= 500 && attempt == 0:
			lastErr = &FetchError{Source: source, URL: url, Status: resp.StatusCode, Err: fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)}
			if sleepCtx(ctx, cliutil.Backoff(attempt)) != nil {
				return Fetch{}, lastErr
			}
			continue
		case resp.StatusCode != http.StatusOK:
			msg := fmt.Sprintf("GET %s: HTTP %d", url, resp.StatusCode)
			if cliutil.LooksLikeTransportBlock(string(body)) {
				msg += " (looks like a bot-protection page)"
			}
			return fail(resp.StatusCode, errors.New(msg))
		}
		lim.OnSuccess()
		return Fetch{URL: url, FetchedAt: time.Now().UTC().Truncate(time.Second), Body: body}, nil
	}
	return Fetch{}, lastErr
}

var errCrossHostRedirect = errors.New("redirect to a different host refused")

// sameHostRedirects follows at most maxRedirects redirects and only within
// the host of the first request.
func sameHostRedirects(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return fmt.Errorf("stopped after %d redirects", maxRedirects)
	}
	if len(via) > 0 && via[0].URL.Scheme == "https" && req.URL.Scheme != "https" {
		return fmt.Errorf("%w: https downgrade to %s", errCrossHostRedirect, req.URL.Scheme)
	}
	if len(via) > 0 && !strings.EqualFold(req.URL.Host, via[0].URL.Host) {
		return fmt.Errorf("%w: %s -> %s", errCrossHostRedirect, via[0].URL.Host, req.URL.Host)
	}
	return nil
}

// sleepCtx waits for d or until ctx is done.
func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// FetchError is every failure of a source request. A 429 wraps a
// *cliutil.RateLimitError, so errors.As still finds it.
type FetchError struct {
	Source string // limiter key: queue-times, tdr or themeparks
	URL    string
	Status int // HTTP status, 0 for transport errors
	Err    error
}

func (e *FetchError) Error() string { return e.Err.Error() }
func (e *FetchError) Unwrap() error { return e.Err }

// IsRateLimit reports whether err is a source 429.
func IsRateLimit(err error) bool {
	var rl *cliutil.RateLimitError
	return errors.As(err, &rl)
}

// ErrorURL returns the request URL of a source error, or "".
func ErrorURL(err error) string {
	var fe *FetchError
	if errors.As(err, &fe) {
		return fe.URL
	}
	return ""
}

// ErrorStatus returns the HTTP status of a source error, or 0.
func ErrorStatus(err error) int {
	var fe *FetchError
	if errors.As(err, &fe) {
		return fe.Status
	}
	return 0
}

// Describe returns a short, single-line error description for JSON output.
func Describe(err error) string {
	if err == nil {
		return ""
	}
	return strings.ReplaceAll(err.Error(), "\n", " ")
}
