// Copyright 2026 Vikas and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored DSpace (Shodhganga) HTML client. Not generated — do not add the
// generated-file header; generate --force preserves this whole file.

// Package dspace is a small, self-contained client for the Shodhganga DSpace 5.3
// JSPUI web surface. Shodhganga exposes no OAI/REST/OpenSearch API, so the only
// machine-readable surface is HTML: /simple-search result pages and
// /handle/10603/<id> item pages whose Dublin Core metadata lives in <meta> tags.
// This package fetches those pages over direct HTTP and parses them into typed
// records. Parsing is split into pure functions (ParseItem, ParseSearch) so it
// is testable without network access.
package dspace

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/mvanhorn/printing-press-library/library/productivity/shodhganga/internal/cliutil"
)

// HandleNamespace is Shodhganga's fixed Handle.net prefix. Every thesis and
// community handle is 10603/<n>.
const HandleNamespace = "10603"

// Thesis is a parsed Shodhganga item record. Fields map from Dublin Core <meta>
// tags on the item page; omitempty fields are frequently absent in the source
// (Shodhganga often stores abstracts as PDFs rather than text).
type Thesis struct {
	Handle        string   `json:"handle"`           // "10603/305247"
	ID            string   `json:"id"`               // "305247"
	Title         string   `json:"title"`            // DC.title
	Researcher    string   `json:"researcher"`       // DC.creator
	Guides        []string `json:"guides,omitempty"` // DC.contributor
	University    string   `json:"university,omitempty"`
	Department    string   `json:"department,omitempty"`
	Place         string   `json:"place,omitempty"`
	Keywords      []string `json:"keywords,omitempty"` // DC.subject
	CompletedDate string   `json:"completed_date,omitempty"`
	Abstract      string   `json:"abstract,omitempty"`
	Language      string   `json:"language,omitempty"`
	Type          string   `json:"type,omitempty"`
	URI           string   `json:"uri"`
}

// SearchHit is one result row from a search or browse listing page.
type SearchHit struct {
	Handle string `json:"handle"`
	ID     string `json:"id"`
	Title  string `json:"title"`
	URL    string `json:"url"`
}

// SearchResult is a page of listing results plus the total match count DSpace
// reports ("Results 1-10 of 137604").
type SearchResult struct {
	Hits  []SearchHit `json:"hits"`
	Total int         `json:"total"`
}

// Client fetches Shodhganga HTML pages over direct HTTP. Construct with New.
type Client struct {
	BaseURL string
	http    *http.Client
	limiter *cliutil.AdaptiveLimiter
	ua      string
}

// New returns a Client for baseURL. ratePerSec seeds the adaptive limiter; pass
// <= 0 to disable pacing (limiter becomes a no-op).
func New(baseURL string, ratePerSec float64) *Client {
	if ratePerSec <= 0 {
		ratePerSec = 4 // polite default for a public academic repository
	}
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		// Per-request safety-net timeout so a stalled TCP connection cannot hang a
		// worker forever, independent of the command-level context deadline (which
		// a user can disable with --timeout 0). No single page fetch should ever
		// take this long, even on a slow Shodhganga.
		http:    &http.Client{Timeout: 120 * time.Second},
		limiter: cliutil.NewAdaptiveLimiter(ratePerSec),
		ua:      "Mozilla/5.0 (compatible; shodhganga-pp-cli/0.1)",
	}
}

const maxBodyBytes = 6 << 20 // 6 MiB cap on any single page

// get fetches path (+params) and returns the raw HTML body. It paces via the
// adaptive limiter and, on a 429 that survives retries, returns a
// *cliutil.RateLimitError so callers never confuse throttling with "no data".
func (c *Client) get(ctx context.Context, path string, params url.Values) ([]byte, error) {
	u := c.BaseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		c.limiter.Wait()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", c.ua)
		req.Header.Set("Accept", "text/html,application/xhtml+xml")
		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			// Context cancellation/timeout is terminal, not retryable.
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
		_ = resp.Body.Close()
		switch {
		case resp.StatusCode == http.StatusTooManyRequests:
			c.limiter.OnRateLimit()
			ra := cliutil.RetryAfter(resp)
			if attempt == 2 {
				return nil, &cliutil.RateLimitError{URL: u, RetryAfter: ra, Body: snippet(body)}
			}
			if ra <= 0 {
				ra = time.Second
			}
			select {
			case <-time.After(ra):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			continue
		case resp.StatusCode == http.StatusNotFound:
			return nil, ErrNotFound
		case resp.StatusCode >= 400:
			return nil, fmt.Errorf("GET %s: HTTP %d", u, resp.StatusCode)
		default:
			c.limiter.OnSuccess()
			return body, nil
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("GET %s: %w", u, lastErr)
	}
	return nil, fmt.Errorf("GET %s: exhausted retries", u)
}

// ErrNotFound is returned when a handle or page does not exist (HTTP 404).
var ErrNotFound = fmt.Errorf("not found")

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

// Search runs a keyword search. limit maps to DSpace rpp; start is the result
// offset for paging. It returns the page of hits plus the reported total.
func (c *Client) Search(ctx context.Context, query string, limit, start int) (*SearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("query is required")
	}
	if limit <= 0 {
		limit = 10
	}
	params := url.Values{}
	params.Set("query", query)
	params.Set("rpp", strconv.Itoa(limit))
	if start > 0 {
		params.Set("start", strconv.Itoa(start))
	}
	body, err := c.get(ctx, "/simple-search", params)
	if err != nil {
		return nil, err
	}
	return ParseSearch(body, c.BaseURL), nil
}

