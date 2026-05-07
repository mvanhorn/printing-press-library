// Package wikivoyage is a thin per-locale client for the Wikivoyage REST v1
// page summary endpoint. The on-the-wire shape mirrors Wikipedia's REST API.
package wikivoyage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// PageSummary is the flattened REST summary payload. PageURL pulls the nested
// content_urls.desktop.page string up to the top level.
type PageSummary struct {
	Title       string  `json:"title"`
	Extract     string  `json:"extract"`
	ExtractHTML string  `json:"extract_html"`
	PageURL     string  `json:"page_url"`
	Coordinates *Coords `json:"coordinates,omitempty"`
}

// Client is a per-locale Wikivoyage client.
type Client struct {
	Locale     string
	RESTBase   string
	HTTPClient *http.Client
	UserAgent  string
}

// New constructs a Client bound to the given locale (e.g. "en", "ja", "fr").
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
		RESTBase:   fmt.Sprintf("https://%s.wikivoyage.org/api/rest_v1", locale),
		HTTPClient: httpClient,
		UserAgent:  ua,
	}
}

func (c *Client) doGet(ctx context.Context, fullURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wikivoyage request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("wikivoyage read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned %d: %s", fullURL, resp.StatusCode, httperr.Snippet(body))
	}
	return body, nil
}

// summaryEnvelope mirrors the nested REST shape.
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
		return nil, fmt.Errorf("wikivoyage summary parse: %w", err)
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
