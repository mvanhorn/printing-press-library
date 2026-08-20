// Package hotpepper is the Stage-2 source client for hotpepper.jp,
// Recruit's gourmet review site (the public face of the Hot Pepper API).
//
// Two paths:
//
//  1. If HOTPEPPER_API_KEY is set, use Recruit's official API
//     (https://webservice.recruit.co.jp/hotpepper/gourmet/v1/) — free up
//     to 3000 req/day. Returns clean JSON.
//  2. Otherwise, fall back to HTML scrape of hotpepper.jp/strJ?keyword=
//     (no key required, but rate-limited and brittle).
//
// The dispatcher should prefer the API path when available; the doctor
// command surfaces missing-key as a soft warning (Hot Pepper is
// optional, not required like Google Places).
package hotpepper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/closedsignal"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/httperr"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/sourcetypes"
)

const (
	APIKeyEnv       = "HOTPEPPER_API_KEY"
	defaultBase     = "https://www.hotpepper.jp"
	defaultAPIBase  = "https://webservice.recruit.co.jp"
	defaultUA       = "wanderlust-goat-pp-cli/0.2 (+https://github.com/joeheitzeberg/wanderlust-goat)"
	defaultInterval = 1500 * time.Millisecond
)

type Client struct {
	apiKey     string
	base       string
	apiBase    string
	ua         string
	interval   time.Duration
	httpClient *http.Client
	mu         sync.Mutex
	lastCall   time.Time
}

func NewClient() *Client {
	return &Client{
		apiKey:     strings.TrimSpace(os.Getenv(APIKeyEnv)),
		base:       defaultBase,
		apiBase:    defaultAPIBase,
		ua:         defaultUA,
		interval:   defaultInterval,
		httpClient: &http.Client{Timeout: 12 * time.Second},
	}
}

// NewClientWithBase is for tests.
func NewClientWithBase(base, apiBase, apiKey string, interval time.Duration) *Client {
	return &Client{
		apiKey:     apiKey,
		base:       strings.TrimRight(base, "/"),
		apiBase:    strings.TrimRight(apiBase, "/"),
		ua:         defaultUA,
		interval:   interval,
		httpClient: &http.Client{Timeout: 8 * time.Second},
	}
}

func (c *Client) Slug() string    { return "hotpepper" }
func (c *Client) Locale() string  { return "ja" }
func (c *Client) IsStub() bool    { return false }
func (c *Client) HasAPIKey() bool { return c.apiKey != "" }

func (c *Client) throttle() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if since := time.Since(c.lastCall); since < c.interval {
		time.Sleep(c.interval - since)
	}
	c.lastCall = time.Now()
}

// API path: GET /hotpepper/gourmet/v1/?key=<k>&keyword=<q>&format=json
type apiResponse struct {
	Results struct {
		Shop []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			URLs struct {
				PC string `json:"pc"`
			} `json:"urls"`
			Catch   string `json:"catch"`
			Address string `json:"address"`
			Open    string `json:"open"`
			Close   string `json:"close"`
		} `json:"shop"`
	} `json:"results"`
}

func (c *Client) lookupAPI(ctx context.Context, name, city string, maxResults int) ([]sourcetypes.Hit, error) {
	q := strings.TrimSpace(name)
	if city != "" {
		q = q + " " + strings.TrimSpace(city)
	}
	u := fmt.Sprintf("%s/hotpepper/gourmet/v1/?key=%s&keyword=%s&format=json&count=%d",
		c.apiBase, url.QueryEscape(c.apiKey), url.QueryEscape(q), maxResults)
	body, err := c.fetch(ctx, u)
	if err != nil {
		return nil, err
	}
	var resp apiResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil, fmt.Errorf("hotpepper API parse: %w", err)
	}
	out := make([]sourcetypes.Hit, 0, len(resp.Results.Shop))
	for _, s := range resp.Results.Shop {
		out = append(out, sourcetypes.Hit{
			Source:  "hotpepper",
			URL:     s.URLs.PC,
			Title:   s.Name,
			Snippet: trim200(s.Catch),
			Locale:  "ja",
		})
	}
	return out, nil
}

// HTML path: hotpepper search result extraction (used when no API key).
var htmlAnchor = regexp.MustCompile(`<a[^>]+href="(/str[A-Z]{0,2}[0-9]+/?)"[^>]*>\s*([^<\s][^<]{1,120})\s*</a>`)

func (c *Client) lookupHTML(ctx context.Context, name, city string, maxResults int) ([]sourcetypes.Hit, error) {
	q := strings.TrimSpace(name)
	if city != "" {
		q = q + " " + strings.TrimSpace(city)
	}
	// HotPepper Gourmet's real freeword search lives at the root with the
	// `freeword` and `Submit=Search` query params; /strJ/?keyword= is a
	// 404. The result page exposes /strJ<digits>/ shop links that the
	// existing htmlAnchor regex picks up.
	u := fmt.Sprintf("%s/?freeword=%s&Submit=Search", c.base, url.QueryEscape(q))
	body, err := c.fetch(ctx, u)
	if err != nil {
		return nil, err
	}
	return extractHotpepperHTMLHits(body, c.base, maxResults), nil
}

func extractHotpepperHTMLHits(body, base string, maxResults int) []sourcetypes.Hit {
	matches := htmlAnchor.FindAllStringSubmatch(body, -1)
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
			Source: "hotpepper",
			URL:    full,
			Title:  title,
			Locale: "ja",
		})
	}
	return out
}

// LookupByName picks API path when a key is set, falls back to HTML.
func (c *Client) LookupByName(ctx context.Context, name, city string, maxResults int) ([]sourcetypes.Hit, error) {
	if maxResults <= 0 || maxResults > 20 {
		maxResults = 5
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("hotpepper.LookupByName: empty query")
	}
	if c.apiKey != "" {
		return c.lookupAPI(ctx, name, city, maxResults)
	}
	return c.lookupHTML(ctx, name, city, maxResults)
}

// CheckClosed looks for the same JP closed kanji on the detail page.
func (c *Client) CheckClosed(ctx context.Context, hit sourcetypes.Hit) closedsignal.Verdict {
	if hit.URL == "" {
		return closedsignal.Open
	}
	body, err := c.fetch(ctx, hit.URL)
	if err != nil {
		return closedsignal.Open
	}
	v := closedsignal.CheckTabelogHTML(body)
	if v.Closed || v.Temporary {
		v.Source = "hotpepper"
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
	req.Header.Set("Accept-Language", "ja,en;q=0.5")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("hotpepper GET %s: %w", u, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("hotpepper GET %s: read body: %w", u, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("hotpepper GET %s: HTTP %d: %s", u, resp.StatusCode, httperr.Snippet(body))
	}
	return string(body), nil
}

func trim200(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		runes := []rune(s)
		if len(runes) > 200 {
			return string(runes[:200]) + "..."
		}
	}
	return s
}
