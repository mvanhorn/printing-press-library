// Package tabelog is the Stage-2 source client for tabelog.com (Japan's
// dominant restaurant review aggregator). Anti-bot mitigation: polite
// browser UA + 1.5s minimum interval between requests.
//
// LookupByName uses Tabelog's keyword search URL; HTML parsing extracts
// listing anchors + titles. CheckClosed reads the listing detail page
// and runs closedsignal.CheckTabelogHTML.
package tabelog

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
	defaultBase     = "https://tabelog.com"
	defaultUA       = "wanderlust-goat-pp-cli/0.2 (+https://github.com/joeheitzeberg/wanderlust-goat)"
	defaultInterval = 1500 * time.Millisecond
)

// Client is the Tabelog client.
type Client struct {
	base       string
	ua         string
	interval   time.Duration
	httpClient *http.Client

	mu       sync.Mutex
	lastCall time.Time
}

// NewClient returns a default Tabelog client.
func NewClient() *Client {
	return &Client{
		base:       defaultBase,
		ua:         defaultUA,
		interval:   defaultInterval,
		httpClient: &http.Client{Timeout: 12 * time.Second},
	}
}

// NewClientWithBase is for tests. interval is also test-configurable.
func NewClientWithBase(base string, interval time.Duration) *Client {
	return &Client{
		base:       strings.TrimRight(base, "/"),
		ua:         defaultUA,
		interval:   interval,
		httpClient: &http.Client{Timeout: 8 * time.Second},
	}
}

// Slug implements sourcetypes.Client.
func (c *Client) Slug() string { return "tabelog" }

// Locale implements sourcetypes.Client.
func (c *Client) Locale() string { return "ja" }

// IsStub implements sourcetypes.Client.
func (c *Client) IsStub() bool { return false }

// throttle sleeps if needed to respect the per-instance interval. Cheap;
// no goroutines needed because per-source fanout already runs each
// source in its own goroutine.
func (c *Client) throttle() {
	c.mu.Lock()
	defer c.mu.Unlock()
	since := time.Since(c.lastCall)
	if since < c.interval {
		time.Sleep(c.interval - since)
	}
	c.lastCall = time.Now()
}

// listingAnchor matches Tabelog listing card links: <a href="/<area>/A.../<rstId>/" class="list-rst__rst-name-target">Name</a>
var listingAnchor = regexp.MustCompile(`<a[^>]+href="(/[^"\s]+/[A-Z][0-9]+/[A-Z][0-9]+/[0-9]+/?)"[^>]*class="[^"]*list-rst__rst-name-target[^"]*"[^>]*>([^<]+)</a>`)

// fallbackTitleAnchor matches restaurant detail anchors when listing-card
// markup isn't present (search-result fallback).
var fallbackTitleAnchor = regexp.MustCompile(`<a[^>]+href="(/[^"\s]+/[0-9]+/?)"[^>]*>\s*([^\s<][^<]{1,80})\s*</a>`)

// LookupByName implements sourcetypes.Client.
func (c *Client) LookupByName(ctx context.Context, name, city string, maxResults int) ([]sourcetypes.Hit, error) {
	if maxResults <= 0 || maxResults > 20 {
		maxResults = 5
	}
	q := strings.TrimSpace(name)
	if city != "" {
		q = q + " " + strings.TrimSpace(city)
	}
	if q == "" {
		return nil, fmt.Errorf("tabelog.LookupByName: empty query")
	}
	// Use the English-language listing path: the Japanese path returns the
	// same restaurants but with markup our regex parser doesn't match.
	u := fmt.Sprintf("%s/en/rstLst/?sw=%s", c.base, url.QueryEscape(q))

	body, err := c.fetch(ctx, u)
	if err != nil {
		return nil, err
	}
	hits := extractTabelogHits(body, c.base, maxResults)
	return hits, nil
}

func extractTabelogHits(body, base string, maxResults int) []sourcetypes.Hit {
	var hits []sourcetypes.Hit
	matches := listingAnchor.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		matches = fallbackTitleAnchor.FindAllStringSubmatch(body, -1)
	}
	seen := map[string]bool{}
	for _, m := range matches {
		if len(hits) >= maxResults {
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
		hits = append(hits, sourcetypes.Hit{
			Source: "tabelog",
			URL:    full,
			Title:  title,
			Locale: "ja",
		})
	}
	return hits
}

// CheckClosed fetches the listing detail page and runs the Tabelog-specific
// closed-keyword detector. Errors fetching are returned as Open verdicts
// (no signal, not a closure).
func (c *Client) CheckClosed(ctx context.Context, hit sourcetypes.Hit) closedsignal.Verdict {
	if hit.URL == "" {
		return closedsignal.Open
	}
	body, err := c.fetch(ctx, hit.URL)
	if err != nil {
		return closedsignal.Open
	}
	return closedsignal.CheckTabelogHTML(body)
}

func (c *Client) fetch(ctx context.Context, u string) (string, error) {
	c.throttle()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", c.ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "ja,en;q=0.5")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("tabelog GET %s: %w", u, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("tabelog GET %s: read body: %w", u, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("tabelog GET %s: HTTP %d: %s", u, resp.StatusCode, httperr.Snippet(body))
	}
	return string(body), nil
}
