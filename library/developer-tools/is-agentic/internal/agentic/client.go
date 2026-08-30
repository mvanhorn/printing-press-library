// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0.

package agentic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/is-agentic/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/is-agentic/internal/types"
)

const defaultOrigin = "https://is-agentic.com"

type Report struct {
	Target         string
	FetchedAt      time.Time
	Raw            []byte
	Parsed         types.PublicScanReport
	Issues         []types.PublicScanIssue
	ScoreBreakdown map[string]types.ScoreBucket
}

type Client struct {
	Origin  string
	HTTP    *http.Client
	Limiter *cliutil.AdaptiveLimiter
}

func New() *Client {
	origin := strings.TrimRight(defaultOrigin, "/")
	return &Client{Origin: origin, HTTP: &http.Client{Timeout: 30 * time.Second}, Limiter: cliutil.NewAdaptiveLimiterAuto(2)}
}

func NormalizeTarget(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("target is required; pass a domain or public HTTP(S) URL")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("target %q must be a public HTTP or HTTPS URL", raw)
	}
	if len(raw) > 2048 {
		return "", fmt.Errorf("target exceeds the API's 2048-character URL limit")
	}
	return u.String(), nil
}

func (c *Client) Fetch(ctx context.Context, target string) (*Report, error) {
	normalized, err := NormalizeTarget(target)
	if err != nil {
		return nil, err
	}
	endpoint, _ := url.Parse(c.Origin + "/api/v1/report")
	q := endpoint.Query()
	q.Set("url", normalized)
	endpoint.RawQuery = q.Encode()

	for attempt := 0; attempt < 2; attempt++ {
		c.Limiter.Wait()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "is-agentic-pp-cli/0.0.0")
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return nil, fmt.Errorf("requesting report for %s: %w", normalized, err)
		}
		if remaining, reset, ok := cliutil.ParseRateLimitHeaders(resp.Header); ok {
			c.Limiter.ObserveHeaders(remaining, reset)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("reading report response: %w", readErr)
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			c.Limiter.OnRateLimit()
			if attempt == 0 {
				wait := cliutil.RetryAfter(resp)
				timer := time.NewTimer(wait)
				select {
				case <-ctx.Done():
					timer.Stop()
					return nil, ctx.Err()
				case <-timer.C:
				}
				continue
			}
			return nil, &cliutil.RateLimitError{URL: endpoint.String(), RetryAfter: cliutil.RetryAfter(resp), Body: string(body)}
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			var problem types.ProblemDetails
			if json.Unmarshal(body, &problem) == nil && problem.Code != "" {
				return nil, &ProblemError{Status: resp.StatusCode, Problem: problem}
			}
			return nil, fmt.Errorf("Is Agentic returned HTTP %d for %s", resp.StatusCode, normalized)
		}
		c.Limiter.OnSuccess()
		return ParseReport(body, time.Now().UTC())
	}
	return nil, fmt.Errorf("report request exhausted retries")
}

func ParseReport(raw []byte, fetchedAt time.Time) (*Report, error) {
	var parsed types.PublicScanReport
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decoding report: %w", err)
	}
	var issues []types.PublicScanIssue
	if len(parsed.Issues) > 0 && string(parsed.Issues) != "null" {
		if err := json.Unmarshal(parsed.Issues, &issues); err != nil {
			return nil, fmt.Errorf("decoding report issues: %w", err)
		}
	}
	if issues == nil {
		issues = make([]types.PublicScanIssue, 0)
	}
	breakdown := make(map[string]types.ScoreBucket)
	if len(parsed.ScoreBreakdown) > 0 && string(parsed.ScoreBreakdown) != "null" {
		if err := json.Unmarshal(parsed.ScoreBreakdown, &breakdown); err != nil {
			return nil, fmt.Errorf("decoding score breakdown: %w", err)
		}
	}
	return &Report{Target: parsed.Target, FetchedAt: fetchedAt, Raw: append([]byte(nil), raw...), Parsed: parsed, Issues: issues, ScoreBreakdown: breakdown}, nil
}

type ProblemError struct {
	Status  int
	Problem types.ProblemDetails
}

func (e *ProblemError) Error() string {
	if e.Problem.Resolution != "" {
		return fmt.Sprintf("%s [%s]: %s; next: %s", e.Problem.Title, e.Problem.Code, e.Problem.Detail, e.Problem.Resolution)
	}
	return fmt.Sprintf("%s [%s]: %s", e.Problem.Title, e.Problem.Code, e.Problem.Detail)
}
