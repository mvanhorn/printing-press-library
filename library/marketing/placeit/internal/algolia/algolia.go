// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

// Package algolia is a minimal client for Placeit's public catalog search.
//
// Placeit's web app searches its template catalog through Algolia using a
// public, search-only API key embedded in the site's JavaScript bundle (the
// same class of credential as a Stripe publishable key — safe to ship, read
// only, no write or admin scope). Both values are overridable via the
// PLACEIT_ALGOLIA_APP_ID and PLACEIT_ALGOLIA_API_KEY environment variables.
//
// The Algolia host is not behind Cloudflare, so this client uses a plain
// net/http transport rather than the browser-compatible client the rest of
// the CLI uses for placeit.net.
package algolia

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/placeit/internal/cliutil"
)

// Public, search-only catalog credentials extracted from placeit.net's JS
// bundle. These are not secrets — they are published to every browser that
// loads the site and grant read-only search access to the public catalog.
const (
	defaultAppID = "KSLVR81FGG"
	// #nosec G101 -- public, search-only Algolia key published in placeit.net's
	// browser JS bundle (same class as a Stripe publishable key); read-only, no
	// write/admin scope. Overridable via PLACEIT_ALGOLIA_API_KEY.
	defaultAPIKey = "7dfd48b5c8d2820351a477db1aeab99f"

	// IndexMain is the primary published-templates index (~164k records).
	IndexMain = "Stage_production"
	// IndexNewest, IndexBestSelling, and IndexFree are sort replicas.
	IndexNewest      = "Stage_production_replica_newest"
	IndexBestSelling = "Stage_production_replica_best_selling"
	IndexFree        = "Stage_production_replica_free"
	// IndexIndustries is the 152-entry industry taxonomy.
	IndexIndustries = "Industries_production"

	userAgent = "placeit-pp-cli (+https://github.com/mvanhorn/printing-press-library)"
)

// defaultRatePerSec paces outbound Algolia queries. Catalog sync paginates
// with many back-to-back calls, so the shared AdaptiveLimiter ramps up on
// sustained success and backs off on 429.
const defaultRatePerSec = 10.0

// Client talks to Placeit's Algolia catalog.
type Client struct {
	AppID   string
	APIKey  string
	http    *http.Client
	limiter *cliutil.AdaptiveLimiter
}

// New returns a client using the public catalog credentials, overridable via
// PLACEIT_ALGOLIA_APP_ID / PLACEIT_ALGOLIA_API_KEY.
func New(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	appID := defaultAppID
	if v := strings.TrimSpace(os.Getenv("PLACEIT_ALGOLIA_APP_ID")); v != "" {
		appID = v
	}
	key := defaultAPIKey
	if v := strings.TrimSpace(os.Getenv("PLACEIT_ALGOLIA_API_KEY")); v != "" {
		key = v
	}
	return &Client{
		AppID:   appID,
		APIKey:  key,
		http:    &http.Client{Timeout: timeout},
		limiter: cliutil.NewAdaptiveLimiter(defaultRatePerSec),
	}
}

// SearchParams describes a single catalog query.
type SearchParams struct {
	Index        string     // defaults to IndexMain when empty
	Query        string     // free-text query ("" matches everything)
	HitsPerPage  int        // page size (defaults to 20, max 1000)
	Page         int        // zero-based page index
	FacetFilters [][]string // AND of ORs, e.g. [["category_name:Mockups"],["device_tags:T-Shirt"]]
	Filters      string     // numeric/boolean filter string, e.g. "is_free=1"
	Facets       []string   // facets to request distributions for
}

// SearchResult is the subset of the Algolia response the CLI consumes.
type SearchResult struct {
	Hits        []json.RawMessage         `json:"hits"`
	NbHits      int                       `json:"nbHits"`
	NbPages     int                       `json:"nbPages"`
	Page        int                       `json:"page"`
	HitsPerPage int                       `json:"hitsPerPage"`
	Facets      map[string]map[string]int `json:"facets"`
	Query       string                    `json:"query"`
	Index       string                    `json:"-"`
}

