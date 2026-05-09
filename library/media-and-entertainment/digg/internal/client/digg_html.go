// Hand-written client extensions for di.gg HTML pages and undocumented
// JSON endpoints that don't map to the OpenAPI surface the generator
// consumes. Lives alongside the generated client.go (which is owned by
// the Printing Press generator and carries a "DO NOT EDIT" header) so
// future regeneration doesn't blow these methods away.
//
// PATCH(library-side): added by U4 of the digg search/roster plan, then
// extended by U1 to host /api/search/stories alongside /ai/1000.

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/diggparse"
)

// roster1000URL is the canonical /ai/1000 page URL. Lives on di.gg, not
// the API host the generated client points at. Defined as a const so
// tests can substitute via FetchRoster1000From.
const roster1000URL = "https://di.gg/ai/1000"

// searchStoriesURL is the upstream cluster-search endpoint backing the
// di.gg/ai Cmd+K modal. No published contract; verified by curl probe
// 2026-05-09 to return {query, results[], count, duration_ms}. The base
// URL is overridable in tests via SearchStoriesFrom.
const searchStoriesURL = "https://di.gg/api/search/stories"

// StoriesSearchResult is one entry in a /api/search/stories response.
// Mirrors the documented envelope verbatim so callers can pass through
// pagination-relevant fields (rank, postCount, uniqueAuthors) without
// reshaping. firstPostAge stays a string ("2d", "26d", "5h") because
// U3's --since filter parses it on the CLI side.
type StoriesSearchResult struct {
	ClusterID     string `json:"clusterId"`
	ClusterURLID  string `json:"clusterUrlId"`
	Rank          int    `json:"rank"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	PostCount     int    `json:"postCount"`
	UniqueAuthors int    `json:"uniqueAuthors"`
	FirstPostAge  string `json:"firstPostAge"`
}

// StoriesSearchResponse is the full envelope returned by
// /api/search/stories. count and duration_ms are top-level upstream and
// preserved here so doctor / smoke checks can flag drift.
type StoriesSearchResponse struct {
	Query      string                `json:"query"`
	Results    []StoriesSearchResult `json:"results"`
	Count      int                   `json:"count"`
	DurationMS int                   `json:"duration_ms"`
}

// FetchRoster1000 GETs the /ai/1000 page, hands the HTML to the RSC
// parser, and returns the structured roster. Uses the client's
// configured HTTP client (with the same impersonation, timeout, and
// rate-limit guarantees as JSON API calls) so live runs share one
// connection pool. No retries beyond what the underlying transport does
// — the page is large but stable; retrying a 5xx is more likely to
// hammer Digg than to recover.
func (c *Client) FetchRoster1000(ctx context.Context) ([]diggparse.Roster1000Author, error) {
	return c.FetchRoster1000From(ctx, roster1000URL)
}

// FetchRoster1000From is FetchRoster1000 with a caller-supplied URL.
// Exists so unit tests can point at a local httptest server.
func (c *Client) FetchRoster1000From(ctx context.Context, url string) ([]diggparse.Roster1000Author, error) {
	cctx, cancel := context.WithTimeout(ctx, c.ConfiguredTimeout())
	defer cancel()

	c.limiter.Wait()

	req, err := http.NewRequestWithContext(cctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "digg-pp-cli/0.1.0 (+https://github.com/mvanhorn/printing-press-library)")
	req.Header.Set("Accept", "text/html")

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading /ai/1000 body: %w", err)
	}
	authors, err := diggparse.ParseRoster1000(body)
	// Parser returns partial results + error on partial parse; surface both.
	return authors, err
}

// SearchStories hits Digg's undocumented /api/search/stories endpoint
// — the JSON surface backing the di.gg/ai Cmd+K modal. Returns the
// upstream envelope unchanged so callers can use rank/postCount/
// uniqueAuthors/firstPostAge directly.
//
// query is required; empty query is the caller's responsibility (the
// upstream returns an empty results array, not an error). limit is sent
// as an upstream query param when > 0 — verified by curl probe that the
// server honors `limit` (nolimit returns 11 results, limit=2 returns 2).
func (c *Client) SearchStories(ctx context.Context, query string, limit int) (*StoriesSearchResponse, error) {
	return c.SearchStoriesFrom(ctx, searchStoriesURL, query, limit)
}

// SearchStoriesFrom is SearchStories with a caller-supplied base URL.
// Exists so unit tests can point at a local httptest server.
func (c *Client) SearchStoriesFrom(ctx context.Context, baseURL, query string, limit int) (*StoriesSearchResponse, error) {
	cctx, cancel := context.WithTimeout(ctx, c.ConfiguredTimeout())
	defer cancel()

	c.limiter.Wait()

	q := url.Values{}
	q.Set("q", query)
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	full := baseURL + "?" + q.Encode()

	req, err := http.NewRequestWithContext(cctx, http.MethodGet, full, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "digg-pp-cli/0.1.0 (+https://github.com/mvanhorn/printing-press-library)")
	req.Header.Set("Accept", "application/json")

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", baseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET %s: HTTP %d: %s", baseURL, resp.StatusCode, truncate(string(body), 200))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading /api/search/stories body: %w", err)
	}
	var out StoriesSearchResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decoding /api/search/stories: %w", err)
	}
	return &out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
