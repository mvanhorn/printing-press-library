// steam.go — typed keyless Steam storefront client.
//
// Steam is keyless: no credential, no auth header. Three live-probed
// storefront endpoints power the enrichment:
//   - /api/storesearch  (title -> appid)
//   - /appreviews/<id>   (review score rollup)
//   - /api/appdetails   (price, fetched only when ratings needs it)

package steam

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultBaseURL is the Steam storefront host every endpoint lives on.
	DefaultBaseURL = "https://store.steampowered.com"
	// UserAgent identifies the CLI to Steam's storefront API.
	UserAgent = "game-goat-pp-cli/0.1.0 (+printing-press)"
	// httpTimeout bounds every outbound Steam request.
	httpTimeout = 10 * time.Second
)

// Typed sentinel errors: callers degrade on these without string matching.
var (
	// ErrAppNotFound: the title did not resolve to a Steam app (empty
	// storesearch, or appdetails success:false for the appid).
	ErrAppNotFound = errors.New("steam: no Steam app matched the title")
	// ErrReviewsUnavailable: appreviews reported success != 1 — the app
	// exists but has no review summary (e.g. not yet released).
	ErrReviewsUnavailable = errors.New("steam: review summary unavailable for app")
)

// Client is the typed keyless Steam storefront client.
type Client struct {
	// BaseURL is the storefront host (storesearch/appdetails) and is
	// overridable so tests can point at an httptest server.
	BaseURL string
	// APIBaseURL is the keyless store-SERVICES host (IStoreQueryService,
	// IStoreBrowseService, IStoreService), also test-overridable.
	APIBaseURL string
	// Country is the ISO 3166-1 alpha-2 storefront region.
	Country string
	// Language is the store locale.
	Language string
	doer     *adaptiveDoer

	tagMu    sync.Mutex
	tagCache map[int]string
}

// New builds a Steam client from cfg (nil falls back to NewConfig). The
// HTTP client uses a 10s timeout and the CLI's descriptive User-Agent.
func New(cfg *Config) *Client {
	if cfg == nil {
		cfg = NewConfig()
	}
	return &Client{
		BaseURL:    DefaultBaseURL,
		APIBaseURL: DefaultAPIBaseURL,
		Country:    cfg.Country,
		Language:   cfg.Language,
		doer:       newAdaptiveDoer(&http.Client{Timeout: httpTimeout}, cfg.RateLimit, DefaultBurst),
	}
}

func (c *Client) headers() map[string]string {
	return map[string]string{
		"User-Agent": UserAgent,
		"Accept":     "application/json",
	}
}

