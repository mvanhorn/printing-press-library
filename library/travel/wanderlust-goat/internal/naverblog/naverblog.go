// Package naverblog is the Stage-2 source client for blog.naver.com — the
// long-tail Korean review-blog network. Search returns blog post URLs;
// each post tends to be one specific venue review.
package naverblog

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
	defaultBase     = "https://search.naver.com"
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

func (c *Client) Slug() string   { return "naverblog" }
func (c *Client) Locale() string { return "ko" }
func (c *Client) IsStub() bool   { return false }

func (c *Client) throttle() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if since := time.Since(c.lastCall); since < c.interval {
		time.Sleep(c.interval - since)
	}
	c.lastCall = time.Now()
}

// Naver Blog post URLs: blog.naver.com/<id>/<postNo>.
var blogAnchor = regexp.MustCompile(`<a[^>]+href="(https?://blog\.naver\.com/[A-Za-z0-9_]+/[0-9]+)"[^>]*>\s*([^<\s][^<]{1,160})\s*</a>`)

func (c *Client) LookupByName(ctx context.Context, name, city string, maxResults int) ([]sourcetypes.Hit, error) {
	if maxResults <= 0 || maxResults > 20 {
		maxResults = 5
	}
	q := strings.TrimSpace(name)
	if city != "" {
		q = q + " " + strings.TrimSpace(city)
	}
	if q == "" {
		return nil, fmt.Errorf("naverblog.LookupByName: empty query")
	}
	u := fmt.Sprintf("%s/search.naver?where=blog&query=%s", c.base, url.QueryEscape(q))
	body, err := c.fetch(ctx, u)
	if err != nil {
		return nil, err
	}
	return extractBlogHits(body, maxResults), nil
}

func extractBlogHits(body string, maxResults int) []sourcetypes.Hit {
	matches := blogAnchor.FindAllStringSubmatch(body, -1)
	seen := map[string]bool{}
	var out []sourcetypes.Hit
	for _, m := range matches {
		if len(out) >= maxResults {
			break
		}
		full := m[1]
		title := strings.TrimSpace(m[2])
		if title == "" || seen[full] {
			continue
		}
		seen[full] = true
		out = append(out, sourcetypes.Hit{
			Source: "naverblog",
			URL:    full,
			Title:  title,
			Locale: "ko",
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
	// Blogs frequently mention 폐업 about visited venues; defer to the
	// generic Naver detector.
	v := closedsignal.CheckNaverHTML(body)
	if v.Closed || v.Temporary {
		v.Source = "naverblog"
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
	req.Header.Set("Accept-Language", "ko,en;q=0.5")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("naverblog GET %s: %w", u, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("naverblog GET %s: read body: %w", u, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("naverblog GET %s: HTTP %d: %s", u, resp.StatusCode, httperr.Snippet(body))
	}
	return string(body), nil
}