// Browse lists items under a browse facet (title, author, subject, keyword,
// dateissued, dateaccessioned). value drills into a specific facet term. It
// returns whatever /handle/10603 links the listing page carries.
func (c *Client) Browse(ctx context.Context, btype, value string, limit, start int) (*SearchResult, error) {
	if btype == "" {
		btype = "title"
	}
	if limit <= 0 {
		limit = 20
	}
	params := url.Values{}
	params.Set("type", btype)
	params.Set("rpp", strconv.Itoa(limit))
	if value != "" {
		params.Set("value", value)
	}
	if start > 0 {
		// DSpace 5 offset pagination uses `offset`; `starts_with` is an unrelated
		// alphabetical-jump filter, so it must not be sent here.
		params.Set("offset", strconv.Itoa(start))
	}
	body, err := c.get(ctx, "/browse", params)
	if err != nil {
		return nil, err
	}
	return ParseSearch(body, c.BaseURL), nil
}

// Item fetches and parses a single thesis item page. handleOrID accepts a bare
// numeric id ("305247"), a full handle ("10603/305247"), or a hdl.handle.net or
// site URL; all resolve to the same page.
func (c *Client) Item(ctx context.Context, handleOrID string) (*Thesis, error) {
	id, err := NormalizeID(handleOrID)
	if err != nil {
		return nil, err
	}
	body, err := c.get(ctx, "/handle/"+HandleNamespace+"/"+id, nil)
	if err != nil {
		return nil, err
	}
	return ParseItem(body, id, c.BaseURL), nil
}

var idRe = regexp.MustCompile(`(?:^|/)(?:` + HandleNamespace + `/)?(\d+)\s*$`)

// NormalizeID extracts the numeric handle id from any accepted form: "305247",
// "10603/305247", "http://hdl.handle.net/10603/305247",
// "https://shodhganga.inflibnet.ac.in/handle/10603/305247".
func NormalizeID(s string) (string, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "/")
	if s == "" {
		return "", fmt.Errorf("empty handle")
	}
	// Bare number.
	if _, err := strconv.Atoi(s); err == nil {
		return s, nil
	}
	if m := idRe.FindStringSubmatch(s); m != nil {
		return m[1], nil
	}
	return "", fmt.Errorf("could not parse a thesis handle from %q (expected 305247 or 10603/305247)", s)
}

// --- Pure parsers (network-free, unit-tested) ---

var (
	// Search/browse result links: <a href="/handle/10603/ID">Title</a>.
	// Tolerate additional anchor attributes (class=, data-*, etc.) between the
	// href value and the closing '>'; DSpace themes decorate result links.
	hitRe = regexp.MustCompile(`<a href="(?:https?://[^"/]+)?/handle/` + HandleNamespace + `/(\d+)"[^>]*>([^<]+)</a>`)
	// "Results 1-10 of 137604" total-count line.
	totalRe = regexp.MustCompile(`Results\s+[\d,]+\s*-\s*[\d,]+\s+of\s+([\d,]+)`)
	uniRe   = regexp.MustCompile(`(?i)universit|institut|college|vishwavidyalaya|vidyapeeth|deemed|\bIIT\b|\bNIT\b`)
	deptRe  = regexp.MustCompile(`(?i)department|\bdept\b|centre|center|school of|faculty|division`)
)

