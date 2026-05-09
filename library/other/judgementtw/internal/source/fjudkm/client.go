// Copyright 2026 wayne-lai. Licensed under Apache-2.0. See LICENSE.

// Package fjudkm is an HTTP/HTML client for fjudkm.judicial.gov.tw, the
// Judicial Knowledge Base (司法智識庫). It exposes the 462-topic browse,
// per-topic case-commentary list, single-commentary fetch, and full-text
// search.
package fjudkm

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"judgementtw-pp-cli/internal/cliutil"
	"judgementtw-pp-cli/internal/extract"
)

const BaseURL = "https://fjudkm.judicial.gov.tw"

const UserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

type Client struct {
	httpClient *http.Client
	limiter    *cliutil.AdaptiveLimiter
}

func New(rate float64) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Jar:     jar,
		},
		limiter: cliutil.NewAdaptiveLimiter(rate),
	}
}

func (c *Client) SetHTTPClient(h *http.Client) {
	if h.Jar == nil {
		h.Jar = c.httpClient.Jar
	}
	c.httpClient = h
}

func (c *Client) fetch(ctx context.Context, method, urlStr string, body io.Reader, contentType string, referer string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.limiter.Wait()
	req, err := http.NewRequestWithContext(ctx, method, urlStr, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-TW,zh;q=0.9,en;q=0.7")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP %s %s: %w", method, urlStr, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		c.limiter.OnRateLimit()
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, &cliutil.RateLimitError{
			URL:        urlStr,
			RetryAfter: cliutil.RetryAfter(resp),
			Body:       string(bodyBytes),
		}
	}
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("HTTP %s %s: %d %s — %s", method, urlStr, resp.StatusCode, resp.Status, string(bodyBytes))
	}
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}
	c.limiter.OnSuccess()
	return out, nil
}

// Topic is a single FJUDKM topic-tree entry.
type Topic struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

// TopicDetail is a topic page with its case-commentary list.
type TopicDetail struct {
	ID    int      `json:"id"`
	Title string   `json:"title"`
	Docs  []DocRef `json:"docs"`
}

// DocRef is a single case-commentary reference inside a topic.
type DocRef struct {
	Par     string `json:"par"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet,omitempty"`
}

// Doc is a single case-commentary detail.
type Doc struct {
	Par       string `json:"par"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	SourceURL string `json:"source_url"`
}

var (
	topicLinkPat       = regexp.MustCompile(`<a[^>]+href="index_title\.aspx\?id=(\d+)"[^>]*>([^<]+)</a>`)
	docLinkPat         = regexp.MustCompile(`<a[^>]+href="index_doc\.aspx\?par=([^"&]+)"[^>]*>([^<]*)</a>`)
	hiddenInputPattern = regexp.MustCompile(`(?is)<input[^>]+type="hidden"[^>]+name="([^"]+)"[^>]+value="([^"]*)"`)
)

// Topics fetches the list of all 462 topic categories from the home page.
func (c *Client) Topics(ctx context.Context) ([]Topic, error) {
	body, err := c.fetch(ctx, http.MethodGet, BaseURL+"/Default.aspx", nil, "", "")
	if err != nil {
		return nil, err
	}
	matches := topicLinkPat.FindAllStringSubmatch(string(body), -1)
	seen := make(map[int]struct{})
	var out []Topic
	for _, m := range matches {
		id, _ := strconv.Atoi(m[1])
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, Topic{
			ID:    id,
			Title: extract.CleanHTML(m[2]),
			URL:   BaseURL + "/index_title.aspx?id=" + m[1],
		})
	}
	return out, nil
}

// Topic fetches a single topic page with its case-commentary list.
func (c *Client) Topic(ctx context.Context, id int) (*TopicDetail, error) {
	urlStr := fmt.Sprintf("%s/index_title.aspx?id=%d", BaseURL, id)
	body, err := c.fetch(ctx, http.MethodGet, urlStr, nil, "", "")
	if err != nil {
		return nil, err
	}
	respStr := string(body)
	matches := docLinkPat.FindAllStringSubmatch(respStr, -1)
	seen := make(map[string]struct{})
	var docs []DocRef
	for _, m := range matches {
		par := m[1]
		if _, dup := seen[par]; dup {
			continue
		}
		seen[par] = struct{}{}
		docs = append(docs, DocRef{
			Par:   par,
			Title: extract.CleanHTML(m[2]),
			URL:   BaseURL + "/index_doc.aspx?par=" + par,
		})
	}
	// Page <title> typically holds the topic name.
	title := ""
	if t := regexp.MustCompile(`<title>([^<]+)</title>`).FindStringSubmatch(respStr); t != nil {
		title = strings.TrimSpace(t[1])
	}
	return &TopicDetail{
		ID:    id,
		Title: title,
		Docs:  docs,
	}, nil
}

