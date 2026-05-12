// Package news implements RSS source registry, concurrent feed fetching,
// RSS/Atom parsing, and deterministic mention extraction for the
// pubsec-tech CLI's news↔contract correlation features.

package news

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/monitoring/pubsec-tech/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/monitoring/pubsec-tech/internal/store"
)

// DefaultSources is the curated set of federal-tech RSS feeds shipped with
// the CLI. enabled-by-default ones are tier=primary; opt-ins are tier=optional.
var DefaultSources = []store.Source{
	{ID: "nextgov-fcw", Name: "Nextgov/FCW", FeedURL: "https://www.nextgov.com/rss/all/", Category: "federal-tech", Tier: "primary", Enabled: true},
	{ID: "fedscoop", Name: "FedScoop", FeedURL: "https://fedscoop.com/feed/", Category: "federal-tech", Tier: "primary", Enabled: true},
	{ID: "cyberscoop", Name: "CyberScoop", FeedURL: "https://cyberscoop.com/feed/", Category: "federal-cyber", Tier: "primary", Enabled: true},
	{ID: "meritalk", Name: "MeriTalk", FeedURL: "https://www.meritalk.com/feed/", Category: "federal-tech", Tier: "primary", Enabled: true},
	{ID: "federal-news-network", Name: "Federal News Network", FeedURL: "https://federalnewsnetwork.com/feed/", Category: "federal", Tier: "primary", Enabled: true},
	{ID: "govexec-technology", Name: "GovExec Technology", FeedURL: "https://www.govexec.com/rss/technology/", Category: "federal-tech", Tier: "primary", Enabled: true},
	{ID: "statescoop", Name: "StateScoop", FeedURL: "https://statescoop.com/feed/", Category: "state-local-tech", Tier: "optional", Enabled: false},
	{ID: "route-fifty", Name: "Route Fifty", FeedURL: "https://www.route-fifty.com/rss/all/", Category: "state-local", Tier: "optional", Enabled: false},
	// GovTech is reachable but only with a browser-shaped UA — opt-in.
	{ID: "govtech", Name: "GovTech", FeedURL: "https://www.govtech.com/rss", Category: "state-local-tech", Tier: "optional", Enabled: false},
}

// browserUA is the canonical fetch User-Agent. Some feeds (GovTech, FNN
// category feeds) gate plain `Go-http-client/1.1` with 403; the browser
// shape passes everywhere we tested in Phase 1.9.
const browserUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_4) AppleWebKit/605.1.15 (KHTML, like Gecko) pubsec-tech-pp-cli/1.0"

// FetchResult is one feed's result after a fetch attempt.
type FetchResult struct {
	SourceID     string
	Status       int           // HTTP status code
	NotModified  bool          // 304 - skip parsing
	Items        []ParsedItem  // empty if Status != 200
	ETag         string        // ETag for next conditional request
	LastModified string        // Last-Modified header value
	Err          error         // non-nil iff fetch failed
	Took         time.Duration // wall time
}

// ParsedItem is one RSS/Atom item after normalization.
type ParsedItem struct {
	GUID        string
	Title       string
	Link        string
	Summary     string
	Content     string
	Author      string
	Categories  []string
	PublishedAt time.Time
}

// Fetcher fetches RSS feeds with conditional GETs (ETag + If-Modified-Since)
// and concurrent dispatch. Concurrency is bounded to avoid hammering small sites.
// Outbound requests pass through cliutil.AdaptiveLimiter so a 429 from any one
// feed throttles further requests and surfaces as *cliutil.RateLimitError
// (typed) rather than silently producing an empty result set.
type Fetcher struct {
	HTTP        *http.Client
	Concurrency int
	Limiter     *cliutil.AdaptiveLimiter
	MaxRetries  int
}

// NewFetcher returns a Fetcher with sensible defaults. The limiter starts at
// 4 req/s, well under the lightest RSS endpoint's documented threshold, and
// halves on every 429 it sees.
func NewFetcher() *Fetcher {
	return &Fetcher{
		HTTP:        &http.Client{Timeout: 20 * time.Second},
		Concurrency: 4,
		Limiter:     cliutil.NewAdaptiveLimiter(4),
		MaxRetries:  2,
	}
}

