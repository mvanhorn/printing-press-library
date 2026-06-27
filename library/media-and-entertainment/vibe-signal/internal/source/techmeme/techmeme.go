// Copyright 2026 not0xjarvis and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored aggregator source (not generator-emitted).

// Package techmeme is the Techmeme source for vibe-signal, backed by the public
// Techmeme RSS river (no auth). Techmeme is a curated headline river, not a
// topic-searchable API: Sync fetches the current feed and filters locally by
// the query terms when a query is provided.
package techmeme

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/vibe-signal/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/vibe-signal/internal/source"
)

const (
	feedURL   = "https://www.techmeme.com/feed.xml"
	userAgent = "vibe-signal-pp-cli (+https://github.com/mvanhorn/printing-press-library)"
)

func init() { source.Register(&techmeme{limiter: cliutil.NewAdaptiveLimiter(2)}) }

type techmeme struct{ limiter *cliutil.AdaptiveLimiter }

func (t *techmeme) Name() string          { return "techmeme" }
func (t *techmeme) Description() string   { return "Techmeme tech-news headline river (RSS)" }
func (t *techmeme) AuthRequired() bool    { return false }
func (t *techmeme) TopicSearchable() bool { return false }

func (t *techmeme) Sync(ctx context.Context, opts source.SyncOptions) ([]source.Signal, error) {
	body, err := source.Fetch(ctx, t.limiter, feedURL, userAgent)
	if err != nil {
		return nil, err
	}
	return parseFeed(body, opts)
}

type rss struct {
	Channel struct {
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`
}

// pubDate layouts seen on Techmeme's RSS feed.
var pubDateLayouts = []string{time.RFC1123Z, time.RFC1123, time.RFC822Z, time.RFC822}

func parsePubDate(s string) time.Time {
	s = strings.TrimSpace(s)
	for _, layout := range pubDateLayouts {
		if ts, err := time.Parse(layout, s); err == nil {
			return ts.UTC()
		}
	}
	return time.Time{}
}

// parseFeed maps a Techmeme RSS feed to []source.Signal, applying the recency
// window (opts.Since) and local query filter (opts.Query) when set.
func parseFeed(body []byte, opts source.SyncOptions) ([]source.Signal, error) {
	var feed rss
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("techmeme: decoding RSS feed: %w", err)
	}
	terms := queryTerms(opts.Query)
	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}
	out := make([]source.Signal, 0, len(feed.Channel.Items))
	for _, it := range feed.Channel.Items {
		title := cliutil.CleanText(it.Title)
		excerpt := cliutil.CleanText(it.Description)
		published := parsePubDate(it.PubDate)
		if !opts.Since.IsZero() && !published.IsZero() && published.Before(opts.Since) {
			continue
		}
		if len(terms) > 0 && !matchesAll(title+" "+excerpt, terms) {
			continue
		}
		id := it.GUID
		if id == "" {
			id = it.Link
		}
		raw, _ := json.Marshal(it)
		out = append(out, source.Signal{
			Source:      "techmeme",
			ID:          id,
			Title:       title,
			URL:         it.Link,
			PublishedAt: published,
			Excerpt:     excerpt,
			RawJSON:     string(raw),
		})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// queryTerms lowercases and splits a query into significant terms (length > 2).
func queryTerms(q string) []string {
	var terms []string
	for _, f := range strings.Fields(strings.ToLower(q)) {
		if len(f) > 2 {
			terms = append(terms, f)
		}
	}
	return terms
}

func matchesAll(haystack string, terms []string) bool {
	h := strings.ToLower(haystack)
	for _, t := range terms {
		if !strings.Contains(h, t) {
			return false
		}
	}
	return true
}
