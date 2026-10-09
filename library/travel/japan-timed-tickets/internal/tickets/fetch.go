// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

// Package tickets reads public ticket-sale rules and dated availability for
// Ghibli Museum (Mitaka), SHIBUYA SKY and teamLab venues in Japan.
//
// Every request is a read. The package never buys, adds to cart, joins a
// waiting room or logs in. It sends an honest, identifying User-Agent and
// never spoofs a browser.
package tickets

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-timed-tickets/internal/cliutil"
)

// UserAgent identifies this CLI to every source. It is not a browser string.
// Keep it a plain product token: Lawson Ticket resets HTTP/2 streams when the
// User-Agent carries a URL comment.
var UserAgent = "japan-timed-tickets-pp-cli/0.1.0"

const (
	maxBodyBytes = 4 << 20
	maxRedirects = 4
	maxAttempts  = 3
	acceptHTML   = "text/html,application/xhtml+xml"
	acceptJSON   = "application/json"
)

// Fetcher defaults.
const (
	// MaxPerHostRate is the request rate cap per source host (requests/second).
	MaxPerHostRate = 2.0
	// DefaultMaxRequests bounds the requests of one command when no budget is given.
	DefaultMaxRequests = 48
	// DefaultRequestTimeout bounds one request.
	DefaultRequestTimeout = 20 * time.Second
)

// errRedirectRefused marks a redirect the redirect policy refused.
var errRedirectRefused = errors.New("redirect refused")

// ErrWaitingRoom marks a source that redirected to a virtual waiting room.
// The CLI never follows such a redirect.
var ErrWaitingRoom = errors.New("source redirected to a virtual waiting room; not followed")

// SourceError is a non-throttle failure from one source request.
type SourceError struct {
	URL    string
	Status int
	Err    error
}

func (e *SourceError) Error() string {
	if e.Status > 0 {
		return fmt.Sprintf("%s: HTTP %d", e.URL, e.Status)
	}
	return fmt.Sprintf("%s: %v", e.URL, e.Err)
}

func (e *SourceError) Unwrap() error { return e.Err }

// Fetcher performs bounded, rate-limited reads with one limiter per host.
type Fetcher struct {
	HTTP     *http.Client
	rate     float64
	mu       sync.Mutex
	limiters map[string]*cliutil.AdaptiveLimiter
	requests int
	bytes    int64
	maxReqs  int
	// backoff is the wait before retry attempt n (tests shorten it).
	backoff func(attempt int) time.Duration
	// noPacing disables per-host pacing (tests only).
	noPacing bool
	// maxRetryWait caps an honored Retry-After; longer waits fall back to backoff.
	maxRetryWait time.Duration
}

// NewFetcher returns a Fetcher. ratePerSec <= 0 or above MaxPerHostRate uses
// MaxPerHostRate (callers warn about the clamp). maxRequests bounds the total
// requests one command may make (<= 0 uses DefaultMaxRequests).
func NewFetcher(timeout time.Duration, ratePerSec float64, maxRequests int) *Fetcher {
	if timeout <= 0 {
		timeout = DefaultRequestTimeout
	}
	if ratePerSec <= 0 || ratePerSec > MaxPerHostRate {
		ratePerSec = MaxPerHostRate
	}
	if maxRequests <= 0 {
		maxRequests = DefaultMaxRequests
	}
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Timeout: timeout,
		Jar:     jar,
	}
	f := &Fetcher{HTTP: client, rate: ratePerSec, limiters: map[string]*cliutil.AdaptiveLimiter{}, maxReqs: maxRequests, backoff: cliutil.Backoff, maxRetryWait: 10 * time.Second}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return fmt.Errorf("%w: stopped after %d redirects", errRedirectRefused, maxRedirects)
		}
		if isWaitingRoomHost(req.URL.Host) {
			return ErrWaitingRoom
		}
		// Follow redirects only within the same host and never to plain HTTP.
		if !strings.EqualFold(req.URL.Host, via[0].URL.Host) {
			return fmt.Errorf("%w: cross-host redirect from %q to %q", errRedirectRefused, via[0].URL.Host, req.URL.Host)
		}
		if via[0].URL.Scheme == "https" && req.URL.Scheme != "https" {
			return fmt.Errorf("%w: https to %q", errRedirectRefused, req.URL.Scheme)
		}
		// Each followed hop is a real request: count it against the
		// command budget and pace it with the same per-host limiter.
		if err := f.reserve(); err != nil {
			return fmt.Errorf("%w: %v", errRedirectRefused, err)
		}
		return f.limiter(req.URL.Host).Wait(req.Context())
	}
	return f
}

func isWaitingRoomHost(host string) bool {
	h := strings.ToLower(host)
	return strings.HasPrefix(h, "waiting.") || strings.Contains(h, "queue-it") || strings.Contains(h, "queueit")
}

