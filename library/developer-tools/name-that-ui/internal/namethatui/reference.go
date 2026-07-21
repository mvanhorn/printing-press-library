package namethatui

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// Translation is one explicitly published NameThatUI AppKit/SwiftUI row.
// It never represents an inferred framework mapping.
type Translation struct {
	Plain     string `json:"plain"`
	AppKit    string `json:"appkit"`
	SwiftUI   string `json:"swiftui"`
	Note      string `json:"note,omitempty"`
	SourceURL string `json:"source_url"`
}

// UpdateEntry is one public feed or sitemap entry. Unknown upstream dates are
// deliberately retained without an invented timestamp.
type UpdateEntry struct {
	SourceKind     string   `json:"source_kind"`
	SourceKinds    []string `json:"source_kinds"`
	Title          string   `json:"title"`
	SourceURL      string   `json:"source_url"`
	Timestamp      string   `json:"timestamp,omitempty"`
	TimestampKnown bool     `json:"timestamp_known"`
}

// FetchPublicReference applies the same bounded public-page fetch contract as
// the sync mirror while deliberately avoiding configured auth material.
func FetchPublicReference(ctx context.Context, client *http.Client, baseURL, path string) ([]byte, error) {
	if !strings.HasPrefix(path, "/") {
		return nil, fmt.Errorf("public reference path must start with '/'")
	}
	return fetch(ctx, client, strings.TrimRight(baseURL, "/")+path)
}

