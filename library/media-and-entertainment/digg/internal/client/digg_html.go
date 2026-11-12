// Hand-written client extensions for di.gg HTML pages and undocumented
// JSON endpoints that don't map to the OpenAPI surface the generator
// consumes. Lives alongside the generated client.go (which is owned by
// the Printing Press generator and carries a "DO NOT EDIT" header) so
// future regeneration doesn't blow these methods away.
//
// PATCH(library-side): added by U4 of the digg search/roster plan, then
// extended by U2 to host /api/search/users. (/api/search/stories lived
// here too until Digg removed it in October 2026.)

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

// roster1000URL is the page that carries the 1000-account roster. It was
// /ai/1000 until Digg moved it under /tech in October 2026 (the old path
// now 308-redirects here). Defined as a const so tests can substitute via
// FetchRoster1000From.
const roster1000URL = "https://digg.com/tech/x/rankings"

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

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// searchUsersURL is the upstream user-search endpoint backing
// `authors get <handle>` lookups. Verified by curl probe 2026-05-09
// and still live 2026-10-21. The base URL is overridable in tests via
// SearchUsersFrom.
const searchUsersURL = "https://digg.com/api/search/users"

// UsersSearchResult is one entry in a /api/search/users response.
//
// `CurrentRank` and `Category` are pointers because the endpoint
// returns JSON null for off-1000 handles (verified via probe with
// `mvanhorn`: `current_rank: null`, `category: null`). Using *int /
// *string preserves the null-vs-zero distinction so the CLI can
// branch on tier_status without inferring it from a sentinel.
//
// `IsPrefixMatch` and `SimilarityScore` are returned for both
// in-1000 and off-1000 results; we surface them so callers can
// distinguish exact-prefix matches from fuzzier vector hits.
type UsersSearchResult struct {
	XID             string  `json:"x_id"`
	Username        string  `json:"username"`
	DisplayName     string  `json:"display_name"`
	ProfileImageURL string  `json:"profile_image_url"`
	FollowersCount  int     `json:"followers_count"`
	Category        *string `json:"category"`
	CurrentRank     *int    `json:"current_rank"`
	SimilarityScore float64 `json:"similarity_score"`
	IsPrefixMatch   bool    `json:"is_prefix_match"`
}

// UsersSearchResponse is the full envelope returned by
// /api/search/users. count and duration_ms are top-level upstream and
// preserved here so doctor / smoke checks can flag drift.
type UsersSearchResponse struct {
	Query      string              `json:"query"`
	Results    []UsersSearchResult `json:"results"`
	Count      int                 `json:"count"`
	DurationMS int                 `json:"duration_ms"`
}

// SearchUsers hits Digg's undocumented /api/search/users endpoint —
// the JSON surface that powers handle lookups across the full
// 1000-plus-off-1000 author universe. Returns the upstream envelope
// unchanged so callers can use `current_rank` / `category` /
// `is_prefix_match` / `similarity_score` directly.
//
// query is required; an empty query is the caller's responsibility
// (upstream returns an empty results array, not an error). limit is
// sent as an upstream query param when > 0; small limits are common
// here because the call site
// usually wants exact-or-top-fuzzy and capped fan-out.
func (c *Client) SearchUsers(ctx context.Context, query string, limit int) (*UsersSearchResponse, error) {
	return c.SearchUsersFrom(ctx, searchUsersURL, query, limit)
}

// SearchUsersFrom is SearchUsers with a caller-supplied base URL.
// Exists so unit tests can point at a local httptest server.
func (c *Client) SearchUsersFrom(ctx context.Context, baseURL, query string, limit int) (*UsersSearchResponse, error) {
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
		return nil, fmt.Errorf("reading /api/search/users body: %w", err)
	}
	var out UsersSearchResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decoding /api/search/users: %w", err)
	}
	return &out, nil
}
