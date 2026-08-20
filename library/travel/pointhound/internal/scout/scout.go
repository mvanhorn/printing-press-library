// Package scout is a small client for Pointhound's airport autocomplete service
// at scout.pointhound.com. The endpoint is anonymous, deal-aware (returns a
// dealRating per airport), and used both directly by the `airports` command
// and indirectly by `explore-deal-rating`.
//
// This is hand-written rather than generated because scout.pointhound.com is a
// separate base URL from the spec's primary www.pointhound.com host.
package scout

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/pointhound/internal/cliutil"
)

// Place is one row in the Scout search results.
type Place struct {
	Rank         float64 `json:"rank"`
	Code         string  `json:"code"`
	Type         string  `json:"type"`
	Name         string  `json:"name"`
	City         string  `json:"city"`
	StateCode    string  `json:"stateCode"`
	StateName    string  `json:"stateName"`
	RegionName   string  `json:"regionName"`
	CountryCode  string  `json:"countryCode"`
	CountryName  string  `json:"countryName"`
	DealRating   string  `json:"dealRating"`
	SortPriority any     `json:"sortPriority"`
	IsTracked    bool    `json:"isTracked"`
}

// SearchResponse is the raw response shape returned by /places/search.
type SearchResponse struct {
	Results      []Place `json:"results"`
	SearchStatus string  `json:"searchStatus"`
}

// Client is a minimal Scout client.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	Limiter    *cliutil.AdaptiveLimiter
}

// New returns a Scout client. baseURL defaults to the production endpoint when
// empty. The default limiter paces requests at ~2 rps; pass a nil limiter via
// the field to disable.
func New(baseURL string) *Client {
	if baseURL == "" {
		baseURL = "https://scout.pointhound.com"
	}
	return &Client{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: 15 * time.Second},
		Limiter:    cliutil.NewAdaptiveLimiter(2),
	}
}

// SearchOptions configures a Scout /places/search call.
type SearchOptions struct {
	Query string
	Limit int
	Metro bool
	Bound bool
	Live  bool
}

// Search calls GET /places/search and returns the response.
func (c *Client) Search(ctx context.Context, opts SearchOptions) (*SearchResponse, error) {
	if opts.Query == "" {
		return nil, fmt.Errorf("scout: query is required")
	}
	if opts.Limit <= 0 {
		opts.Limit = 10
	}
	q := url.Values{}
	q.Set("q", opts.Query)
	q.Set("limit", fmt.Sprintf("%d", opts.Limit))
	q.Set("metro", fmt.Sprintf("%t", opts.Metro))
	q.Set("bound", fmt.Sprintf("%t", opts.Bound))
	q.Set("live", fmt.Sprintf("%t", opts.Live))
	q.Set("v", "2")

	endpoint := c.BaseURL + "/places/search?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("scout: building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "pointhound-pp-cli")

	// PATCH: Scout calls run under Cobra's command context, so rate-limit
	// waits must be interruptible on SIGINT.
	if err := c.Limiter.WaitContext(ctx); err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("scout: GET %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		c.Limiter.OnRateLimit()
		retryAfter := resp.Header.Get("Retry-After")
		if retryAfter != "" {
			return nil, fmt.Errorf("scout: GET /places/search rate limited (HTTP 429); retry after %s", retryAfter)
		}
		return nil, fmt.Errorf("scout: GET /places/search rate limited (HTTP 429)")
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("scout: GET /places/search returned HTTP %d", resp.StatusCode)
	}
	c.Limiter.OnSuccess()

	var out SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("scout: decoding response: %w", err)
	}
	return &out, nil
}
