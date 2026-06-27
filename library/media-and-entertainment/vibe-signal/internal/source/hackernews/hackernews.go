// Copyright 2026 not0xjarvis and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored aggregator source (not generator-emitted).

// Package hackernews is the Hacker News source for vibe-signal, backed by the
// public Algolia HN Search API (no auth).
package hackernews

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/vibe-signal/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/vibe-signal/internal/source"
)

const (
	searchByDateURL = "https://hn.algolia.com/api/v1/search_by_date"
	userAgent       = "vibe-signal-pp-cli (+https://github.com/mvanhorn/printing-press-library)"
	itemPermalink   = "https://news.ycombinator.com/item?id="
)

func init() { source.Register(&hn{limiter: cliutil.NewAdaptiveLimiter(5)}) }

type hn struct{ limiter *cliutil.AdaptiveLimiter }

func (h *hn) Name() string          { return "hackernews" }
func (h *hn) Description() string   { return "Hacker News stories via the Algolia HN Search API" }
func (h *hn) AuthRequired() bool    { return false }
func (h *hn) TopicSearchable() bool { return true }

func (h *hn) Sync(ctx context.Context, opts source.SyncOptions) ([]source.Signal, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}
	q := url.Values{}
	q.Set("query", opts.Query)
	q.Set("tags", "story")
	q.Set("hitsPerPage", strconv.Itoa(limit))
	if !opts.Since.IsZero() {
		q.Set("numericFilters", fmt.Sprintf("created_at_i>%d", opts.Since.Unix()))
	}
	body, err := fetchJSON(ctx, h.limiter, searchByDateURL+"?"+q.Encode())
	if err != nil {
		return nil, err
	}
	return parseSearch(body)
}

// fetchJSON is a thin wrapper so tests can exercise parseSearch directly
// against a fixture without network access.
func fetchJSON(ctx context.Context, limiter *cliutil.AdaptiveLimiter, u string) ([]byte, error) {
	return source.Fetch(ctx, limiter, u, userAgent)
}

type algoliaResponse struct {
	Hits []algoliaHit `json:"hits"`
}

type algoliaHit struct {
	ObjectID    string `json:"objectID"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Author      string `json:"author"`
	Points      int    `json:"points"`
	NumComments int    `json:"num_comments"`
	CreatedAtI  int64  `json:"created_at_i"`
	StoryText   string `json:"story_text"`
}

// parseSearch maps an Algolia HN search response to []source.Signal.
func parseSearch(body []byte) ([]source.Signal, error) {
	var resp algoliaResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("hackernews: decoding Algolia response: %w", err)
	}
	out := make([]source.Signal, 0, len(resp.Hits))
	for _, h := range resp.Hits {
		permalink := itemPermalink + h.ObjectID
		link := h.URL
		if link == "" {
			link = permalink
		}
		raw, _ := json.Marshal(h)
		out = append(out, source.Signal{
			Source:      "hackernews",
			ID:          h.ObjectID,
			Title:       cliutil.CleanText(h.Title),
			URL:         link,
			Author:      h.Author,
			Points:      h.Points,
			Comments:    h.NumComments,
			PublishedAt: time.Unix(h.CreatedAtI, 0).UTC(),
			Excerpt:     cliutil.CleanText(h.StoryText),
			RawJSON:     string(raw),
		})
	}
	return out, nil
}
