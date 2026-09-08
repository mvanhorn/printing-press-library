// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// Package pbsfetch retrieves PBS pages and release files over plain HTTP.
//
// Transport is deliberately ordinary: PBS answers every content URL with HTTP
// 200 over Go's net/http with no auth, no cookies, no Cloudflare challenge and
// no JavaScript. Measured across roughly sixty probes at 30-900ms. There is
// nothing here to work around, so nothing here works around anything.
package pbsfetch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/pbs/internal/cliutil"
)

// DefaultUserAgent identifies the CLI honestly.
//
// It deliberately does not impersonate a named AI crawler. PBS's robots.txt is
// fully permissive and carries no AI-crawler directives, but identifying as
// ClaudeBot or GPTBot would be a false claim about what is making the request.
const DefaultUserAgent = "pbs-pp-cli/1.0 (+https://github.com/mvanhorn/printing-press-library; Pakistani price-panel research CLI)"

// Result is one fetched artifact.
type Result struct {
	URL         string
	Status      int
	Body        []byte
	SHA256      string
	ContentType string
	Elapsed     time.Duration
}

// Client fetches PBS artifacts with pacing and bounded retries.
type Client struct {
	HTTP    *http.Client
	Limiter *cliutil.AdaptiveLimiter
	UA      string
	// MaxAttempts bounds retries per URL.
	MaxAttempts int
	// AllowedHosts is the closed set of hosts this client will fetch from.
	// Release URLs are scraped from a page, so without this a defaced index
	// could point the CLI at an arbitrary host and its bytes would be stored
	// as if they were PBS data.
	AllowedHosts []string
	// MaxBody caps one response body. Zero means MaxBodyBytes.
	//
	// This is a field for the same reason AllowedHosts is: the real cap is
	// far larger than any legitimate PBS artifact, so a test can only reach
	// it by lowering it here.
	MaxBody int64
}

// maxBody returns the effective body cap.
func (c *Client) maxBody() int64 {
	if c.MaxBody > 0 {
		return c.MaxBody
	}
	return MaxBodyBytes
}

// New returns a Client paced at ratePerSec requests per second.
func New(timeout time.Duration, ratePerSec float64) *Client {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	if ratePerSec <= 0 {
		ratePerSec = 2
	}
	c := &Client{
		HTTP:         &http.Client{Timeout: timeout},
		Limiter:      cliutil.NewAdaptiveLimiter(ratePerSec),
		UA:           DefaultUserAgent,
		MaxAttempts:  4,
		AllowedHosts: DefaultAllowedHosts,
		MaxBody:      MaxBodyBytes,
	}
	// The allowlist has to be re-applied to EVERY hop, not just the URL we
	// were handed. Go's default policy follows up to 10 redirects without
	// consulting checkOrigin again, so an allowed PBS URL could bounce the
	// fetch to loopback or a cloud metadata address and those bytes would be
	// stored as if they were PBS data. Closing over c rather than over the
	// slice keeps a test that narrows AllowedHosts authoritative on redirects
	// too.
	c.HTTP.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= MaxRedirects {
			return fmt.Errorf("%s: %w (over %d)", req.URL.String(), ErrTooManyRedirects, MaxRedirects)
		}
		return c.checkOrigin(req.URL.String())
	}
	return c
}

// MaxBodyBytes caps a single response body.
const MaxBodyBytes = 128 << 20

// MaxRedirects bounds one redirect chain.
//
// PBS serves every content URL directly, so a long chain means something other
// than a release file is being handed back.
const MaxRedirects = 5

// ErrBodyTooLarge reports a response body over MaxBodyBytes.
//
// The largest legitimate artifact this CLI fetches is a ~4 MB PDF annexure, so
// the cap cannot be reached by real PBS content. It exists so a hostile or
// broken upstream cannot exhaust memory before any parsing begins.
var ErrBodyTooLarge = errors.New("response body exceeds the maximum size")

// ErrTooManyRedirects reports a redirect chain longer than MaxRedirects.
var ErrTooManyRedirects = errors.New("too many redirects")

// ErrOffOrigin reports a URL that does not belong to the expected origin.
//
// Release URLs are SCRAPED from a page, so a defaced or compromised index could
// otherwise point the CLI at an arbitrary host — including a service on
// localhost — and the fetched bytes would be stored as if they were PBS data.
// No credentials are ever sent, so this is not a credential-leak risk; it is a
// data-integrity and blind-request one.
var ErrOffOrigin = errors.New("refusing a URL outside the expected origin")

// DefaultAllowedHosts are the only hosts a production client will fetch from.
//
// This is a Client field rather than a package constant so the client remains
// testable against a local server. It is set by the constructor and never from
// user input or from anything scraped, so it cannot be widened at runtime by a
// hostile index page — which is the whole point of the check.
var DefaultAllowedHosts = []string{"www.pbs.gov.pk", "pbs.gov.pk"}

// checkOrigin refuses a URL outside the client's allowed hosts.
func (c *Client) checkOrigin(raw string) error {
	u, err := neturl.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w: unparseable url %q", ErrOffOrigin, raw)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return fmt.Errorf("%w: scheme %q", ErrOffOrigin, u.Scheme)
	}
	h := strings.ToLower(u.Hostname())
	for _, allowed := range c.AllowedHosts {
		if h == strings.ToLower(allowed) {
			return nil
		}
	}
	return fmt.Errorf("%w: host %q", ErrOffOrigin, h)
}

