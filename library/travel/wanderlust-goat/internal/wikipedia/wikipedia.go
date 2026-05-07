// Package wikipedia is a thin per-locale client for the Wikipedia MediaWiki
// action API (geosearch) and the REST v1 API (page summary).
package wikipedia

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/httperr"
)

const (
	defaultUserAgent = "wanderlust-goat-pp-cli/0.1 (+https://github.com/mvanhorn/printing-press-library)"
	defaultTimeout   = 10 * time.Second
)

// Coords is a flat lat/lon pair as returned by the REST page summary endpoint.
type Coords struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// GeoSearchPage is a single page in a geosearch response.
type GeoSearchPage struct {
	PageID   int     `json:"pageid"`
	Title    string  `json:"title"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Distance float64 `json:"dist"`
}

// GeoSearchResponse is the flattened list of pages returned to callers.
type GeoSearchResponse struct {
	Pages []GeoSearchPage `json:"pages"`
}

// PageSummary is the flattened REST summary payload. PageURL pulls the nested
// content_urls.desktop.page string up to the top level.
type PageSummary struct {
	Title       string  `json:"title"`
	Extract     string  `json:"extract"`
	ExtractHTML string  `json:"extract_html"`
	PageURL     string  `json:"page_url"`
	Coordinates *Coords `json:"coordinates,omitempty"`
}

// Client is a per-locale Wikipedia client. ActionBase serves the MediaWiki
// action API; RESTBase serves the REST v1 endpoints.
type Client struct {
	Locale             string
	ActionBase         string
	RESTBase           string
	HTTPClient         *http.Client
	UserAgent          string
	RateLimitPerSecond float64

	mu       sync.Mutex
	lastSent time.Time
}

// New constructs a Client bound to the given locale (e.g. "en", "ja", "ko").
// Empty locale defaults to "en".
func New(locale string, httpClient *http.Client, ua string) *Client {
	if locale == "" {
		locale = "en"
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	if ua == "" {
		ua = defaultUserAgent
	}
	return &Client{
		Locale:     locale,
		ActionBase: fmt.Sprintf("https://%s.wikipedia.org/w/api.php", locale),
		RESTBase:   fmt.Sprintf("https://%s.wikipedia.org/api/rest_v1", locale),
		HTTPClient: httpClient,
		UserAgent:  ua,
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

func (c *Client) doGet(ctx context.Context, fullURL string) ([]byte, error) {
	if err := c.throttle(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wikipedia request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("wikipedia read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned %d: %s", fullURL, resp.StatusCode, httperr.Snippet(body))
	}
	return body, nil
}

// geosearchEnvelope mirrors the action-API JSON shape so we can flatten it.
type geosearchEnvelope struct {
	Query struct {
		GeoSearch []GeoSearchPage `json:"geosearch"`
	} `json:"query"`
}

// GeoSearch finds pages within radiusMeters of (lat, lng), capped at limit.
func (c *Client) GeoSearch(ctx context.Context, lat, lng float64, radiusMeters int, limit int) (*GeoSearchResponse, error) {
	if limit <= 0 {
		limit = 10
	}
	q := url.Values{}
	q.Set("action", "query")
	q.Set("list", "geosearch")
	q.Set("gscoord", fmt.Sprintf("%f|%f", lat, lng))
	q.Set("gsradius", strconv.Itoa(radiusMeters))
	q.Set("gslimit", strconv.Itoa(limit))
	q.Set("format", "json")
	q.Set("formatversion", "2")
	full := c.ActionBase + "?" + q.Encode()

	body, err := c.doGet(ctx, full)
	if err != nil {
		return nil, err
	}
	var env geosearchEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("wikipedia geosearch parse: %w", err)
	}
	return &GeoSearchResponse{Pages: env.Query.GeoSearch}, nil
}

// summaryEnvelope mirrors the nested REST shape; we flatten content_urls.desktop.page.
type summaryEnvelope struct {
	Title       string  `json:"title"`
	Extract     string  `json:"extract"`
	ExtractHTML string  `json:"extract_html"`
	Coordinates *Coords `json:"coordinates,omitempty"`
	ContentURLs *struct {
		Desktop *struct {
			Page string `json:"page"`
		} `json:"desktop"`
	} `json:"content_urls,omitempty"`
}

// PageSummary fetches the REST summary for a page title and flattens
// content_urls.desktop.page into PageURL.
func (c *Client) PageSummary(ctx context.Context, title string) (*PageSummary, error) {
	full := c.RESTBase + "/page/summary/" + url.PathEscape(title)
	body, err := c.doGet(ctx, full)
	if err != nil {
		return nil, err
	}
	var env summaryEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("wikipedia summary parse: %w", err)
	}
	out := &PageSummary{
		Title:       env.Title,
		Extract:     env.Extract,
		ExtractHTML: env.ExtractHTML,
		Coordinates: env.Coordinates,
	}
	if env.ContentURLs != nil && env.ContentURLs.Desktop != nil {
		out.PageURL = env.ContentURLs.Desktop.Page
	}
	return out, nil
}