// FetchAll fetches every source concurrently and returns one FetchResult per source.
func (f *Fetcher) FetchAll(ctx context.Context, sources []store.Source) []FetchResult {
	concurrency := f.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}
	results := make([]FetchResult, len(sources))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i := range sources {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = f.fetchOne(ctx, sources[i])
		}(i)
	}
	wg.Wait()
	return results
}

func (f *Fetcher) fetchOne(ctx context.Context, src store.Source) FetchResult {
	start := time.Now()
	r := FetchResult{SourceID: src.ID}
	maxRetries := f.MaxRetries
	if maxRetries < 1 {
		maxRetries = 1
	}
	var resp *http.Response
	for attempt := 0; attempt < maxRetries; attempt++ {
		if f.Limiter != nil {
			f.Limiter.Wait()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, src.FeedURL, nil)
		if err != nil {
			r.Err = err
			r.Took = time.Since(start)
			return r
		}
		req.Header.Set("User-Agent", browserUA)
		req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml;q=0.9, */*;q=0.8")
		if src.LastETag != "" {
			req.Header.Set("If-None-Match", src.LastETag)
		}
		if src.LastModified != "" {
			req.Header.Set("If-Modified-Since", src.LastModified)
		}
		resp, err = f.HTTP.Do(req)
		if err != nil {
			r.Err = err
			r.Took = time.Since(start)
			return r
		}
		// 429 -> halve the limiter and retry once
		if resp.StatusCode == http.StatusTooManyRequests {
			_ = resp.Body.Close()
			if f.Limiter != nil {
				f.Limiter.OnRateLimit()
			}
			if attempt+1 < maxRetries {
				time.Sleep(time.Duration(500*(attempt+1)) * time.Millisecond)
				continue
			}
			r.Status = http.StatusTooManyRequests
			r.Err = &cliutil.RateLimitError{URL: src.FeedURL}
			r.Took = time.Since(start)
			return r
		}
		if f.Limiter != nil && resp.StatusCode == http.StatusOK {
			f.Limiter.OnSuccess()
		}
		break
	}
	defer resp.Body.Close()
	r.Status = resp.StatusCode
	r.ETag = resp.Header.Get("ETag")
	r.LastModified = resp.Header.Get("Last-Modified")
	switch resp.StatusCode {
	case http.StatusNotModified:
		r.NotModified = true
		r.Took = time.Since(start)
		return r
	case http.StatusOK:
		// Read and parse below
	default:
		r.Err = fmt.Errorf("non-2xx response: %d", resp.StatusCode)
		// Drain a small body for diagnostics
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if len(body) > 0 {
			r.Err = fmt.Errorf("%w (body excerpt: %s)", r.Err, strings.TrimSpace(string(body)))
		}
		r.Took = time.Since(start)
		return r
	}
	// Limit response size to 25MB - none of the feeds we ship should exceed a few hundred KB
	body, err := io.ReadAll(io.LimitReader(resp.Body, 25*1024*1024))
	if err != nil {
		r.Err = err
		r.Took = time.Since(start)
		return r
	}
	items, perr := Parse(body)
	if perr != nil {
		r.Err = perr
		r.Took = time.Since(start)
		return r
	}
	r.Items = items
	r.Took = time.Since(start)
	return r
}

// Parse handles RSS 2.0 and Atom 1.0 feeds. We don't pull in a dep for this -
// the RSS surface we need is small and stdlib xml is enough.
func Parse(body []byte) ([]ParsedItem, error) {
	body = stripBOM(body)
	// Inspect the root element name to decide parser branch
	root, err := peekRoot(body)
	if err != nil {
		return nil, err
	}
	switch root {
	case "rss":
		var r rssFeed
		if err := xml.Unmarshal(body, &r); err != nil {
			return nil, fmt.Errorf("rss parse: %w", err)
		}
		out := make([]ParsedItem, 0, len(r.Channel.Items))
		for _, it := range r.Channel.Items {
			out = append(out, normalizeRSS(it))
		}
		return out, nil
	case "feed":
		var a atomFeed
		if err := xml.Unmarshal(body, &a); err != nil {
			return nil, fmt.Errorf("atom parse: %w", err)
		}
		out := make([]ParsedItem, 0, len(a.Entries))
		for _, it := range a.Entries {
			out = append(out, normalizeAtom(it))
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unrecognized feed root element: %q", root)
	}
}

func peekRoot(body []byte) (string, error) {
	dec := xml.NewDecoder(strings.NewReader(string(body)))
	for {
		tok, err := dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return "", fmt.Errorf("no root element found")
			}
			return "", err
		}
		if se, ok := tok.(xml.StartElement); ok {
			return strings.ToLower(se.Name.Local), nil
		}
	}
}

func stripBOM(b []byte) []byte {
	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		return b[3:]
	}
	return b
}

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string   `xml:"title"`
	Link        string   `xml:"link"`
	Description string   `xml:"description"`
	Content     string   `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
	Author      string   `xml:"http://purl.org/dc/elements/1.1/ creator"`
	GUID        string   `xml:"guid"`
	PubDate     string   `xml:"pubDate"`
	Categories  []string `xml:"category"`
}

type atomFeed struct {
	XMLName xml.Name    `xml:"feed"`
	Entries []atomEntry `xml:"entry"`
}

type atomEntry struct {
	Title     string     `xml:"title"`
	ID        string     `xml:"id"`
	Updated   string     `xml:"updated"`
	Published string     `xml:"published"`
	Summary   string     `xml:"summary"`
	Content   string     `xml:"content"`
	Author    atomAuthor `xml:"author"`
	Links     []atomLink `xml:"link"`
	Cats      []atomCat  `xml:"category"`
}

type atomAuthor struct {
	Name string `xml:"name"`
}

type atomLink struct {
	Rel  string `xml:"rel,attr"`
	Href string `xml:"href,attr"`
}

type atomCat struct {
	Term string `xml:"term,attr"`
}

func normalizeRSS(it rssItem) ParsedItem {
	guid := strings.TrimSpace(it.GUID)
	link := strings.TrimSpace(it.Link)
	if guid == "" {
		guid = link
	}
	if guid == "" {
		// fallback to a hash of title+pubdate for stability
		h := sha1.Sum([]byte(it.Title + "|" + it.PubDate))
		guid = "sha1:" + hex.EncodeToString(h[:])
	}
	published := parseDate(it.PubDate)
	return ParsedItem{
		GUID:        guid,
		Title:       strings.TrimSpace(it.Title),
		Link:        link,
		Summary:     stripHTMLBasic(it.Description),
		Content:     stripHTMLBasic(it.Content),
		Author:      strings.TrimSpace(it.Author),
		Categories:  trimAll(it.Categories),
		PublishedAt: published,
	}
}

func normalizeAtom(it atomEntry) ParsedItem {
	guid := strings.TrimSpace(it.ID)
	var link string
	for _, l := range it.Links {
		if l.Rel == "" || l.Rel == "alternate" {
			link = l.Href
			break
		}
	}
	if guid == "" {
		guid = link
	}
	published := parseDate(it.Published)
	if published.IsZero() {
		published = parseDate(it.Updated)
	}
	cats := make([]string, 0, len(it.Cats))
	for _, c := range it.Cats {
		cats = append(cats, c.Term)
	}
	return ParsedItem{
		GUID:        guid,
		Title:       strings.TrimSpace(it.Title),
		Link:        link,
		Summary:     stripHTMLBasic(it.Summary),
		Content:     stripHTMLBasic(it.Content),
		Author:      strings.TrimSpace(it.Author.Name),
		Categories:  cats,
		PublishedAt: published,
	}
}

// parseDate handles RFC1123Z (RSS) and RFC3339 (Atom) plus a couple of
// alternates seen in the wild.
func parseDate(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC1123Z, time.RFC1123, time.RFC3339, time.RFC822Z, time.RFC822,
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func trimAll(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// stripHTMLBasic removes HTML tags and decodes the few common entities. It's
// deliberately simple — better signal than raw HTML, but we don't pretend to
// be a full HTML parser. Downstream FTS5 / mention extraction handle
// whitespace-normalized text gracefully.
func stripHTMLBasic(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	inTag := false
	for _, r := range s {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				b.WriteRune(r)
			}
		}
	}
	out := b.String()
	// Decode the most common entities. Anything we don't know stays as-is.
	out = strings.ReplaceAll(out, "&amp;", "&")
	out = strings.ReplaceAll(out, "&lt;", "<")
	out = strings.ReplaceAll(out, "&gt;", ">")
	out = strings.ReplaceAll(out, "&quot;", "\"")
	out = strings.ReplaceAll(out, "&#39;", "'")
	out = strings.ReplaceAll(out, "&#x27;", "'")
	out = strings.ReplaceAll(out, "&nbsp;", " ")
	out = strings.ReplaceAll(out, "&apos;", "'")
	// Collapse whitespace
	fields := strings.Fields(out)
	return strings.Join(fields, " ")
}

// ItemID produces a deterministic article ID from source + GUID.
func ItemID(sourceID, guid string) string {
	h := sha1.Sum([]byte(sourceID + "|" + guid))
	return hex.EncodeToString(h[:])
}

// ExtractMentions matches the article text against the provided vendor and
// agency name lists and returns Tag rows. Matching is case-insensitive and
// word-boundary aware: "BAH" won't accidentally match inside "abhor", and
// "Microsoft" won't match "microsoft.com" in a URL fragment because URLs are
// not in the cleaned text we feed in.
//
// This is the deterministic alternative to NLP/LLM-based NER. False negatives
// are fine; false positives are not - a hit means the article literally
// contains the entity name. Phase 1.5 brief calls this out as the mechanism
// that makes the news↔contract feature buildable without a sentiment model.
func ExtractMentions(text string, vendors []string, agencies []struct{ Name, Abbrev string }) []store.Tag {
	if text == "" {
		return nil
	}
	textLow := strings.ToLower(text)
	var tags []store.Tag
	seen := map[string]bool{}
	for _, v := range vendors {
		v = strings.TrimSpace(v)
		if len(v) < 4 {
			continue // skip too-short names to avoid noise
		}
		needle := strings.ToLower(v)
		if matchWord(textLow, needle) {
			key := "recipient|" + needle
			if !seen[key] {
				seen[key] = true
				tags = append(tags, store.Tag{Kind: "recipient", Value: v, MatchSpan: v})
			}
		}
	}
	for _, a := range agencies {
		// Try full name first
		full := strings.TrimSpace(a.Name)
		if len(full) >= 4 {
			if matchWord(textLow, strings.ToLower(full)) {
				key := "agency|" + strings.ToLower(full)
				if !seen[key] {
					seen[key] = true
					tags = append(tags, store.Tag{Kind: "agency", Value: full, MatchSpan: full})
				}
			}
		}
		// Try abbreviation (DOD, GSA, etc.). Require word boundaries and
		// a length >= 3 to keep noise down.
		abbr := strings.TrimSpace(a.Abbrev)
		if len(abbr) >= 3 {
			if matchWord(textLow, strings.ToLower(abbr)) {
				key := "agency|" + strings.ToLower(abbr)
				if !seen[key] {
					seen[key] = true
					tags = append(tags, store.Tag{Kind: "agency", Value: full, MatchSpan: abbr})
				}
			}
		}
	}
	return tags
}

// matchWord reports whether needle occurs in haystack with word boundaries on
// both sides. Word boundary = start-of-string, end-of-string, or a character
// that is not a letter/digit/underscore. Avoids substring false-positives.
func matchWord(haystack, needle string) bool {
	if needle == "" {
		return false
	}
	i := 0
	for {
		idx := strings.Index(haystack[i:], needle)
		if idx < 0 {
			return false
		}
		pos := i + idx
		end := pos + len(needle)
		if isWordBoundary(haystack, pos, end) {
			return true
		}
		i = pos + 1
	}
}

func isWordBoundary(s string, start, end int) bool {
	if start > 0 {
		c := s[start-1]
		if isWordChar(c) {
			return false
		}
	}
	if end < len(s) {
		c := s[end]
		if isWordChar(c) {
			return false
		}
	}
	return true
}

func isWordChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}
