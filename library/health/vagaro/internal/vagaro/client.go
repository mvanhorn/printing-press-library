// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Package vagaro is a hand-authored sibling client for Vagaro's consumer
// marketplace endpoints that the generated internal/client does not model:
// the /us02/websiteapi/homepage/ POST surface (services, staff, reviews,
// availability) and slug->businessID resolution from server-rendered HTML.
//
// It intentionally does not import internal/client. The generated client is
// tuned for the OpenAPI-derived REST surface; these endpoints need the
// ASP.NET {"d":...} envelope unwrap, browser-shaped headers, and HTML
// scraping that would not generalize back into the generator. Rate limiting
// and shared parsing helpers come from internal/cliutil so behavior matches
// the rest of the CLI.
package vagaro

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/vagaro/internal/cliutil"
)

const (
	defaultBaseURL = "https://www.vagaro.com"
	// websiteAPIPath is the shared prefix for the homepage POST surface.
	websiteAPIPath = "/us02/websiteapi/homepage/"
	// chromeUA mirrors the browser UA the generated client sends; Vagaro's
	// WAF buckets a script-shaped UA as a bot and answers with empty 5xx.
	chromeUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
)

// Endpoint method names on the websiteapi homepage surface.
const (
	MethodServices     = "getshopdetailcompositeservice"
	MethodStaff        = "getshopdetailcompositestaff"
	MethodReviews      = "getreviews"
	MethodAvailability = "getavailablemultiappointments"
)

// Client talks to Vagaro's consumer marketplace endpoints.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	limiter    *cliutil.AdaptiveLimiter
}

// New returns a Client. A non-positive timeout defaults to 60s; a
// non-positive rateLimit disables pacing (the limiter no-ops on nil).
func New(timeout time.Duration, rateLimit float64) *Client {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &Client{
		BaseURL:    defaultBaseURL,
		HTTPClient: &http.Client{Timeout: timeout},
		limiter:    cliutil.NewAdaptiveLimiter(rateLimit),
	}
}

// PostWebsiteAPI POSTs body as JSON to /us02/websiteapi/homepage/<method>
// and returns the response with the ASP.NET {"d":...} envelope unwrapped.
// When .d is itself a JSON-encoded string, the string is decoded so the
// caller sees the inner payload (JSON array/object, or an HTML fragment for
// endpoints that return markup inside .d).
func (c *Client) PostWebsiteAPI(ctx context.Context, method string, body any) (json.RawMessage, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshaling body: %w", err)
	}
	targetURL := c.BaseURL + websiteAPIPath + method
	raw, err := c.doPost(ctx, targetURL, b)
	if err != nil {
		return nil, err
	}
	return unwrapEnvelope(raw), nil
}

// ResolveBusinessID fetches GET /{slug} and parses the numeric businessID
// from the server-rendered HTML.
func (c *Client) ResolveBusinessID(ctx context.Context, slug string) (string, error) {
	html, err := c.fetchHTML(ctx, "/"+strings.Trim(slug, "/"))
	if err != nil {
		return "", err
	}
	id, err := ParseBusinessID(html)
	if err != nil {
		return "", fmt.Errorf("resolving businessID for %q: %w", slug, err)
	}
	return id, nil
}

// FetchProfile fetches GET /{slug} and returns the parsed SSR profile.
func (c *Client) FetchProfile(ctx context.Context, slug string) (BusinessProfile, error) {
	html, err := c.fetchHTML(ctx, "/"+strings.Trim(slug, "/"))
	if err != nil {
		return BusinessProfile{}, err
	}
	prof := ParseBusinessProfile(html)
	prof.Slug = strings.Trim(slug, "/")
	if prof.BusinessID == "" {
		return prof, fmt.Errorf("resolving businessID for %q: no businessID found in page", slug)
	}
	return prof, nil
}

func (c *Client) doPost(ctx context.Context, targetURL string, body []byte) ([]byte, error) {
	c.limiter.Wait()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Referer", c.BaseURL+"/")
	req.Header.Set("Origin", c.BaseURL)
	req.Header.Set("User-Agent", chromeUA)
	return c.readBody(ctx, req, targetURL)
}

func (c *Client) fetchHTML(ctx context.Context, path string) (string, error) {
	c.limiter.Wait()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("User-Agent", chromeUA)
	data, err := c.readBody(ctx, req, c.BaseURL+path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// readBody executes req, surfacing a *cliutil.RateLimitError on 429 so a
// throttle is never mistaken for empty results.
func (c *Client) readBody(ctx context.Context, req *http.Request, targetURL string) ([]byte, error) {
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("%s %s: %w", req.Method, targetURL, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		c.limiter.OnRateLimit()
		return nil, &cliutil.RateLimitError{
			URL:        targetURL,
			RetryAfter: cliutil.RetryAfter(resp),
			Body:       truncate(string(data), 200),
		}
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%s %s returned HTTP %d: %s", req.Method, targetURL, resp.StatusCode, truncate(string(data), 200))
	}
	c.limiter.OnSuccess()
	return data, nil
}

// unwrapEnvelope strips the ASP.NET {"d":...} wrapper. When .d is a
// JSON-encoded string it is decoded once so the caller sees the inner
// payload. Responses that are already a bare array/object (some endpoints
// return the payload directly) pass through unchanged.
func unwrapEnvelope(raw []byte) json.RawMessage {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return json.RawMessage("null")
	}
	if trimmed[0] == '{' {
		var env struct {
			D json.RawMessage `json:"d"`
		}
		if err := json.Unmarshal(trimmed, &env); err == nil && len(env.D) > 0 {
			d := bytes.TrimSpace(env.D)
			if len(d) > 0 && d[0] == '"' {
				var s string
				if err := json.Unmarshal(d, &s); err == nil {
					return json.RawMessage(s)
				}
			}
			return json.RawMessage(d)
		}
	}
	return json.RawMessage(trimmed)
}

// pickMode returns the highest-count key, breaking ties on the numerically
// larger value for determinism.
func pickMode(counts map[string]int) string {
	best := ""
	bestCount := 0
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) > len(keys[j])
		}
		return keys[i] > keys[j]
	})
	for _, k := range keys {
		if counts[k] > bestCount {
			bestCount = counts[k]
			best = k
		}
	}
	return best
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return strings.ToValidUTF8(s[:max], "") + "..."
}
