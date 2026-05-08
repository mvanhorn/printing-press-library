// Package retty is the Stage-2 source client for retty.me — Japan's
// review-aggregator alternative to Tabelog. Same structural shape as
// tabelog: HTML scrape with polite UA + per-call throttle, locale "ja".
//
// v1 had retty body extraction marked "deferred"; v2 promotes it: the
// search result page yields enough title + URL data for a useful Hit.
package retty

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
	defaultBase     = "https://retty.me"
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

func (c *Client) Slug() string   { return "retty" }
func (c *Client) Locale() string { return "ja" }
func (c *Client) IsStub() bool   { return true } // LookupByName is stubbed; sitemap discovery + JS-rendered search not yet implemented.

func (c *Client) throttle() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if since := time.Since(c.lastCall); since < c.interval {
		time.Sleep(c.interval - since)
	}
	c.lastCall = time.Now()
}

// Retty restaurant detail anchors look like /restaurants/<id>/ or
// /restaurant/<id>/. The card surrounding them carries the title in the
// anchor's inner text.
var rettyAnchor = regexp.MustCompile(`<a[^>]+href="(/restaurants?/[0-9a-zA-Z_-]+/?)"[^>]*>\s*([^<\s][^<]{1,120})\s*</a>`)

// LookupByName: Retty's user-facing search is JS-rendered (the homepage
// search box submits via React, no public REST search URL). Until we
// implement a Tabelog-style mapping or browser-sniff path, return
// ErrNotImplemented so the dispatcher records this as a stub rather
// than a hard error.
func (c *Client) LookupByName(ctx context.Context, name, city string, maxResults int) ([]sourcetypes.Hit, error) {
	_ = name
	_ = city
	_ = maxResults
	_ = url.QueryEscape // keep import; minimum cost.
	return nil, sourcetypes.ErrNotImplemented
}

// lookupByNameLive is the (currently dead) HTML-scrape path. Reachable
// once we add a working search URL discovery.
func (c *Client) lookupByNameLive(ctx context.Context, name, city string, maxResults int) ([]sourcetypes.Hit, error) {
	if maxResults <= 0 || maxResults > 20 {
		maxResults = 5
	}
	q := strings.TrimSpace(name)
	if city != "" {
		q = q + " " + strings.TrimSpace(city)
	}
	if q == "" {
		return nil, fmt.Errorf("retty.LookupByName: empty query")
	}
	u := fmt.Sprintf("%s/search/?keyword=%s", c.base, url.QueryEscape(q))
	body, err := c.fetch(ctx, u)
	if err != nil {
		return nil, err
	}
	return extractRettyHits(body, c.base, maxResults), nil
}

func extractRettyHits(body, base string, maxResults int) []sourcetypes.Hit {
	matches := rettyAnchor.FindAllStringSubmatch(body, -1)
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
			Source: "retty",
			URL:    full,
			Title:  title,
			Locale: "ja",
		})
	}
	return out
}

// CheckClosed reuses the Tabelog kanji check; "閉店" / "営業終了" are
// the same Japanese closed-keywords on Retty.
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
		v.Source = "retty"
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
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "ja,en;q=0.5")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("retty GET %s: %w", u, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("retty GET %s: read body: %w", u, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("retty GET %s: HTTP %d: %s", u, resp.StatusCode, httperr.Snippet(body))
	}
	return string(body), nil
}