// ErrNotFound reports a genuine upstream 404.
//
// PBS returns a real 404 status for a missing file rather than a soft 200, so
// this is trustworthy. It is a distinct error from a transport failure because
// the two mean different things for coverage: a 404 is upstream rot, a
// transport failure is our problem and must be retried, never recorded as an
// empty release.
var ErrNotFound = errors.New("upstream returned 404")

// Get fetches one URL.
//
// A non-2xx response is ALWAYS an error, never an empty success. Converting a
// failed fetch into a zero-row result is what punches silent holes in a panel,
// so the caller is forced to classify the outcome.
func (c *Client) Get(ctx context.Context, url string) (*Result, error) {
	var lastErr error
	attempts := c.MaxAttempts
	if attempts < 1 {
		attempts = 1
	}
	if err := c.checkOrigin(url); err != nil {
		return nil, err
	}
	for attempt := 0; attempt < attempts; attempt++ {
		if err := c.Limiter.Wait(ctx); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("build request for %s: %w", url, err)
		}
		req.Header.Set("User-Agent", c.UA)
		// A browser-shaped Accept matters on some hosts even without a
		// challenge; sending */* has produced 403s elsewhere in this family of
		// sources, so it is set explicitly rather than left to the default.
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")

		t0 := time.Now()
		resp, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("fetch %s: %w", url, err)
			// A redirect the policy refused is a decision, not a transient
			// fault. Retrying it re-walks the same chain and only burns the
			// backoff budget, so surface it immediately.
			if errors.Is(err, ErrOffOrigin) || errors.Is(err, ErrTooManyRedirects) {
				return nil, lastErr
			}
			if !sleepBackoff(ctx, attempt) {
				return nil, lastErr
			}
			continue
		}
		// Read one byte past the cap so an oversized body is detectable
		// rather than silently truncated into the panel.
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, c.maxBody()+1))
		_ = resp.Body.Close()
		elapsed := time.Since(t0)

		if remaining, resetAt, ok := cliutil.ParseRateLimitHeaders(resp.Header); ok {
			c.Limiter.ObserveHeaders(remaining, resetAt)
		}

		switch {
		case resp.StatusCode == http.StatusNotFound:
			return nil, fmt.Errorf("%s: %w", url, ErrNotFound)
		case resp.StatusCode == http.StatusTooManyRequests:
			c.Limiter.OnRateLimit()
			wait := cliutil.RetryAfter(resp)
			lastErr = &cliutil.RateLimitError{
				Cause:      fmt.Errorf("%s returned HTTP 429", url),
				RetryAfter: wait,
			}
			if attempt == attempts-1 {
				return nil, lastErr
			}
			if !sleepFor(ctx, wait, attempt) {
				return nil, lastErr
			}
			continue
		case resp.StatusCode >= 500:
			lastErr = fmt.Errorf("%s returned HTTP %d", url, resp.StatusCode)
			if !sleepBackoff(ctx, attempt) {
				return nil, lastErr
			}
			continue
		case resp.StatusCode != http.StatusOK:
			return nil, fmt.Errorf("%s returned HTTP %d", url, resp.StatusCode)
		}
		if readErr != nil {
			lastErr = fmt.Errorf("read body of %s: %w", url, readErr)
			if !sleepBackoff(ctx, attempt) {
				return nil, lastErr
			}
			continue
		}
		// Not retryable: a second attempt returns the same oversized body,
		// and a truncated artifact must never reach the parser.
		if int64(len(body)) > c.maxBody() {
			return nil, fmt.Errorf("%s: %w (over %d bytes)", url, ErrBodyTooLarge, c.maxBody())
		}
		c.Limiter.OnSuccess()
		sum := sha256.Sum256(body)
		return &Result{
			URL:         url,
			Status:      resp.StatusCode,
			Body:        body,
			SHA256:      hex.EncodeToString(sum[:]),
			ContentType: resp.Header.Get("Content-Type"),
			Elapsed:     elapsed,
		}, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("fetch %s: exhausted %d attempts", url, attempts)
	}
	return nil, lastErr
}

func sleepBackoff(ctx context.Context, attempt int) bool {
	return sleepFor(ctx, cliutil.Backoff(attempt), attempt)
}

func sleepFor(ctx context.Context, d time.Duration, attempt int) bool {
	if d <= 0 {
		d = cliutil.Backoff(attempt)
	}
	if d <= 0 {
		d = time.Second
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// LooksHTML reports whether a result is an HTML document.
func (r *Result) LooksHTML() bool {
	return strings.Contains(strings.ToLower(r.ContentType), "text/html")
}

// LooksXLSX reports whether a result is an Office Open XML workbook.
func (r *Result) LooksXLSX() bool {
	ct := strings.ToLower(r.ContentType)
	if strings.Contains(ct, "spreadsheetml") || strings.Contains(ct, "officedocument") {
		return true
	}
	// Fall back to the ZIP magic number: some hosts serve workbooks as
	// application/octet-stream.
	return len(r.Body) > 4 && r.Body[0] == 'P' && r.Body[1] == 'K'
}

// LooksPDF reports whether a result is a PDF.
func (r *Result) LooksPDF() bool {
	if strings.Contains(strings.ToLower(r.ContentType), "pdf") {
		return true
	}
	return len(r.Body) > 5 && string(r.Body[:5]) == "%PDF-"
}