// ParseSearch extracts result hits and the total match count from a
// search/browse listing page. Duplicate handles (nav + result) are collapsed;
// the first occurrence with non-trivial title text wins.
func ParseSearch(body []byte, baseURL string) *SearchResult {
	baseURL = strings.TrimRight(baseURL, "/")
	res := &SearchResult{Hits: []SearchHit{}}
	if m := totalRe.FindSubmatch(body); m != nil {
		res.Total = atoiComma(string(m[1]))
	}
	seen := map[string]bool{}
	for _, m := range hitRe.FindAllSubmatch(body, -1) {
		id := string(m[1])
		title := cliutil.CleanText(html.UnescapeString(string(m[2])))
		if title == "" || seen[id] {
			continue
		}
		seen[id] = true
		res.Hits = append(res.Hits, SearchHit{
			Handle: HandleNamespace + "/" + id,
			ID:     id,
			Title:  title,
			URL:    baseURL + "/handle/" + HandleNamespace + "/" + id,
		})
	}
	return res
}

// ParseItem builds a Thesis from an item page's Dublin Core <meta> tags.
func ParseItem(body []byte, id, baseURL string) *Thesis {
	baseURL = strings.TrimRight(baseURL, "/")
	meta := extractMeta(body)
	t := &Thesis{
		Handle:        HandleNamespace + "/" + id,
		ID:            id,
		Title:         clean(first(meta["DC.title"])),
		Researcher:    clean(first(meta["DC.creator"])),
		Guides:        cleanAll(meta["DC.contributor"]),
		Keywords:      cleanAll(meta["DC.subject"]),
		CompletedDate: clean(first(meta["DC.date"])),
		Language:      clean(first(meta["DC.language"])),
		Type:          clean(first(meta["DC.type"])),
		URI:           firstNonEmpty(clean(first(meta["DC.identifier"])), baseURL+"/handle/"+HandleNamespace+"/"+id),
	}
	classifyPublishers(t, cleanAll(meta["DC.publisher"]))
	t.Abstract = normalizeAbstract(clean(first(meta["DCTERMS.abstract"])))
	return t
}

// extractMeta returns a map of meta name -> ordered content values, robust to
// attribute order and self-closing tags via the HTML tokenizer.
func extractMeta(body []byte) map[string][]string {
	out := map[string][]string{}
	z := html.NewTokenizer(strings.NewReader(string(body)))
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		if tt != html.StartTagToken && tt != html.SelfClosingTagToken {
			continue
		}
		name, hasAttr := z.TagName()
		if string(name) != "meta" || !hasAttr {
			continue
		}
		var metaName, content string
		var haveContent bool
		for {
			k, v, more := z.TagAttr()
			switch string(k) {
			case "name":
				metaName = string(v)
			case "content":
				content = string(v)
				haveContent = true
			}
			if !more {
				break
			}
		}
		if metaName != "" && haveContent {
			out[metaName] = append(out[metaName], content)
		}
	}
	return out
}

// classifyPublishers splits DSpace's DC.publisher triple into place, university,
// and department by keyword, falling back to positional [place, university,
// department] order when keywords are ambiguous.
func classifyPublishers(t *Thesis, pubs []string) {
	if len(pubs) == 0 {
		return
	}
	var rest []string
	for _, p := range pubs {
		switch {
		case t.University == "" && uniRe.MatchString(p):
			t.University = p
		case t.Department == "" && deptRe.MatchString(p):
			t.Department = p
		default:
			rest = append(rest, p)
		}
	}
	// Positional fallback for anything unclassified.
	for _, p := range rest {
		switch {
		case t.Place == "":
			t.Place = p
		case t.University == "":
			t.University = p
		case t.Department == "":
			t.Department = p
		}
	}
	// If nothing matched the university keyword but we do have values, prefer the
	// longest remaining as the university (departments/places are usually shorter).
	if t.University == "" && t.Place != "" {
		t.University, t.Place = t.Place, ""
	}
}

// normalizeAbstract drops Shodhganga's placeholder abstracts. Many items store
// the abstract as a PDF, leaving the meta tag as "newline" or
// "Abstract Available newline" — those carry no text and are normalized away.
func normalizeAbstract(s string) string {
	low := strings.ToLower(strings.TrimSpace(s))
	if low == "" || low == "newline" || strings.HasPrefix(low, "abstract available") {
		return ""
	}
	return s
}

func clean(s string) string { return cliutil.CleanText(html.UnescapeString(s)) }
func cleanAll(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, s := range in {
		c := clean(s)
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	return out
}
func first(in []string) string {
	if len(in) == 0 {
		return ""
	}
	return in[0]
}
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
func atoiComma(s string) int {
	n, _ := strconv.Atoi(strings.ReplaceAll(strings.TrimSpace(s), ",", ""))
	return n
}
