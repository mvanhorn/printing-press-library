package jalan

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mvanhorn/printing-press-library/library/travel/jalan/internal/cliutil"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/transform"
)

type boundedReader struct {
	reader    io.Reader
	remaining int64
}

func (r *boundedReader) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		return 0, fmt.Errorf("response exceeds the configured size limit")
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	n, err := r.reader.Read(p)
	r.remaining -= int64(n)
	return n, err
}

type observation struct {
	URL        string    `json:"url"`
	ObservedAt time.Time `json:"observed_at"`
	Cache      string    `json:"cache_status"`
	CacheAgeMS int64     `json:"cache_age_ms"`
}

type requestSession struct {
	started      time.Time
	requests     int
	observations []observation
	warnings     []string
}

func (c *Client) newSession() *requestSession {
	return &requestSession{started: c.now(), observations: []observation{}, warnings: []string{}}
}

func (c *Client) fetch(ctx context.Context, sourceURL string, session *requestSession) (string, error) {
	if cached, ok := c.readCache(sourceURL); ok {
		session.observations = append(session.observations, observation{URL: sourceURL, ObservedAt: cached.ObservedAt, Cache: "hit", CacheAgeMS: c.now().Sub(cached.ObservedAt).Milliseconds()})
		return cached.Body, nil
	}
	select {
	case c.semaphore <- struct{}{}:
		defer func() { <-c.semaphore }()
	case <-ctx.Done():
		return "", &Error{Code: "timeout", Message: "source request budget expired", URL: sourceURL, Cause: ctx.Err()}
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := c.limiter.Wait(ctx); err != nil {
			return "", &Error{Code: "timeout", Message: "source request budget expired", URL: sourceURL, Cause: err}
		}
		reqCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, sourceURL, nil)
		if err != nil {
			cancel()
			return "", &Error{Code: "invalid_query", Message: "cannot construct source URL", URL: sourceURL, Cause: err}
		}
		req.Header.Set("User-Agent", "jalan-pp-cli/1.0 (anonymous read-only accommodation research)")
		req.Header.Set("Accept", "text/html")
		req.Header.Set("Accept-Language", "ja")
		session.requests++
		response, err := c.HTTP.Do(req)
		if err != nil {
			cancel()
			if ctx.Err() != nil {
				return "", &Error{Code: "timeout", Message: "source request budget expired", URL: sourceURL, Cause: ctx.Err()}
			}
			var typed *Error
			if errors.As(err, &typed) {
				return "", typed
			}
			if attempt == 0 {
				if err := waitRetry(ctx, 250*time.Millisecond); err != nil {
					return "", &Error{Code: "timeout", Message: "source retry budget expired", URL: sourceURL, Cause: err}
				}
				continue
			}
			return "", &Error{Code: "fetch_failure", Message: "anonymous source request failed", URL: sourceURL, Cause: err}
		}
		if remaining, resetAt, ok := cliutil.ParseRateLimitHeaders(response.Header); ok {
			c.limiter.ObserveHeaders(remaining, resetAt)
		}
		if response.StatusCode == http.StatusTooManyRequests {
			retryAfter := cliutil.RetryAfter(response)
			_ = response.Body.Close()
			cancel()
			c.limiter.OnRateLimit()
			rateErr := &cliutil.RateLimitError{URL: sourceURL, RetryAfter: retryAfter}
			if attempt == 1 || !retryFits(ctx, retryAfter) {
				return "", rateErr
			}
			if err := waitRetry(ctx, retryAfter); err != nil {
				rateErr.Cause = err
				return "", rateErr
			}
			continue
		}
		if response.StatusCode == 500 || response.StatusCode == 502 || response.StatusCode == 503 || response.StatusCode == 504 {
			delay := 250 * time.Millisecond
			if response.Header.Get("Retry-After") != "" {
				delay = cliutil.RetryAfter(response)
			}
			_ = response.Body.Close()
			cancel()
			if attempt == 0 && retryFits(ctx, delay) {
				if err := waitRetry(ctx, delay); err != nil {
					return "", &Error{Code: "timeout", Message: "source retry budget expired", URL: sourceURL, Cause: err}
				}
				continue
			}
			return "", &Error{Code: "fetch_failure", Message: fmt.Sprintf("source returned HTTP %d after bounded retries", response.StatusCode), URL: sourceURL}
		}
		if response.StatusCode != http.StatusOK {
			_ = response.Body.Close()
			cancel()
			return "", &Error{Code: "access_failure", Message: fmt.Sprintf("anonymous source returned HTTP %d", response.StatusCode), URL: sourceURL, Hint: "Open the source link to inspect access or unavailable-detail restrictions."}
		}
		raw, readErr := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
		contentType := response.Header.Get("Content-Type")
		_ = response.Body.Close()
		cancel()
		if readErr != nil {
			return "", &Error{Code: "fetch_failure", Message: "source response could not be read", URL: sourceURL, Cause: readErr}
		}
		if len(raw) > maxResponseBytes {
			return "", &Error{Code: "response_too_large", Message: "source response exceeds 4 MiB", URL: sourceURL}
		}
		body, decodeErr := decodeHTML(raw, contentType)
		if decodeErr != nil {
			return "", &Error{Code: "parse_failure", Message: "source character encoding could not be decoded", Hint: "Inspect the source Content-Type and document meta charset. " + decodeErr.Error(), URL: sourceURL, Cause: decodeErr}
		}
		if len(body) > maxResponseBytes*4 {
			return "", &Error{Code: "response_too_large", Message: "decoded source response exceeds 16 MiB", URL: sourceURL}
		}
		lower := strings.ToLower(body)
		if strings.Contains(lower, "<title>access denied") || strings.Contains(lower, "<title>403 forbidden") || strings.Contains(lower, "captcha") && !strings.Contains(lower, "p-searchresultitem") {
			return "", &Error{Code: "access_failure", Message: "source requires an access challenge", URL: sourceURL, Hint: "Open the source link; this CLI does not bypass source access controls."}
		}
		c.limiter.OnSuccess()
		at := c.now().UTC()
		session.observations = append(session.observations, observation{URL: sourceURL, ObservedAt: at, Cache: "live", CacheAgeMS: 0})
		if err := c.writeCache(cacheEntry{Version: cacheVersion, URL: sourceURL, ObservedAt: at, Body: body}); err != nil {
			session.warnings = append(session.warnings, "cache write unavailable; returned live source observation")
		}
		return body, nil
	}
	return "", &Error{Code: "fetch_failure", Message: "source retry budget exhausted", URL: sourceURL}
}

// decodeHTML follows document BOM, MIME and meta charset declarations. Script
// charset attributes describe only external scripts and cannot override HTML.
func decodeHTML(raw []byte, contentType string) (string, error) {
	encoding, name, _ := charset.DetermineEncoding(raw, contentType)
	// The standard UTF-8 decoder replaces malformed bytes. Fail explicitly so a
	// mislabeled or damaged document cannot silently become a quoted observation.
	if name == "utf-8" && !utf8.Valid(raw) {
		return "", fmt.Errorf("HTML document encoding %s: invalid UTF-8 bytes", name)
	}
	decoded, _, err := transform.Bytes(encoding.NewDecoder(), raw)
	if err != nil {
		return "", fmt.Errorf("HTML document encoding %s: %w", name, err)
	}
	return string(decoded), nil
}

func retryFits(ctx context.Context, delay time.Duration) bool {
	deadline, ok := ctx.Deadline()
	return !ok || time.Until(deadline) > delay+50*time.Millisecond
}
func waitRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