// Get fetches a single case-commentary detail by its par-token.
func (c *Client) Get(ctx context.Context, par string) (*Doc, error) {
	if par == "" {
		return nil, fmt.Errorf("empty par token")
	}
	urlStr := BaseURL + "/index_doc.aspx?par=" + par
	body, err := c.fetch(ctx, http.MethodGet, urlStr, nil, "", "")
	if err != nil {
		return nil, err
	}
	respStr := string(body)
	title := ""
	if t := regexp.MustCompile(`<title>([^<]+)</title>`).FindStringSubmatch(respStr); t != nil {
		title = strings.TrimSpace(t[1])
	}
	// FJUDKM detail pages put the full commentary inside the main <form> body.
	// For robustness, just clean the entire HTML and trust readers to scan.
	bodyText := extract.CleanHTML(respStr)
	return &Doc{
		Par:       par,
		Title:     title,
		Body:      bodyText,
		SourceURL: urlStr,
	}, nil
}

// SearchParams describes one FJUDKM full-text search.
type SearchParams struct {
	Query    string
	Court    string
	CaseChar string
	No       int
	Limit    int
}

// Search runs a FJUDKM full-text search. Returns referenced docs with snippets.
//
// The form's POST handler responds with a 302 redirect to
// /searchList.aspx?par=<token> (a server-side query token). Go's http.Client
// does not auto-follow 302 redirects after a POST, so we drive the round-trip
// manually: POST → read Location header → GET the result page → parse links.
func (c *Client) Search(ctx context.Context, p SearchParams) ([]DocRef, error) {
	formURL := BaseURL + "/searcher.aspx"
	body, err := c.fetch(ctx, http.MethodGet, formURL, nil, "", "")
	if err != nil {
		return nil, fmt.Errorf("fetching search form: %w", err)
	}
	hidden := extractHiddenFields(string(body))
	form := url.Values{}
	form.Set("__VIEWSTATE", hidden["__VIEWSTATE"])
	form.Set("__VIEWSTATEGENERATOR", hidden["__VIEWSTATEGENERATOR"])
	if v := hidden["__EVENTVALIDATION"]; v != "" {
		form.Set("__EVENTVALIDATION", v)
	}
	// Both fields carry the search term: txtSearchText is the visible input,
	// hfSW is the server-side hidden that the JS submit handler copies into.
	// The minimal field set proven to work in browser-replay is below; adding
	// empty txtyear/txtcase/txtno/ddlcourt or empty lc* fields causes the
	// server to redirect to /Error.htm.
	form.Set("txtSearchText", p.Query)
	form.Set("hfSW", p.Query)
	if p.CaseChar != "" {
		form.Set("txtcase", p.CaseChar)
	}
	if p.No > 0 {
		form.Set("txtno", strconv.Itoa(p.No))
	}
	if p.Court != "" {
		form.Set("ddlcourt", p.Court)
	}
	// Default search scope: 專題名稱+關鍵詞 (chkbxSearch$0). The page also
	// pre-checks chkbxSearch$1 (精選裁判) on load via JS — replay both for
	// parity with the user-facing default.
	form.Set("chkbxSearch$0", "on")
	form.Set("chkbxSearch$1", "on")
	form.Set("btnSearch", "查詢")

	listURL, err := c.postSearchAndFollow(ctx, formURL, form)
	if err != nil {
		return nil, fmt.Errorf("posting search: %w", err)
	}
	respBody, err := c.fetch(ctx, http.MethodGet, listURL, nil, "", formURL)
	if err != nil {
		return nil, fmt.Errorf("fetching search results: %w", err)
	}
	respStr := string(respBody)
	matches := docLinkPat.FindAllStringSubmatch(respStr, -1)
	seen := make(map[string]struct{})
	var out []DocRef
	for _, m := range matches {
		par := m[1]
		if _, dup := seen[par]; dup {
			continue
		}
		seen[par] = struct{}{}
		out = append(out, DocRef{
			Par:   par,
			Title: extract.CleanHTML(m[2]),
			URL:   BaseURL + "/index_doc.aspx?par=" + par,
		})
	}
	limit := p.Limit
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func extractHiddenFields(html string) map[string]string {
	out := make(map[string]string)
	for _, m := range hiddenInputPattern.FindAllStringSubmatch(html, -1) {
		out[m[1]] = m[2]
	}
	return out
}

// postSearchAndFollow POSTs the search form and returns the absolute URL the
// 302 redirect points to. The default http.Client returns the 302 body for a
// POST without auto-following; we need the Location header.
func (c *Client) postSearchAndFollow(ctx context.Context, formURL string, form url.Values) (string, error) {
	c.limiter.Wait()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, formURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", formURL)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-TW,zh;q=0.9,en;q=0.7")

	// Tell the client to NOT follow this redirect — we want the Location header.
	prev := c.httpClient.CheckRedirect
	c.httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	defer func() { c.httpClient.CheckRedirect = prev }()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusMovedPermanently &&
		resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusTemporaryRedirect {
		return "", fmt.Errorf("expected redirect after search POST, got HTTP %d", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if loc == "" {
		return "", fmt.Errorf("redirect response missing Location header")
	}
	if strings.HasPrefix(loc, "/") {
		loc = BaseURL + loc
	}
	if strings.Contains(loc, "Error.htm") {
		return "", fmt.Errorf("search returned an error page (%s) — form fields may be incomplete", loc)
	}
	c.limiter.OnSuccess()
	return loc, nil
}