// ParseTranslations parses NameThatUI's semantic translation table rather
// than relying on page-wide text or RSC serialization details.
func ParseTranslations(page []byte, baseURL string) ([]Translation, error) {
	doc, err := html.Parse(strings.NewReader(string(page)))
	if err != nil {
		return []Translation{}, fmt.Errorf("parse translation HTML: %w", err)
	}
	var table *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if table != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "table" && translationTable(n) {
			table = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if table == nil {
		return []Translation{}, fmt.Errorf("NameThatUI translation table not found")
	}
	sourceURL := strings.TrimRight(baseURL, "/") + "/translate"
	rows := tableRows(table)
	out := make([]Translation, 0, len(rows))
	for _, row := range rows {
		cells := directCells(row)
		if len(cells) != 3 {
			continue
		}
		plain := strings.TrimSpace(nodeText(cells[0]))
		appkit := strings.TrimSpace(nodeText(cells[1]))
		swiftUI := strings.TrimSpace(nodeText(cells[2]))
		if plain == "" || appkit == "" || swiftUI == "" {
			continue
		}
		note := ""
		// The row label is commonly a direct `font-medium` span. Published
		// explanatory notes are direct block spans, so distinguish them by
		// their semantic layout class rather than treating all span text as a
		// note.
		if span := directNote(cells[0]); span != nil {
			note = strings.TrimSpace(nodeText(span))
			plain = strings.TrimSpace(strings.TrimSuffix(plain, note))
		}
		if plain == "" {
			continue
		}
		out = append(out, Translation{Plain: plain, AppKit: appkit, SwiftUI: swiftUI, Note: note, SourceURL: sourceURL})
	}
	if len(out) == 0 {
		return []Translation{}, fmt.Errorf("NameThatUI translation table has no rows")
	}
	return out, nil
}

func translationTable(table *html.Node) bool {
	headings := []string{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "th" {
			headings = append(headings, strings.ToLower(strings.TrimSpace(nodeText(n))))
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(table)
	return len(headings) >= 3 && headings[0] == "the thing" && headings[1] == "appkit" && headings[2] == "swiftui"
}

func tableRows(n *html.Node) []*html.Node {
	rows := []*html.Node{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "tr" {
			rows = append(rows, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return rows
}

func directCells(row *html.Node) []*html.Node {
	cells := []*html.Node{}
	for c := row.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "td" {
			cells = append(cells, c)
		}
	}
	return cells
}

func directNote(n *html.Node) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "span" && strings.Contains(attr(c, "class"), "block") {
			return c
		}
	}
	return nil
}

type rssDocument struct {
	Channel struct {
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

type rssItem struct {
	Title   string `xml:"title"`
	Link    string `xml:"link"`
	PubDate string `xml:"pubDate"`
}

type sitemapDocument struct {
	URLs []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod"`
}

func ParseFeed(page []byte) ([]UpdateEntry, error) {
	var doc rssDocument
	if err := xml.Unmarshal(page, &doc); err != nil {
		return []UpdateEntry{}, fmt.Errorf("parse RSS feed: %w", err)
	}
	out := make([]UpdateEntry, 0, len(doc.Channel.Items))
	for _, item := range doc.Channel.Items {
		url := strings.TrimSpace(item.Link)
		if url == "" {
			continue
		}
		timestamp, known := parseUpstreamTime(item.PubDate)
		out = append(out, UpdateEntry{SourceKind: "feed", SourceKinds: []string{"feed"}, Title: strings.TrimSpace(item.Title), SourceURL: url, Timestamp: timestamp, TimestampKnown: known})
	}
	return out, nil
}

func ParseSitemap(page []byte) ([]UpdateEntry, error) {
	var doc sitemapDocument
	if err := xml.Unmarshal(page, &doc); err != nil {
		return []UpdateEntry{}, fmt.Errorf("parse sitemap: %w", err)
	}
	out := make([]UpdateEntry, 0, len(doc.URLs))
	for _, item := range doc.URLs {
		url := strings.TrimSpace(item.Loc)
		if url == "" {
			continue
		}
		timestamp, known := parseUpstreamTime(item.LastMod)
		out = append(out, UpdateEntry{SourceKind: "sitemap", SourceKinds: []string{"sitemap"}, Title: url, SourceURL: url, Timestamp: timestamp, TimestampKnown: known})
	}
	return out, nil
}

func parseUpstreamTime(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	for _, layout := range []string{time.RFC1123Z, time.RFC1123, time.RFC822Z, time.RFC822, time.RFC3339, "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC().Format(time.RFC3339), true
		}
	}
	return "", false
}

// MergeUpdates removes identical public URLs, retaining richer feed metadata
// when both sources describe the same item, then orders known timestamps
// newest-first and unknown timestamps by canonical URL.
func MergeUpdates(groups ...[]UpdateEntry) []UpdateEntry {
	byURL := map[string]UpdateEntry{}
	for _, group := range groups {
		for _, entry := range group {
			current, found := byURL[entry.SourceURL]
			if !found {
				byURL[entry.SourceURL] = entry
				continue
			}
			winner := current
			if betterUpdate(entry, current) {
				winner = entry
			}
			winner.SourceKinds = mergedSourceKinds(current.SourceKinds, entry.SourceKinds)
			byURL[entry.SourceURL] = winner
		}
	}
	out := make([]UpdateEntry, 0, len(byURL))
	for _, entry := range byURL {
		entry.SourceKinds = nonNil(entry.SourceKinds)
		out = append(out, entry)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].TimestampKnown != out[j].TimestampKnown {
			return out[i].TimestampKnown
		}
		if out[i].TimestampKnown && out[i].Timestamp != out[j].Timestamp {
			return out[i].Timestamp > out[j].Timestamp
		}
		return out[i].SourceURL < out[j].SourceURL
	})
	return out
}

func mergedSourceKinds(left, right []string) []string {
	seen := map[string]struct{}{}
	for _, value := range append(append([]string{}, left...), right...) {
		if value != "" {
			seen[value] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func betterUpdate(candidate, current UpdateEntry) bool {
	if candidate.TimestampKnown != current.TimestampKnown {
		return candidate.TimestampKnown
	}
	if candidate.SourceKind != current.SourceKind {
		return candidate.SourceKind == "feed"
	}
	return candidate.Title != "" && current.Title == ""
}

func FilterUpdates(entries []UpdateEntry, since time.Time, hasSince bool, limit int) []UpdateEntry {
	out := make([]UpdateEntry, 0, len(entries))
	for _, entry := range entries {
		if hasSince && entry.TimestampKnown {
			parsed, err := time.Parse(time.RFC3339, entry.Timestamp)
			if err == nil && parsed.Before(since) {
				continue
			}
		}
		out = append(out, entry)
		if len(out) == limit {
			break
		}
	}
	return out
}
