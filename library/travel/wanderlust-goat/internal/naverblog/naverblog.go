// Package naverblog scrapes Naver Blog search results
// (https://search.naver.com/search.naver?where=blog&query=...). The page is
// server-rendered HTML and Naver gates anonymous traffic to browser-shaped UAs.
package naverblog

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://search.naver.com/search.naver"
	// Naver returns a Korean-only error page for spider-shaped UAs, so we ship a
	// browser-like default. Callers can still override via the constructor.
	defaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 wanderlust-goat-pp-cli/0.1 (+https://github.com/mvanhorn/printing-press-library)"
	defaultTimeout   = 10 * time.Second
	maxResults       = 30
)

// Post is a flattened blog card from a Naver search result.
type Post struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Author  string `json:"author"`
	Snippet string `json:"snippet"`
	Posted  string `json:"posted"`
}

// Client is a no-auth Naver Blog scraper.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	UserAgent  string
}

// New constructs a Client. The default UA is browser-shaped because Naver
// returns an error page to obvious bot UAs.
func New(httpClient *http.Client, ua string) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	if ua == "" {
		ua = defaultUserAgent
	}
	return &Client{
		BaseURL:    defaultBaseURL,
		HTTPClient: httpClient,
		UserAgent:  ua,
	}
}

// Search runs a blog-only search and returns up to ~30 cards.
func (c *Client) Search(ctx context.Context, query string) ([]Post, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("naverblog search: empty query")
	}
	v := url.Values{}
	v.Set("where", "blog")
	v.Set("query", query)
	full := c.BaseURL + "?" + v.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "ko,en;q=0.8")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("naverblog request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("naverblog read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned %d", full, resp.StatusCode)
	}
	return parseSearch(string(body)), nil
}

// Naver's blog search HTML wraps each card in a <li class="bx" ...> with stable
// inner classes (.title_link, .user_info, .dsc_txt, .sub_time). Class names get
// renamed every few quarters; we match liberally on substrings to absorb churn.
var (
	cardRe = regexp.MustCompile(`(?is)<li[^>]+class=["'][^"']*\bbx\b[^"']*["'][^>]*>([\s\S]+?)</li>`)

	// Title link: any <a class="…title_link…" href="…">…</a>.
	titleLinkRe = regexp.MustCompile(`(?is)<a[^>]+class=["'][^"']*title_link[^"']*["'][^>]+href=["']([^"']+)["'][^>]*>([\s\S]+?)</a>`)

	// Author: <a class="…name…">name</a> or <a class="user_info"...>name</a>.
	authorRe = regexp.MustCompile(`(?is)<a[^>]+class=["'][^"']*(?:name|user_info)[^"']*["'][^>]*>([\s\S]+?)</a>`)

	// Snippet/description: <a|div class="…dsc_txt…|api_txt_lines…">text</a|div>.
	snippetRe = regexp.MustCompile(`(?is)<(?:a|div)[^>]+class=["'][^"']*(?:dsc_txt|api_txt_lines)[^"']*["'][^>]*>([\s\S]+?)</(?:a|div)>`)

	// Date: <span class="…sub_time…|…sub_date…">2024.01.15.</span>.
	dateRe = regexp.MustCompile(`(?is)<span[^>]+class=["'][^"']*(?:sub_time|sub_date)[^"']*["'][^>]*>([\s\S]+?)</span>`)

	tagStripRe   = regexp.MustCompile(`<[^>]+>`)
	whitespaceRe = regexp.MustCompile(`\s+`)
)

func parseSearch(html string) []Post {
	out := make([]Post, 0, 16)
	for _, m := range cardRe.FindAllStringSubmatch(html, -1) {
		card := m[1]
		var p Post

		if tm := titleLinkRe.FindStringSubmatch(card); len(tm) == 3 {
			p.URL = strings.TrimSpace(tm[1])
			p.Title = stripAndCollapse(tm[2])
		}
		if p.Title == "" || p.URL == "" {
			continue
		}
		if am := authorRe.FindStringSubmatch(card); len(am) == 2 {
			p.Author = stripAndCollapse(am[1])
		}
		if sm := snippetRe.FindStringSubmatch(card); len(sm) == 2 {
			p.Snippet = stripAndCollapse(sm[1])
		}
		if dm := dateRe.FindStringSubmatch(card); len(dm) == 2 {
			p.Posted = stripAndCollapse(dm[1])
		}
		out = append(out, p)
		if len(out) >= maxResults {
			break
		}
	}
	return out
}

func stripAndCollapse(s string) string {
	s = tagStripRe.ReplaceAllString(s, " ")
	s = whitespaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
