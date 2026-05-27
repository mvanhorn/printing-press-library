// Copyright 2026 kothari-nikunj. Licensed under Apache-2.0. See LICENSE.

package parser

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hotel-goat/internal/cliutil"
)

// limiter paces outbound Google Hotels requests. Defaults to 2 req/sec
// to stay polite against the public SSR endpoint; honours 429s
// adaptively. Configurable via SetRateLimit (wired from the
// --rate-limit root flag).
var (
	limiterMu sync.Mutex
	limiter   = cliutil.NewAdaptiveLimiter(2)
)

// SetRateLimit replaces the package limiter. Pass 0 to disable rate-limiting.
func SetRateLimit(ratePerSec float64) {
	limiterMu.Lock()
	defer limiterMu.Unlock()
	limiter = cliutil.NewAdaptiveLimiter(ratePerSec)
}

func currentLimiter() *cliutil.AdaptiveLimiter {
	limiterMu.Lock()
	defer limiterMu.Unlock()
	return limiter
}

// DefaultUserAgent mimics a recent Chrome on macOS. Google's SSR returns
// the rich AF_initDataCallback payload only when the request looks like
// a real browser; bot-style UAs get an empty shell.
const DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"

// FetchHotelsHTML issues a plain HTTP GET against /travel/search with the
// query params Google's web UI uses. Returns the raw HTML body. Caller
// passes the location ("San Francisco", "hotels in Paris", etc.) — we
// always prepend "hotels in " when missing because Google's parser needs
// the lexical hint to dispatch to the hotel SERP.
func FetchHotelsHTML(ctx context.Context, httpClient *http.Client, location, checkin, checkout string, extra map[string]string) ([]byte, error) {
	q := normalizeLocation(location)
	params := url.Values{}
	params.Set("q", q)
	params.Set("checkin", checkin)
	params.Set("checkout", checkout)
	if _, has := extra["hl"]; !has {
		params.Set("hl", "en")
	}
	for k, v := range extra {
		if v != "" {
			params.Set(k, v)
		}
	}
	u := "https://www.google.com/travel/search?" + params.Encode()
	return doGet(ctx, httpClient, u)
}

// FetchPropertyDetailHTML fetches the detail page for one property_token.
func FetchPropertyDetailHTML(ctx context.Context, httpClient *http.Client, token, checkin, checkout string, extra map[string]string) ([]byte, error) {
	params := url.Values{}
	if checkin != "" {
		params.Set("checkin", checkin)
	}
	if checkout != "" {
		params.Set("checkout", checkout)
	}
	if _, has := extra["hl"]; !has {
		params.Set("hl", "en")
	}
	for k, v := range extra {
		if v != "" {
			params.Set(k, v)
		}
	}
	u := "https://www.google.com/travel/hotels/entity/" + url.PathEscape(token)
	if q := params.Encode(); q != "" {
		u += "?" + q
	}
	return doGet(ctx, httpClient, u)
}

func doGet(ctx context.Context, httpClient *http.Client, u string) ([]byte, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	lim := currentLimiter()
	lim.Wait()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", DefaultUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", u, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		lim.OnRateLimit()
		return nil, &RateLimitError{URL: u, StatusCode: resp.StatusCode}
	}
	lim.OnSuccess()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("GET %s: HTTP %d: %s", u, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	return body, nil
}

// RateLimitError signals Google returned 429. Callers should back off
// and surface this distinctly from "no results" so empty-on-throttle
// doesn't silently corrupt downstream queries.
type RateLimitError struct {
	URL        string
	StatusCode int
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limited by Google Hotels: HTTP %d for %s", e.StatusCode, e.URL)
}

func normalizeLocation(loc string) string {
	loc = strings.TrimSpace(loc)
	if loc == "" {
		return ""
	}
	lower := strings.ToLower(loc)
	if strings.HasPrefix(lower, "hotels in ") || strings.HasPrefix(lower, "hotels ") {
		return loc
	}
	return "hotels in " + loc
}
