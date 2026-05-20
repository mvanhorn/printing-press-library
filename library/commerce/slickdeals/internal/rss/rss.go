// Copyright 2026 david. Licensed under Apache-2.0. See LICENSE.

// Package rss is a thin Slickdeals-flavored RSS 2.0 fetcher and parser. It
// understands the quirks the v0.2 handoff documents:
//
//   - pubDate uses a 2-digit year ("Mon, 11 May 26 15:23:37 +0000"), so vanilla
//     time.RFC1123 fails. ParsePubDate tries the 2-digit layout first and falls
//     through to RFC1123 / RFC1123Z for safety.
//   - The numeric deal ID lives in the URL path ("/f/19510173-foo"). When the
//     <link> isn't present, we accept the GUID form "thread-19510173".
//   - The Thumb count is embedded as "Thumb Score: +N" inside the description
//     text. ExtractThumbs reads it.
//   - The merchant is encoded two ways in Slickdeals' HTML, depending on the
//     post: data-store-slug="amazon" (new style) or data-product-exitWebsite=
//     "amazon.com" (legacy). ExtractMerchant tries both and ExtractMerchantHost
//     gives callers the legacy form when they need it.
//
// We deliberately use encoding/xml from the stdlib rather than gofeed: the
// integration owner controls go.mod, and Slickdeals' feed is plain RSS 2.0 —
// no Atom-only fields, no JSON Feed, nothing that needs gofeed's universal
// parser. Less surface area, no extra dep.
//
// Item and the core helpers (Parse, FetchURL, ParsePubDate) live here. The
// category map and category/coupons URL builders live in categories.go.
package rss

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/slickdeals/internal/cliutil"
)

// userAgent identifies us to Slickdeals so the feed owner can tell our
// traffic apart from generic curl in the logs.
const userAgent = "slickdeals-pp-cli/0.3 (+https://github.com/mvanhorn/printing-press-library)"

// defaultTimeout applies when the caller passes a nil *http.Client to FetchURL.
const defaultTimeout = 30 * time.Second

// Feed URL constants. Live endpoints; tests inject httptest.NewServer URLs.
//
// Endpoint surface verified 2026-05-14 against Slickdeals' own /forums/forumdisplay.php?f=9
// HTML which advertises these as the canonical RSS feed URLs:
//
//   - mode=frontpage&rss=1                                    -> editor-curated frontpage
//   - mode=popdeals&searcharea=deals&searchin=first&rss=1     -> community-popular deals (different from frontpage)
//   - forumchoice[]=N&searchin=first&rss=1                    -> forum-scoped feed
//   - searchin=first&searcharea=deals&q=<query>&rss=1         -> server-side keyword search
//
// The v0.2 mistake was using ?search=q (silently ignored) instead of ?q=<query>
// and never running mode=popdeals or forumchoice[]=N. See lesson
// 2026-05-14-slickdeals-rss-q-parameter-and-popdeals-endpoint.
const (
	frontpageURL = "https://slickdeals.net/newsearch.php?mode=frontpage&rss=1"
	popdealsURL  = "https://slickdeals.net/newsearch.php?mode=popdeals&searcharea=deals&searchin=first&rss=1"
)

// Item is the normalized Slickdeals RSS row. Field set unions what both the
// rss-core (this file) and rss-browse (categories.go) engineers need; the
// duplicate Thumbs / ThumbScore and Description / Summary pairs exist so
// callers built against either engineer's contract see the same data.
type Item struct {
	GUID            string    `json:"guid"`
	DealID          string    `json:"deal_id"`
	Title           string    `json:"title"`
	Link            string    `json:"link"`
	Description     string    `json:"description"`
	DescriptionHTML string    `json:"description_html,omitempty"`
	Categories      []string  `json:"categories,omitempty"`
	Category        string    `json:"category,omitempty"`
	Creator         string    `json:"creator,omitempty"`
	PubDate         time.Time `json:"pub_date"`
	Thumbs          int       `json:"thumbs"`
	ThumbScore      int       `json:"thumb_score"`
	Merchant        string    `json:"merchant,omitempty"`
	ImgURL          string    `json:"img_url,omitempty"`
	Summary         string    `json:"summary,omitempty"`
}

