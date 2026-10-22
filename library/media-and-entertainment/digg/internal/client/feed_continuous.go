// Hand-written client for Digg's paginated story feed. Kept outside the
// generated client.go so regeneration does not clobber it.
//
// PATCH(digg-feed-continuous-replaces-removed-search-and-trending): Digg
// removed /api/search/stories and /api/trending/status in October 2026.
// /api/feed/continuous is the JSON feed the live site's own bundles call;
// search and sync are built on it.

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// feedContinuousURL is the feed endpoint. It has no published contract.
// Probed 2026-10-21: 20 stories per page, cursor pagination, and strict
// query validation (unknown params return HTTP 400). Requests go to
// digg.com directly to skip the di.gg 308 hop.
const feedContinuousURL = "https://digg.com/api/feed/continuous"

// FeedParams selects one feed listing. Topic is a Digg section slug
// (ai, tech, world-business, ...); View is "top" or "trending"; Range is
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

// FeedAuthor is one contributing X account on a feed story.
type FeedAuthor struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	XID         string `json:"xId"`
	Rank        int    `json:"rank"`
}

// FeedStory is one entry in a feed page's posts array.
type FeedStory struct {
	ClusterID        string          `json:"clusterId"`
	ClusterURLID     string          `json:"clusterUrlId"`
	Type             string          `json:"type"`
	EntryStatus      string          `json:"entryStatus"`
	Rank             int             `json:"rank"`
	PeakRank         *int            `json:"peakRank"`
	Delta            *int            `json:"delta"`
	DiggScore        float64         `json:"diggScore"`
	CreatedAt        string          `json:"createdAt"`
	LastSourcePostAt string          `json:"lastSourcePostAt"`
	PostCount        int             `json:"postCount"`
	UniqueAuthors    int             `json:"uniqueAuthors"`
	Likes            int             `json:"likes"`
	Views            int             `json:"views"`
	Comments         int             `json:"comments"`
	Quotes           int             `json:"quotes"`
	Replies          int             `json:"replies"`
	Bookmarks        int             `json:"bookmarks"`
	Summary          FeedSummary     `json:"summary"`
	Authors          []FeedAuthor    `json:"authors"`
	Raw              json.RawMessage `json:"-"`
}

// UnmarshalJSON keeps the raw story bytes alongside the decoded fields so
// the store can archive what Digg actually sent.
func (s *FeedStory) UnmarshalJSON(b []byte) error {
	type alias FeedStory
	var a alias
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	*s = FeedStory(a)
	s.Raw = append(json.RawMessage(nil), b...)
	return nil
}

// FeedPage is one decoded /api/feed/continuous response.
type FeedPage struct {
	NextCursor *string     `json:"nextCursor"`
	NowMs      int64       `json:"nowMs"`
	Posts      []FeedStory `json:"posts"`
	Range      string      `json:"range"`
	View       string      `json:"view"`
}

// FeedWalkResult says how a WalkFeed run ended. Complete means the feed
// ran out (null cursor or an empty later page); Capped means maxPages
// stopped it first.
type FeedWalkResult struct {
	Pages    int
	Stories  int
	Complete bool
	Capped   bool
}

// WalkFeed pages through /api/feed/continuous, calling fn for each page
// as it arrives, until the cursor ends or maxPages pages were fetched.
func (c *Client) WalkFeed(ctx context.Context, p FeedParams, maxPages int, fn func(*FeedPage) error) (FeedWalkResult, error) {
	return c.WalkFeedFrom(ctx, feedContinuousURL, p, maxPages, fn)
}

// WalkFeedFrom is WalkFeed with a caller-supplied endpoint URL, for tests.
// A page that fails part-way returns the pages already delivered in the
// result alongside the error, so callers can keep partial progress.
func (c *Client) WalkFeedFrom(ctx context.Context, endpoint string, p FeedParams, maxPages int, fn func(*FeedPage) error) (FeedWalkResult, error) {
	var res FeedWalkResult
	cursor := ""
	for {
		if maxPages > 0 && res.Pages >= maxPages {
			res.Capped = true
			return res, nil
		}
		page, err := c.fetchFeedPage(ctx, endpoint, p, cursor)
		if err != nil {
			return res, err
		}
		if len(page.Posts) == 0 {
			if res.Pages == 0 {
				return res, fmt.Errorf("feed %s/%s/%s returned no stories on its first page; Digg's feed layout may have changed", p.Topic, p.View, p.Range)
			}
			res.Pages++
			res.Complete = true
			return res, nil
		}
		res.Pages++
		res.Stories += len(page.Posts)
		if err := fn(page); err != nil {
			return res, err
		}
		if page.NextCursor == nil || *page.NextCursor == "" {
			res.Complete = true
			return res, nil
		}
		cursor = *page.NextCursor
	}
}

func (c *Client) fetchFeedPage(ctx context.Context, endpoint string, p FeedParams, cursor string) (*FeedPage, error) {
	cctx, cancel := context.WithTimeout(ctx, c.ConfiguredTimeout())
	defer cancel()

	c.limiter.Wait()

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
