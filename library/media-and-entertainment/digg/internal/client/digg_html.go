// Hand-written client extensions for di.gg HTML pages and undocumented
// JSON endpoints that don't map to the OpenAPI surface the generator
// consumes. Lives alongside the generated client.go (which is owned by
// the Printing Press generator and carries a "DO NOT EDIT" header) so
// future regeneration doesn't blow these methods away.
//
// PATCH(library-side): added by U4 of the digg search/roster plan, then
// extended by U1 to host /api/search/stories alongside /ai/1000, then
// extended by U2 to host /api/search/users.

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

// searchUsersURL is the upstream user-search endpoint backing
// `authors get <handle>` lookups. Same host shape as
// /api/search/stories; verified by curl probe 2026-05-09. The base
// URL is overridable in tests via SearchUsersFrom.
const searchUsersURL = "https://di.gg/api/search/users"

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
// preserved here for parity with /api/search/stories so doctor / smoke
// checks can flag drift uniformly across the two endpoints.
type UsersSearchResponse struct {
	Query      string              `json:"query"`
	Results    []UsersSearchResult `json:"results"`
	Count      int                 `json:"count"`
	DurationMS int                 `json:"duration_ms"`
}

// clusterPostsURLBase is the base path for /ai/<clusterUrlId> page
// fetches. Joined with the caller-supplied clusterUrlId by
// FetchClusterPosts. Defined as a const so tests can substitute via
// FetchClusterPostsFrom.
const clusterPostsURLBase = "https://di.gg/ai"

// FetchClusterPosts GETs the /ai/<clusterUrlId> page, hands the HTML
// to the parser, and returns the structured posts (RSC array enriched
// with DOM-extracted bodies, media URLs, and repost-context chips).
//
// Same impersonation/timeout/rate-limit guarantees as the other
// page-level fetchers in this file. The parser may return partial
// results plus an error on a partial parse; we surface both so the
// caller can decide whether to render the partial slice or short-
// circuit.
func (c *Client) FetchClusterPosts(ctx context.Context, clusterUrlID string) ([]diggparse.ClusterPost, error) {
	return c.FetchClusterPostsFrom(ctx, clusterPostsURLBase+"/"+clusterUrlID)
}

// FetchClusterPostsFrom is FetchClusterPosts with a caller-supplied
// URL. Exists so unit tests can point at a local httptest server.
func (c *Client) FetchClusterPostsFrom(ctx context.Context, url string) ([]diggparse.ClusterPost, error) {
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
		return nil, fmt.Errorf("reading /ai/<id> body: %w", err)
	}
	posts, err := diggparse.ParseClusterPosts(body)
	// Parser returns partial results + error on partial parse; surface both.
	return posts, err
}

// SearchUsers hits Digg's undocumented /api/search/users endpoint —
// the JSON surface that powers handle lookups across the full
// 1000-plus-off-1000 author universe. Returns the upstream envelope
// unchanged so callers can use `current_rank` / `category` /
// `is_prefix_match` / `similarity_score` directly.
//
// query is required; an empty query is the caller's responsibility
// (upstream returns an empty results array, not an error). limit is
// sent as an upstream query param when > 0 — same shape as
// SearchStories; small limits are common here because the call site
// usually wants exact-or-top-fuzzy and capped fan-out.
func (c *Client) SearchUsers(ctx context.Context, query string, limit int) (*UsersSearchResponse, error) {
	return c.SearchUsersFrom(ctx, searchUsersURL, query, limit)
}

// SearchUsersFrom is SearchUsers with a caller-supplied base URL.
// Exists so unit tests can point at a local httptest server. Mirrors
// SearchStoriesFrom byte-for-byte except for the path constant; if
// the request shape ever diverges (auth headers, alt encoding,
// pagination cursor) split the helpers; until then keep them paired
// so a fix to one is obvious for the other.
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
