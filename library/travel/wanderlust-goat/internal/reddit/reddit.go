// Package reddit is a no-auth client for Reddit's public JSON search endpoint.
//
// Reddit blocks anonymous traffic aggressively, so the default User-Agent
// includes a contact URL and the default rate limit is 1 request/second.
package reddit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/httperr"
)

const (
	defaultBaseURL   = "https://www.reddit.com"
	defaultUserAgent = "wanderlust-goat-pp-cli/0.1 (+https://github.com/mvanhorn/printing-press-library)"
	defaultTimeout   = 10 * time.Second
	defaultLimit     = 25
)

// Thread is a single Reddit submission flattened from data.children[].data.
type Thread struct {
	ID          string  `json:"id"`
	Subreddit   string  `json:"subreddit"`
	Title       string  `json:"title"`
	URL         string  `json:"url"`
	Permalink   string  `json:"permalink"`
	Score       int     `json:"score"`
	NumComments int     `json:"num_comments"`
	Body        string  `json:"selftext"`
	CreatedUTC  float64 `json:"created_utc"`
}

// SearchOpts gates the result list with client-side filters. Reddit's API does
// not support score/comment thresholds as query params, so they are applied
// after the response lands.
type SearchOpts struct {
	MinScore      int
	MinComments   int
	Limit         int
	KeywordFilter []string
}

// Client is a polite, no-auth wrapper around www.reddit.com/r/<sr>/search.json.
type Client struct {
	BaseURL            string
	HTTPClient         *http.Client
	UserAgent          string
	RateLimitPerSecond float64

	mu       sync.Mutex
	lastSent time.Time
}

// New constructs a Client. Default rate limit is 1 req/sec to keep Reddit happy.
func New(httpClient *http.Client, ua string) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	if ua == "" {
		ua = defaultUserAgent
	}
	return &Client{
		BaseURL:            defaultBaseURL,
		HTTPClient:         httpClient,
		UserAgent:          ua,
		RateLimitPerSecond: 1.0,
	}
}

func (c *Client) throttle(ctx context.Context) error {
	if c.RateLimitPerSecond <= 0 {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	gap := time.Duration(float64(time.Second) / c.RateLimitPerSecond)
	wait := time.Until(c.lastSent.Add(gap))
	if wait > 0 {
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	c.lastSent = time.Now()
	return nil
}

// listingEnvelope mirrors Reddit's `{"data":{"children":[{"data":{...}}]}}` shape.
type listingEnvelope struct {
	Data struct {
		Children []struct {
			Data Thread `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

// Search runs a subreddit-restricted search and applies the SearchOpts filters
// client-side. The returned slice is freshly allocated and safe to mutate.
func (c *Client) Search(ctx context.Context, subreddit, query string, opts SearchOpts) ([]Thread, error) {
	if err := c.throttle(ctx); err != nil {
		return nil, err
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	q := url.Values{}
	q.Set("q", query)
	q.Set("restrict_sr", "1")
	q.Set("limit", strconv.Itoa(limit))
	q.Set("sort", "relevance")
	full := fmt.Sprintf("%s/r/%s/search.json?%s", strings.TrimRight(c.BaseURL, "/"), url.PathEscape(subreddit), q.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("reddit request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reddit read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned %d: %s", full, resp.StatusCode, httperr.Snippet(body))
	}

	var env listingEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("reddit parse json: %w", err)
	}

	out := make([]Thread, 0, len(env.Data.Children))
	for _, ch := range env.Data.Children {
		t := ch.Data
		if t.Score < opts.MinScore {
			continue
		}
		if t.NumComments < opts.MinComments {
			continue
		}
		if len(opts.KeywordFilter) > 0 && !matchesAnyKeyword(t.Title, t.Body, opts.KeywordFilter) {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

// matchesAnyKeyword returns true if any keyword (case-insensitive) appears as a
// substring of title+body.
func matchesAnyKeyword(title, body string, keywords []string) bool {
	hay := strings.ToLower(title + " " + body)
	for _, k := range keywords {
		if k == "" {
			continue
		}
		if strings.Contains(hay, strings.ToLower(k)) {
			return true
		}
	}
	return false
}
