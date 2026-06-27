// Copyright 2026 not0xjarvis and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored aggregator source layer (not generator-emitted).
// See skills/printing-press/references/aggregator-pattern.md.

// Package source defines the cross-source contract for vibe-signal. Each
// upstream (Hacker News, Techmeme, ...) implements Source and registers itself
// from init(). The domain entity is a Signal: one observed item of conversation
// with its raw evidence preserved.
package source

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/vibe-signal/internal/cliutil"
)

// Signal is one observed item of conversation from a source. RawJSON preserves
// the verbatim upstream payload so synthesis never replaces citable evidence.
type Signal struct {
	Source      string    `json:"source"`
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	URL         string    `json:"url,omitempty"`
	Author      string    `json:"author,omitempty"`
	Points      int       `json:"points,omitempty"`
	Comments    int       `json:"comments,omitempty"`
	PublishedAt time.Time `json:"published_at"`
	Excerpt     string    `json:"excerpt,omitempty"`
	RawJSON     string    `json:"raw_json,omitempty"`
}

// SyncOptions bound a sync to a topic and recency window.
type SyncOptions struct {
	Query string
	Since time.Time
	Limit int
}

// Source is one upstream vibe-signal can pull from.
type Source interface {
	Name() string
	Description() string
	// AuthRequired reports whether the source needs credentials. v1 sources
	// are all false; credentialed sources (Product Hunt, YouTube) are deferred.
	AuthRequired() bool
	// TopicSearchable reports whether the source filters by query. Feed-only
	// sources (Techmeme river) return the current feed regardless of query.
	TopicSearchable() bool
	Sync(ctx context.Context, opts SyncOptions) ([]Signal, error)
}

// Fetch performs a rate-limit-aware GET. It surfaces a typed
// *cliutil.RateLimitError on 429 so callers never mistake throttling for "no
// data" — empty-on-throttle silently corrupts downstream queries.
func Fetch(ctx context.Context, limiter *cliutil.AdaptiveLimiter, url, userAgent string) ([]byte, error) {
	const maxAttempts = 3
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		limiter.Wait()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", userAgent)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			retry := cliutil.RetryAfter(resp)
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
			resp.Body.Close()
			limiter.OnRateLimit()
			lastErr = &cliutil.RateLimitError{URL: url, RetryAfter: retry, Body: string(body)}
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode >= 400 {
			return nil, &httpError{Status: resp.StatusCode, URL: url}
		}
		limiter.OnSuccess()
		return body, nil
	}
	return nil, lastErr
}

type httpError struct {
	Status int
	URL    string
}

func (e *httpError) Error() string {
	return http.StatusText(e.Status) + " (" + itoa(e.Status) + ") for " + e.URL
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
