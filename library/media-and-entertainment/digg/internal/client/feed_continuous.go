// Hand-written client for Digg's ranked story feed. Kept outside the
// generated client.go so regeneration does not clobber it.
//
// /api/feed/continuous is the JSON feed the live site's own bundles call.
// It is the one public source that carries each story's rank and Digg
// Score per topic, which `top` reads.

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/cliutil"
)

// feedContinuousURL is the feed endpoint. It has no published contract.
// Probed 2026-10-21: 20 stories per page, cursor pagination, and strict
// query validation (unknown params return HTTP 400). Requests go to
// digg.com directly to skip the di.gg 308 hop.
const feedContinuousURL = "https://digg.com/api/feed/continuous"

// FeedParams selects one feed listing. Topic is a Digg topic slug (ai,
// tech, world-business, ...); View is "top" or "trending"; Range is
// "24h", "7d", "30d", or "all".
type FeedParams struct {
	Topic string
	View  string
	Range string
}

// FeedSummary is the editorial headline block on a feed story.
type FeedSummary struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// FeedStory is one entry in a feed page's posts array. Rows carry no
// topic and no URL; the caller knows the topic it asked for.
type FeedStory struct {
	ClusterID     string      `json:"clusterId"`
	ClusterURLID  string      `json:"clusterUrlId"`
	Rank          int         `json:"rank"`
	DiggScore     float64     `json:"diggScore"`
	CreatedAt     string      `json:"createdAt"`
	PostCount     int         `json:"postCount"`
	UniqueAuthors int         `json:"uniqueAuthors"`
	Summary       FeedSummary `json:"summary"`
}

// FeedPage is one decoded /api/feed/continuous response.
type FeedPage struct {
	NextCursor *string     `json:"nextCursor"`
	Posts      []FeedStory `json:"posts"`
}

// Limiter returns the client's request pacer so other Digg endpoints
// called in the same run share one rate budget. Nil when pacing is off.
func (c *Client) Limiter() *cliutil.AdaptiveLimiter { return c.limiter }

// FetchFeedPage reads one page of /api/feed/continuous. cursor is empty
// for the first page, or the previous page's NextCursor. endpoint
// replaces the feed URL when non-empty, for tests.
func (c *Client) FetchFeedPage(ctx context.Context, endpoint string, p FeedParams, cursor string) (*FeedPage, error) {
	if endpoint == "" {
		endpoint = feedContinuousURL
	}
	if err := c.limiter.WaitContext(ctx); err != nil {
		return nil, err
	}
	cctx, cancel := context.WithTimeout(ctx, c.ConfiguredTimeout())
	defer cancel()

	q := url.Values{}
	q.Set("topic", p.Topic)
	q.Set("view", p.View)
	q.Set("range", p.Range)
	q.Set("published", "1")
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, endpoint+"?"+q.Encode(), nil)
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
		return nil, fmt.Errorf("GET %s: %w", endpoint, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading /api/feed/continuous body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusTooManyRequests {
			c.limiter.OnRateLimit()
		}
		return nil, fmt.Errorf("GET %s: HTTP %d: %s", endpoint, resp.StatusCode, truncate(string(body), 200))
	}
	c.limiter.OnSuccess()

	var probe struct {
		Posts json.RawMessage `json:"posts"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return nil, fmt.Errorf("decoding /api/feed/continuous: %w", err)
	}
	if len(probe.Posts) == 0 || string(probe.Posts) == "null" {
		return nil, fmt.Errorf("/api/feed/continuous response has no posts array; Digg's feed layout may have changed")
	}
	var page FeedPage
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, fmt.Errorf("decoding /api/feed/continuous: %w", err)
	}
	return &page, nil
}