// normalizeName folds case and whitespace so the storesearch match prefers
// exact titles over bundles and soundtracks sharing a prefix.
func normalizeName(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// ResolveAppID maps a game title to a Steam appid via the keyless
// storesearch endpoint. Bundles and soundtracks are skipped in favor of the
// first app-typed exact-title match. An empty result set is the typed
// ErrAppNotFound.
func (c *Client) ResolveAppID(ctx context.Context, title string) (int64, error) {
	// Region/locale come from the client so store results are localisable like
	// the IsThereAnyDeal path instead of pinned to the US/en storefront.
	endpoint := fmt.Sprintf("%s/api/storesearch/?term=%s&cc=%s&l=%s", c.BaseURL, url.QueryEscape(title), c.countryCode(), c.languageName())
	body, err := c.doer.get(ctx, endpoint, c.headers())
	if err != nil {
		return 0, err
	}
	var resp struct {
		Total int `json:"total"`
		Items []struct {
			Type string `json:"type"`
			Name string `json:"name"`
			ID   int64  `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, fmt.Errorf("steam: parsing storesearch response for %q: %w", title, err)
	}
	if len(resp.Items) == 0 {
		return 0, fmt.Errorf("%w: %q", ErrAppNotFound, title)
	}
	normalized := normalizeName(title)
	for _, item := range resp.Items {
		if item.Type == "app" && item.ID > 0 && normalizeName(item.Name) == normalized {
			return item.ID, nil
		}
	}
	// No exact store match: never guess. The first app can be a different
	// edition or a similarly named game; presenting its reviews and price
	// as the requested game's is worse than reporting no Steam data.
	return 0, fmt.Errorf("%w: %q (no exact store match among %d results)", ErrAppNotFound, title, len(resp.Items))
}

// ReviewSummary is the keyless appreviews rollup for one app. Score is
// the 0-10 review_score converted to a 0-100 percent scale.
type ReviewSummary struct {
	Score    float64 `json:"score"`
	Desc     string  `json:"desc"`
	Positive int     `json:"positive"`
	Negative int     `json:"negative"`
	Total    int     `json:"total"`
}

// ReviewSummary fetches the keyless review rollup for one appid. A
// success != 1 response is the typed ErrReviewsUnavailable.
func (c *Client) ReviewSummary(ctx context.Context, appid int64) (ReviewSummary, error) {
	endpoint := fmt.Sprintf("%s/appreviews/%d?json=1&num_per_page=0&language=all&purchase_type=all", c.BaseURL, appid)
	body, err := c.doer.get(ctx, endpoint, c.headers())
	if err != nil {
		return ReviewSummary{}, err
	}
	var resp struct {
		Success      int `json:"success"`
		QuerySummary struct {
			ReviewScoreDesc string `json:"review_score_desc"`
			TotalPositive   int    `json:"total_positive"`
			TotalNegative   int    `json:"total_negative"`
			TotalReviews    int    `json:"total_reviews"`
			ReviewScore     int    `json:"review_score"`
		} `json:"query_summary"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return ReviewSummary{}, fmt.Errorf("steam: parsing appreviews response for app %d: %w", appid, err)
	}
	if resp.Success != 1 {
		return ReviewSummary{}, fmt.Errorf("%w: app %d (appreviews success=%d)", ErrReviewsUnavailable, appid, resp.Success)
	}
	return ReviewSummary{
		Score:    float64(resp.QuerySummary.ReviewScore) * 10,
		Desc:     resp.QuerySummary.ReviewScoreDesc,
		Positive: resp.QuerySummary.TotalPositive,
		Negative: resp.QuerySummary.TotalNegative,
		Total:    resp.QuerySummary.TotalReviews,
	}, nil
}

// PriceOverview is the appdetails price block (amounts in minor units,
// e.g. US cents).
type PriceOverview struct {
	Currency        string `json:"currency"`
	Initial         int    `json:"initial"`
	Final           int    `json:"final"`
	DiscountPercent int    `json:"discount_percent"`
}

// AppDetails is the subset of appdetails data the ratings card needs.
type AppDetails struct {
	Name  string         `json:"name"`
	Type  string         `json:"type,omitempty"`
	Price *PriceOverview `json:"price_overview,omitempty"`
	// DemoAppIDs is the appids of the demos the appdetails "demos" array
	// names. Empty means the store reported none.
	DemoAppIDs []int64 `json:"demo_app_ids,omitempty"`
}

// AppDetails fetches the detail record for one appid. The top-level
// response is keyed by the appid string, so the decode goes through
// json.RawMessage to tolerate the dynamic key. success:false is the typed
// ErrAppNotFound.
func (c *Client) AppDetails(ctx context.Context, appid int64) (*AppDetails, error) {
	endpoint := fmt.Sprintf("%s/api/appdetails?appids=%d&cc=%s&l=%s", c.BaseURL, appid, c.countryCode(), c.languageName())
	body, err := c.doer.get(ctx, endpoint, c.headers())
	if err != nil {
		return nil, err
	}
	var keyed map[string]json.RawMessage
	if err := json.Unmarshal(body, &keyed); err != nil {
		return nil, fmt.Errorf("steam: parsing appdetails response for app %d: %w", appid, err)
	}
	key := strconv.FormatInt(appid, 10)
	raw, ok := keyed[key]
	if !ok {
		// Tolerate a single-entry map whose key drifted from our appid.
		for _, v := range keyed {
			raw, ok = v, true
			break
		}
	}
	if !ok {
		return nil, fmt.Errorf("%w: app %d (empty appdetails response)", ErrAppNotFound, appid)
	}
	var wrapper struct {
		Success bool `json:"success"`
		Data    struct {
			Name  string         `json:"name"`
			Type  string         `json:"type"`
			Price *PriceOverview `json:"price_overview"`
			Demos []struct {
				AppID int64 `json:"appid"`
			} `json:"demos"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil, fmt.Errorf("steam: parsing appdetails entry for app %d: %w", appid, err)
	}
	if !wrapper.Success {
		return nil, fmt.Errorf("%w: app %d (appdetails success=false)", ErrAppNotFound, appid)
	}
	details := &AppDetails{
		Name:  wrapper.Data.Name,
		Type:  wrapper.Data.Type,
		Price: wrapper.Data.Price,
	}
	for _, d := range wrapper.Data.Demos {
		if d.AppID != 0 {
			details.DemoAppIDs = append(details.DemoAppIDs, d.AppID)
		}
	}
	return details, nil
}