// rawRSS / rawChannel / rawItem mirror the XML structure for unmarshaling.
// Kept private — callers consume the normalized Item.
type rawRSS struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rawChannel `xml:"channel"`
}

type rawChannel struct {
	Items []rawItem `xml:"item"`
}

type rawItem struct {
	Title          string   `xml:"title"`
	Link           string   `xml:"link"`
	Description    string   `xml:"description"`
	ContentEncoded string   `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
	Categories     []string `xml:"category"`
	Creator        string   `xml:"http://purl.org/dc/elements/1.1/ creator"`
	PubDate        string   `xml:"pubDate"`
	GUID           string   `xml:"guid"`
}

// FetchURL retrieves a Slickdeals RSS feed and returns parsed items.
// Honors ctx for cancellation, sets a CLI-identifying User-Agent, and uses
// the caller-supplied client (or a 30s default).
func FetchURL(ctx context.Context, rawURL string, hc *http.Client) ([]Item, error) {
	if hc == nil {
		hc = &http.Client{Timeout: defaultTimeout}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/rss+xml, application/xml;q=0.9, */*;q=0.5")

	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetching %s: HTTP %d", rawURL, resp.StatusCode)
	}
	return ParseReader(resp.Body, 0)
}

// Parse reads an RSS document from a []byte body and returns up to limit
// items (limit<=0 means "no cap"). The two-arg shape exists to preserve
// the rss-browse engineer's call sites (categories.go's LiveCategory and
// LiveCoupons go through Parse).
func Parse(body []byte, limit int) ([]Item, error) {
	return ParseReader(bytes.NewReader(body), limit)
}

// ParseReader is the io.Reader-flavored parser. It exists because FetchURL
// has an *http.Response.Body in hand and would otherwise need to round-trip
// through io.ReadAll for no reason. The limit semantics match Parse.
func ParseReader(r io.Reader, limit int) ([]Item, error) {
	var feed rawRSS
	dec := xml.NewDecoder(r)
	dec.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		return input, nil
	}
	if err := dec.Decode(&feed); err != nil {
		return nil, fmt.Errorf("decoding rss: %w", err)
	}

	out := make([]Item, 0, len(feed.Channel.Items))
	for _, ri := range feed.Channel.Items {
		if limit > 0 && len(out) >= limit {
			break
		}
		htmlBody := ri.ContentEncoded
		if htmlBody == "" {
			htmlBody = ri.Description
		}
		pubDate := mustParsePubDate(strings.TrimSpace(ri.PubDate))
		categories := trimAll(ri.Categories)
		firstCategory := ""
		if len(categories) > 0 {
			firstCategory = categories[0]
		}
		// DealID prefers the URL path; falls back to GUID's "thread-<id>" form.
		dealID := ExtractDealID(strings.TrimSpace(ri.Link))
		if dealID == "" {
			dealID = extractDealID(strings.TrimSpace(ri.GUID))
		}
		thumbs := extractThumbScore(htmlBody)
		description := cliutil.CleanText(ri.Description)

		out = append(out, Item{
			GUID:            strings.TrimSpace(ri.GUID),
			DealID:          dealID,
			Title:           strings.TrimSpace(ri.Title),
			Link:            strings.TrimSpace(ri.Link),
			Description:     description,
			DescriptionHTML: htmlBody,
			Categories:      categories,
			Category:        firstCategory,
			Creator:         strings.TrimSpace(ri.Creator),
			PubDate:         pubDate,
			Thumbs:          thumbs,
			ThumbScore:      thumbs,
			Merchant:        extractMerchant(htmlBody),
			ImgURL:          extractImgURL(htmlBody),
			Summary:         description,
		})
	}
	return out, nil
}

// LiveFrontpage fetches the Slickdeals frontpage RSS feed (~25 fresh items).
func LiveFrontpage(ctx context.Context, hc *http.Client) ([]Item, error) {
	return FetchURL(ctx, frontpageURL, hc)
}

// LiveSearchRSS is a backward-compatible wrapper around LiveSearch that always
// searches across all forums (forumID=0). Callers that need forum-scoped
// search should use LiveSearch directly.
//
// Note: a v0.2 bug filtered the frontpage client-side because we used the
// wrong parameter name. v0.3 fixes this by using Slickdeals' real search
// endpoint, which returns up to ~25 matching items from across all forums.
func LiveSearchRSS(ctx context.Context, hc *http.Client, query string, limit int) ([]Item, error) {
	return LiveSearch(ctx, hc, query, 0, limit)
}

// LiveSearch is the lower-level entry point: query is the keyword (may be
// empty), forumID is an optional forum scope (0 = all deals), limit caps the
// returned slice.
func LiveSearch(ctx context.Context, hc *http.Client, query string, forumID, limit int) ([]Item, error) {
	url := buildSearchURL(query, forumID)
	items, err := FetchURL(ctx, url, hc)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

// LivePopular fetches the "Popular Deals" RSS feed (community-voted) which is
// distinct from the editor-curated frontpage. Slickdeals advertises this as
// mode=popdeals on its own forumdisplay.php?f=9 HTML.
func LivePopular(ctx context.Context, hc *http.Client, limit int) ([]Item, error) {
	items, err := FetchURL(ctx, popdealsURL, hc)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

// buildSearchURL composes the canonical search URL for the Slickdeals RSS
// endpoint. Internal helper; exposed only via LiveSearch.
func buildSearchURL(query string, forumID int) string {
	base := "https://slickdeals.net/newsearch.php?searchin=first&searcharea=deals&rss=1"
	if forumID > 0 {
		base += fmt.Sprintf("&forumchoice%%5B%%5D=%d", forumID)
	}
	if q := strings.TrimSpace(query); q != "" {
		base += "&q=" + urlEscape(q)
	}
	return base
}

// urlEscape percent-encodes a query-string value for the q= parameter.
// Uses url.QueryEscape so reserved characters like [ ] = " < > are encoded
// correctly — queries such as `Xbox [Series X]` or `price >= 50` would
// otherwise embed literals that some strict HTTP proxies reject or mis-parse.
func urlEscape(s string) string {
	return url.QueryEscape(s)
}

// FilterByQuery returns items whose title or description (case-insensitive)
// contains every whitespace-separated token in query. Empty query returns the
// input unchanged (truncated to limit). Exported so callers and tests can
// exercise the filter without a network round-trip.
func FilterByQuery(items []Item, query string, limit int) []Item {
	q := strings.TrimSpace(strings.ToLower(query))
	out := items
	if q != "" {
		tokens := strings.Fields(q)
		out = out[:0]
		for _, it := range items {
			hay := strings.ToLower(it.Title + "\n" + it.Description)
			match := true
			for _, tok := range tokens {
				if !strings.Contains(hay, tok) {
					match = false
					break
				}
			}
			if match {
				out = append(out, it)
			}
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// LiveHot pulls the frontpage feed, drops anything below minThumbs, sorts
// the remainder by thumb count descending, and truncates to limit. This is
// the v0.2 workaround for `forum=9&hotdeals=1&rss=1` returning empty — we
// filter client-side.
func LiveHot(ctx context.Context, hc *http.Client, minThumbs, limit int) ([]Item, error) {
	items, err := LiveFrontpage(ctx, hc)
	if err != nil {
		return nil, err
	}
	return FilterHot(items, minThumbs, limit), nil
}

// FilterHot exists separately from LiveHot so command handlers and tests can
// exercise the filter/sort/limit logic without a network call.
func FilterHot(items []Item, minThumbs, limit int) []Item {
	filtered := make([]Item, 0, len(items))
	for _, it := range items {
		if it.Thumbs >= minThumbs {
			filtered = append(filtered, it)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].Thumbs > filtered[j].Thumbs
	})
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered
}

// pubDateLayouts lists the formats we attempt, in priority order. The first
// entries match what Slickdeals actually emits ("Mon, 11 May 26 ..."); the
// others are defensive in case the feed format normalizes later.
var pubDateLayouts = []string{
	"Mon, _2 Jan 06 15:04:05 -0700",
	"Mon, _2 Jan 06 15:04:05 MST",
	"Mon, 2 Jan 06 15:04:05 -0700",
	"Mon, 2 Jan 06 15:04:05 MST",
	"Mon, 02 Jan 06 15:04:05 -0700",
	"Mon, 02 Jan 06 15:04:05 MST",
	time.RFC1123Z,
	time.RFC1123,
}

// ParsePubDate handles Slickdeals' 2-digit-year pubDate plus RFC1123 fallbacks.
// Returns the zero time and a non-nil error if every layout fails.
func ParsePubDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty pubDate")
	}
	var lastErr error
	for _, layout := range pubDateLayouts {
		t, err := time.Parse(layout, s)
		if err == nil {
			return t.UTC(), nil
		}
		lastErr = err
	}
	return time.Time{}, fmt.Errorf("parsing pubDate %q: %w", s, lastErr)
}

// mustParsePubDate returns the parsed time or the zero value — used in Parse
// where a malformed entry shouldn't poison the whole feed.
func mustParsePubDate(s string) time.Time {
	t, _ := ParsePubDate(s)
	return t
}

// parsePubDate is the lowercase alias categories_test.go uses. Returns zero
// time on failure (matching the legacy semantics).
func parsePubDate(s string) time.Time { return mustParsePubDate(s) }

// dealIDRE captures the numeric ID at the start of a Slickdeals /f/ path.
// The feed format is "/f/19510173-some-slug?utm_..." — we want only the ID.
var dealIDRE = regexp.MustCompile(`/f/(\d+)`)

// ExtractDealID returns the numeric deal ID embedded in a Slickdeals deal URL,
// or "" if the URL doesn't match the expected pattern.
func ExtractDealID(linkURL string) string {
	m := dealIDRE.FindStringSubmatch(linkURL)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// extractDealID is the legacy GUID-based helper from categories.go. It accepts
// "thread-19510173" or "19510173" or "foo-bar-123" and returns the trailing
// numeric segment.
func extractDealID(guid string) string {
	if idx := strings.LastIndex(guid, "-"); idx >= 0 {
		id := guid[idx+1:]
		if _, err := strconv.Atoi(id); err == nil {
			return id
		}
	}
	// Fall through: maybe the whole string is the ID.
	if _, err := strconv.Atoi(guid); err == nil {
		return guid
	}
	return guid
}

// thumbsRE pulls the signed score out of "Thumb Score: +22" / "Thumb Score: -3"
// / "Thumb Score: 22". Tolerant of the surrounding whitespace the RSS shows.
var thumbsRE = regexp.MustCompile(`Thumb\s*Score:\s*([+-]?\d+)`)

// ExtractThumbs returns the integer thumb score embedded in a description,
// or 0 if none is present.
func ExtractThumbs(description string) int {
	m := thumbsRE.FindStringSubmatch(description)
	if len(m) < 2 {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimPrefix(m[1], "+"))
	if err != nil {
		return 0
	}
	return n
}

// extractThumbScore is the lowercased alias categories_test.go calls.
func extractThumbScore(encoded string) int { return ExtractThumbs(encoded) }

// Two merchant patterns coexist in Slickdeals' HTML:
//   - data-store-slug="amazon"          (new style, slug form)
//   - data-product-exitWebsite="amazon.com" (legacy, hostname form)
// We try slug first and fall through to hostname. ExtractMerchant returns
// whichever it finds; callers that specifically need one form use the
// dedicated regexes below.
var (
	merchantSlugRE = regexp.MustCompile(`data-store-slug="([^"]+)"`)
	merchantHostRE = regexp.MustCompile(`data-product-exitWebsite="([^"]+)"`)
	imgSrcRE       = regexp.MustCompile(`<img\s+src="([^"]+)"`)
)

// ExtractMerchant returns the first merchant identifier found in description
// HTML — slug if present, else hostname. Returns "" when neither pattern hits.
func ExtractMerchant(descriptionHTML string) string {
	return extractMerchant(descriptionHTML)
}

func extractMerchant(encoded string) string {
	if m := merchantSlugRE.FindStringSubmatch(encoded); len(m) == 2 {
		return m[1]
	}
	if m := merchantHostRE.FindStringSubmatch(encoded); len(m) == 2 {
		return m[1]
	}
	return ""
}

func extractImgURL(encoded string) string {
	if m := imgSrcRE.FindStringSubmatch(encoded); len(m) == 2 {
		return m[1]
	}
	return ""
}

// trimAll trims surrounding whitespace from every entry and drops empties.
func trimAll(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		t := strings.TrimSpace(s)
		if t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
