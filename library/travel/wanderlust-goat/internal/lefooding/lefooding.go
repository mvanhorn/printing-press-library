// Package lefooding is the Stage-2 source client for lefooding.com — the
// Le Fooding editorial guide. Search returns curated review-page anchors;
// Le Fooding tags permanently-closed listings with "Fermé définitivement"
// in the page body, which closedsignal detects.
package lefooding

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/closedsignal"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/httperr"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/sourcetypes"
)

const (
	defaultBase     = "https://www.lefooding.com"
	defaultUA       = "wanderlust-goat-pp-cli/0.2 (+https://github.com/joeheitzeberg/wanderlust-goat)"
	defaultInterval = 1500 * time.Millisecond
)

type Client struct {
	base       string
	ua         string
	interval   time.Duration
	httpClient *http.Client
	mu         sync.Mutex
	lastCall   time.Time
}

func NewClient() *Client {
	return &Client{
		base:       defaultBase,
		ua:         defaultUA,
		interval:   defaultInterval,
		httpClient: &http.Client{Timeout: 12 * time.Second},
	}
}

func NewClientWithBase(base string, interval time.Duration) *Client {
	return &Client{
		base:       strings.TrimRight(base, "/"),
		ua:         defaultUA,
		interval:   interval,
		httpClient: &http.Client{Timeout: 8 * time.Second},
	}
}

func (c *Client) Slug() string   { return "lefooding" }
func (c *Client) Locale() string { return "fr" }
func (c *Client) IsStub() bool   { return false }

func (c *Client) throttle() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if since := time.Since(c.lastCall); since < c.interval {
		time.Sleep(c.interval - since)
	}
	c.lastCall = time.Now()
}

// Le Fooding listing URLs are typically /<lang>/<slug> e.g. /en/paris/restaurant-foo.
var listingAnchor = regexp.MustCompile(`<a[^>]+href="(/(?:en|fr)/[^"\s]+)"[^>]*>\s*([^<\s][^<]{1,160})\s*</a>`)

func (c *Client) LookupByName(ctx context.Context, name, city string, maxResults int) ([]sourcetypes.Hit, error) {
	if maxResults <= 0 || maxResults > 20 {
		maxResults = 5
	}
	q := strings.TrimSpace(name)
	if city != "" {
		q = q + " " + strings.TrimSpace(city)
	}
	if q == "" {
		return nil, fmt.Errorf("lefooding.LookupByName: empty query")
	}
	u := fmt.Sprintf("%s/search?q=%s", c.base, url.QueryEscape(q))
	body, err := c.fetch(ctx, u)
	if err != nil {
		return nil, err
	}
	return extractLeFoodingHits(body, c.base, maxResults), nil
}

func extractLeFoodingHits(body, base string, maxResults int) []sourcetypes.Hit {
	matches := listingAnchor.FindAllStringSubmatch(body, -1)
	seen := map[string]bool{}
	var out []sourcetypes.Hit
	for _, m := range matches {
		if len(out) >= maxResults {
			break
		}
		href := m[1]
		title := strings.TrimSpace(m[2])
		if title == "" {
			continue
		}
		full := href
		if strings.HasPrefix(href, "/") {
			full = base + href
		}
		if seen[full] {
			continue
		}
		seen[full] = true
		out = append(out, sourcetypes.Hit{
			Source: "lefooding",
			URL:    full,
			Title:  title,
			Locale: "fr",
		})
	}
	return out
}

func (c *Client) CheckClosed(ctx context.Context, hit sourcetypes.Hit) closedsignal.Verdict {
	if hit.URL == "" {
		return closedsignal.Open
	}
	body, err := c.fetch(ctx, hit.URL)
	if err != nil {
		return closedsignal.Open
	}
	v := closedsignal.CheckLeFoodingHTML(body)
	if v.Closed || v.Temporary {
		v.Source = "lefooding"
	}
	return v
}

func (c *Client) fetch(ctx context.Context, u string) (string, error) {
	c.throttle()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", c.ua)
	req.Header.Set("Accept-Language", "fr,en;q=0.5")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("lefooding GET %s: %w", u, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("lefooding GET %s: read body: %w", u, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("lefooding GET %s: HTTP %d: %s", u, resp.StatusCode, httperr.Snippet(body))
	}
	return string(body), nil
}