func (f *Fetcher) limiter(host string) *cliutil.AdaptiveLimiter {
	if f.noPacing {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	l, ok := f.limiters[host]
	if !ok {
		l = cliutil.NewAdaptiveLimiter(f.rate)
		f.limiters[host] = l
	}
	return l
}

// Stats reports the requests made and bytes read so far.
func (f *Fetcher) Stats() (requests int, bytes int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests, f.bytes
}

func (f *Fetcher) reserve() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.requests >= f.maxReqs {
		return fmt.Errorf("request budget of %d reached for this command", f.maxReqs)
	}
	f.requests++
	return nil
}

// GetHTML reads an HTML page.
func (f *Fetcher) GetHTML(ctx context.Context, rawURL string) ([]byte, error) {
	return f.do(ctx, http.MethodGet, rawURL, acceptHTML, nil, "")
}

// GetJSON reads a JSON document.
func (f *Fetcher) GetJSON(ctx context.Context, rawURL string) ([]byte, error) {
	return f.do(ctx, http.MethodGet, rawURL, acceptJSON, nil, "")
}

// PostFormRead sends a read-only form POST that the official site itself uses
// to list data (for example the DMM time list). It is never used for orders.
func (f *Fetcher) PostFormRead(ctx context.Context, rawURL string, form url.Values, referer string) ([]byte, error) {
	return f.do(ctx, http.MethodPost, rawURL, acceptJSON, form, referer)
}

// FinalURL performs a GET and returns the URL after redirects.
func (f *Fetcher) FinalURL(ctx context.Context, rawURL string) (string, []byte, error) {
	var final string
	body, err := f.doWith(ctx, http.MethodGet, rawURL, acceptHTML, nil, "", func(resp *http.Response) {
		final = resp.Request.URL.String()
	})
	return final, body, err
}

func (f *Fetcher) do(ctx context.Context, method, rawURL, accept string, form url.Values, referer string) ([]byte, error) {
	return f.doWith(ctx, method, rawURL, accept, form, referer, nil)
}

func (f *Fetcher) doWith(ctx context.Context, method, rawURL, accept string, form url.Values, referer string, onResp func(*http.Response)) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	lim := f.limiter(u.Host)
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := lim.Wait(ctx); err != nil {
			return nil, err
		}
		if err := f.reserve(); err != nil {
			return nil, err
		}
		var bodyReader io.Reader
		if form != nil {
			bodyReader = strings.NewReader(form.Encode())
		}
		req, err := http.NewRequestWithContext(ctx, method, rawURL, bodyReader)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", UserAgent)
		req.Header.Set("Accept", accept)
		if form != nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("X-Requested-With", "XMLHttpRequest")
		}
		if referer != "" {
			req.Header.Set("Referer", referer)
		}
		resp, err := f.HTTP.Do(req)
		if err != nil {
			if errors.Is(err, ErrWaitingRoom) {
				return nil, &SourceError{URL: rawURL, Err: ErrWaitingRoom}
			}
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			var certErr *tls.CertificateVerificationError
			if errors.As(err, &certErr) || errors.Is(err, errRedirectRefused) {
				// Certificate and redirect-policy failures are final.
				return nil, &SourceError{URL: rawURL, Err: err}
			}
			// Some CDNs reset HTTP/2 streams on a first request; retry with backoff.
			lastErr = &SourceError{URL: rawURL, Err: err}
			if attempt < maxAttempts {
				if werr := sleepCtx(ctx, f.backoff(attempt)); werr != nil {
					return nil, werr
				}
			}
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
		_ = resp.Body.Close()
		f.mu.Lock()
		f.bytes += int64(len(body))
		f.mu.Unlock()
		if resp.StatusCode == http.StatusTooManyRequests {
			lim.OnRateLimit()
			retryAfter := cliutil.RetryAfter(resp)
			lastErr = &cliutil.RateLimitError{URL: rawURL, RetryAfter: retryAfter}
			if attempt < maxAttempts {
				wait := retryAfter
				if wait <= 0 || wait > f.maxRetryWait {
					wait = f.backoff(attempt)
				}
				if werr := sleepCtx(ctx, wait); werr != nil {
					return nil, werr
				}
			}
			continue
		}
		if resp.StatusCode >= 500 && attempt < maxAttempts {
			lastErr = &SourceError{URL: rawURL, Status: resp.StatusCode}
			if werr := sleepCtx(ctx, f.backoff(attempt)); werr != nil {
				return nil, werr
			}
			continue
		}
		if readErr != nil {
			return nil, &SourceError{URL: rawURL, Err: readErr}
		}
		if len(body) > maxBodyBytes {
			return nil, &SourceError{URL: rawURL, Err: fmt.Errorf("response larger than %d bytes", maxBodyBytes)}
		}
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return body, &SourceError{URL: rawURL, Status: resp.StatusCode}
		}
		lim.OnSuccess()
		if onResp != nil {
			onResp(resp)
		}
		return body, nil
	}
	return nil, lastErr
}

// StripControl removes C0/C1 control characters (except tab and newline) and
// Unicode format characters such as bidi overrides (except the zero-width
// joiner U+200D) from a remote string before it reaches a terminal or a file.
func StripControl(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\t' || r == '\n' || r == '\u200d':
			return r
		case r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f):
			return -1
		case unicode.Is(unicode.Cf, r):
			return -1
		}
		return r
	}, s)
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