// Search runs one query against the given index (or the main index).
func (c *Client) Search(ctx context.Context, p SearchParams) (*SearchResult, error) {
	index := p.Index
	if index == "" {
		index = IndexMain
	}
	if p.HitsPerPage <= 0 {
		p.HitsPerPage = 20
	}
	if p.HitsPerPage > 1000 {
		p.HitsPerPage = 1000
	}

	body := map[string]any{
		"query":       p.Query,
		"hitsPerPage": p.HitsPerPage,
		"page":        p.Page,
	}
	if len(p.FacetFilters) > 0 {
		body["facetFilters"] = p.FacetFilters
	}
	if p.Filters != "" {
		body["filters"] = p.Filters
	}
	if len(p.Facets) > 0 {
		body["facets"] = p.Facets
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://%s-dsn.algolia.net/1/indexes/%s/query", c.AppID, index)

	// Catalog sync paginates with many back-to-back queries, so handle 429
	// (and Algolia's transient 5xx) with a bounded, Retry-After-aware backoff
	// instead of failing the whole sync on a single rate-limited page.
	data, err := c.doWithRetry(ctx, url, payload)
	if err != nil {
		return nil, err
	}
	var result SearchResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decoding algolia response: %w", err)
	}
	result.Index = index
	return &result, nil
}

// maxRetries bounds the rate-limit/transient backoff in doWithRetry.
const maxRetries = 4

// doWithRetry POSTs payload to url, pacing requests through the shared adaptive
// rate limiter and retrying HTTP 429 and transient 5xx with Retry-After-aware
// backoff. A 429 that survives all retries surfaces as a typed RateLimitError
// so callers fail loudly instead of treating throttling as "no results".
func (c *Client) doWithRetry(ctx context.Context, url string, payload []byte) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		c.limiter.Wait()

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-Algolia-Application-Id", c.AppID)
		req.Header.Set("X-Algolia-API-Key", c.APIKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", userAgent)

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("algolia request failed: %w", err)
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, 32<<20))

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			_ = resp.Body.Close()
			if readErr != nil {
				return nil, readErr
			}
			c.limiter.OnSuccess()
			return data, nil
		}

		msg := strings.TrimSpace(string(data))
		if len(msg) > 300 {
			msg = msg[:300]
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			c.limiter.OnRateLimit()
			wait := cliutil.RetryAfter(resp)
			_ = resp.Body.Close()
			lastErr = &cliutil.RateLimitError{URL: url, RetryAfter: wait, Body: msg}
			if attempt == maxRetries {
				break
			}
			if err := sleepCtx(ctx, wait); err != nil {
				return nil, err
			}
			continue
		}

		if resp.StatusCode >= 500 {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("algolia returned HTTP %d: %s", resp.StatusCode, msg)
			if attempt == maxRetries {
				break
			}
			if err := sleepCtx(ctx, cliutil.Backoff(attempt)); err != nil {
				return nil, err
			}
			continue
		}

		_ = resp.Body.Close()
		return nil, fmt.Errorf("algolia returned HTTP %d: %s", resp.StatusCode, msg)
	}
	return nil, lastErr
}

// sleepCtx waits for d or until ctx is cancelled, whichever comes first.
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

// IndexForSort maps a user-facing sort name to the backing replica index.
// Empty/relevance returns the main index.
func IndexForSort(sort string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(sort)) {
	case "", "relevance", "best-match":
		return IndexMain, nil
	case "newest", "new", "recent":
		return IndexNewest, nil
	case "best-selling", "bestselling", "popular", "best_selling":
		return IndexBestSelling, nil
	case "free":
		return IndexFree, nil
	default:
		return "", fmt.Errorf("unknown sort %q (want: relevance, newest, best-selling, free)", sort)
	}
}

// CategoryFacet maps a short category token to the catalog's category_name
// facet value. Empty input returns "" (no category filter).
func CategoryFacet(category string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "":
		return "", nil
	case "mockup", "mockups":
		return "Mockups", nil
	case "design", "designs", "design-templates", "design-template":
		return "Design Templates", nil
	case "logo", "logos":
		return "Logos", nil
	case "video", "videos":
		return "Videos", nil
	default:
		return "", fmt.Errorf("unknown category %q (want: mockups, designs, logos, videos)", category)
	}
}
