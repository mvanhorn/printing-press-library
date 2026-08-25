// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

// Package source holds the hand-authored secondary-registry and enrichment
// clients (EU CTIS, OpenAlex, OpenFDA, RxNorm, PubMed, MeSH). Each is isolated
// behind a typed function so the core intelligence layer never depends on a
// single API's wire shape — Layer 1 of the architecture. Every outbound call
// rides the shared Client, which enforces adaptive rate limiting and surfaces
// *cliutil.RateLimitError when an upstream throttles, so "throttled" is never
// silently flattened into "no data".
package source

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/clinical-trials/internal/cliutil"
)

// Client is the shared HTTP transport for every secondary source. It is not the
// generated internal/client.Client (which is bound to the ClinicalTrials.gov
// base URL); secondary sources each have their own host, so they share this
// lightweight typed client instead.
type Client struct {
	http      *http.Client
	limiter   *cliutil.AdaptiveLimiter
	userAgent string
}

// New builds a source client. rate is requests/second (per source); 0 picks a
// polite default. PubMed and OpenFDA are most sensitive, so the default stays low.
func New(timeout time.Duration, rate float64) *Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if rate <= 0 {
		rate = 3
	}
	return &Client{
		http:      &http.Client{Timeout: timeout},
		limiter:   cliutil.NewAdaptiveLimiter(rate),
		userAgent: "clinical-trials-pp-cli/2.0.5 (+https://github.com/mvanhorn/printing-press-library)",
	}
}

const maxSourceRetries = 3

// GetJSON issues a GET and decodes the JSON body into out (may be nil to ignore
// the body). It retries 429/5xx with backoff and returns *cliutil.RateLimitError
// when 429 retries are exhausted.
func (c *Client) GetJSON(ctx context.Context, rawURL string, headers map[string]string, out any) error {
	body, err := c.do(ctx, http.MethodGet, rawURL, headers, nil)
	if err != nil {
		return err
	}
	return decodeInto(rawURL, body, out)
}

// PostJSON issues a POST with a JSON body and decodes the JSON response into out.
func (c *Client) PostJSON(ctx context.Context, rawURL string, headers map[string]string, reqBody any, out any) error {
	var payload []byte
	if reqBody != nil {
		b, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshaling request body: %w", err)
		}
		payload = b
	}
	body, err := c.do(ctx, http.MethodPost, rawURL, headers, payload)
	if err != nil {
		return err
	}
	return decodeInto(rawURL, body, out)
}

func decodeInto(rawURL string, body []byte, out any) error {
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decoding response from %s: %w", scrubURL(rawURL), err)
	}
	return nil
}

// sensitiveQueryKeys are query-parameter names whose values are secrets (API
// keys, tokens) and must never appear in an error message or log line.
var sensitiveQueryKeys = map[string]bool{
	"api_key":      true,
	"apikey":       true,
	"api-key":      true,
	"key":          true,
	"token":        true,
	"access_token": true,
	"auth":         true,
}

// scrubURL returns rawURL with the values of any sensitive query parameters
// replaced by REDACTED, so a URL can be safely embedded in an error message.
// Sources like OpenFDA and PubMed append &api_key=... to the request URL; that
// URL flows into transport-level errors here, so scrubbing centrally means no
// individual source has to remember to redact. If rawURL can't be parsed, the
// whole query string is dropped as a conservative fallback.
func scrubURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		if i := strings.IndexByte(rawURL, '?'); i >= 0 {
			return rawURL[:i] + "?REDACTED"
		}
		return rawURL
	}
	q := u.Query()
	redacted := false
	for k := range q {
		if sensitiveQueryKeys[strings.ToLower(k)] {
			q.Set(k, "REDACTED")
			redacted = true
		}
	}
	if redacted {
		u.RawQuery = q.Encode()
	}
	return u.String()
}

func (c *Client) do(ctx context.Context, method, rawURL string, headers map[string]string, payload []byte) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= maxSourceRetries; attempt++ {
		c.limiter.Wait()
		var reader io.Reader
		if payload != nil {
			reader = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, rawURL, reader)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("User-Agent", c.userAgent)
		req.Header.Set("Accept", "application/json")
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := c.http.Do(req)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			lastErr = fmt.Errorf("%s %s: %w", method, scrubURL(rawURL), err)
			if !sleepCtx(ctx, cliutil.Backoff(attempt)) {
				return nil, ctx.Err()
			}
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("reading response: %w", readErr)
		}
		switch {
		case resp.StatusCode < 400:
			c.limiter.OnSuccess()
			return body, nil
		case resp.StatusCode == 429:
			c.limiter.OnRateLimit()
			if attempt < maxSourceRetries {
				if !sleepCtx(ctx, cliutil.RetryAfter(resp)) {
					return nil, ctx.Err()
				}
				lastErr = &cliutil.RateLimitError{URL: scrubURL(rawURL), RetryAfter: cliutil.RetryAfter(resp), Body: truncate429(body)}
				continue
			}
			return nil, &cliutil.RateLimitError{URL: scrubURL(rawURL), RetryAfter: cliutil.RetryAfter(resp), Body: truncate429(body)}
		case resp.StatusCode >= 500:
			if attempt < maxSourceRetries {
				if !sleepCtx(ctx, cliutil.Backoff(attempt)) {
					return nil, ctx.Err()
				}
				lastErr = fmt.Errorf("%s %s: HTTP %d", method, scrubURL(rawURL), resp.StatusCode)
				continue
			}
			return nil, fmt.Errorf("%s %s: HTTP %d", method, scrubURL(rawURL), resp.StatusCode)
		default:
			return nil, fmt.Errorf("%s %s: HTTP %d: %s", method, scrubURL(rawURL), resp.StatusCode, truncate429(body))
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("%s %s: exhausted retries", method, scrubURL(rawURL))
	}
	return nil, lastErr
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func truncate429(b []byte) string {
	const max = 300
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "..."
}
