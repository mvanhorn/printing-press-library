// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/diggmcp"
)

// `feed` lists posts from digg_get_feed, which carries no scores. Unless
// --scores=false, it first walks the same topic's ranked story feed
// (/api/feed/continuous, the feed `top` reads) once, and adds each post's
// Digg Score, source count and author count from it, joined by story
// UUID (clusterId) with the short id in the post URL as a fallback.
// Checked live 2026-11-21: on ai/top/7d, all 20 digg_get_feed ids were
// ranked-feed clusterIds, and every short id mapped to the same UUID.

// rankedFetch reads one ranked-feed page for scores.
type rankedFetch func(ctx context.Context, cursor string) (*client.FeedScorePage, error)

type feedScore struct {
	diggScore     float64
	sourceCount   int
	uniqueAuthors int
}

// scoreIndex holds one ranked-feed walk, keyed both ways.
type scoreIndex struct {
	byID    map[string]feedScore
	byShort map[string]feedScore
	pages   int
}

// rankedRange picks the ranked-feed window that covers the posts feed is
// asked for: the --since window, else --range for --view top, else 30d.
func rankedRange(o feedPageOpts, now time.Time) string {
	if !o.since.IsZero() {
		// A relative --since is resolved a moment before this runs; the
		// slack keeps "24h" from reading as just over a day.
		switch age := now.Sub(o.since) - time.Minute; {
		case age <= 24*time.Hour:
			return "24h"
		case age <= 7*24*time.Hour:
			return "7d"
		case age <= 30*24*time.Hour:
			return "30d"
		default:
			return "all"
		}
	}
	if o.view == "top" {
		switch o.feedRange {
		case "today", "24h":
			return "24h"
		case "7d", "30d":
			return o.feedRange
		case "1y", "all":
			return "all"
		}
	}
	return "30d"
}

// walkRanked reads up to maxPages ranked-feed pages, stopping when the
// feed ends. It never retries: a refusal comes back as the error.
func walkRanked(ctx context.Context, fetch rankedFetch, maxPages int) (*scoreIndex, error) {
	idx := &scoreIndex{byID: map[string]feedScore{}, byShort: map[string]feedScore{}}
	cursor := ""
	for idx.pages < maxPages {
		page, err := fetch(ctx, cursor)
		if err != nil {
			return idx, err
		}
		idx.pages++
		for _, r := range page.Posts {
			s := feedScore{diggScore: r.DiggScore, sourceCount: r.PostCount, uniqueAuthors: r.UniqueAuthors}
			if r.ClusterID != "" {
				idx.byID[r.ClusterID] = s
			}
			if r.ClusterURLID != "" {
				idx.byShort[r.ClusterURLID] = s
			}
		}
		if page.NextCursor == nil || *page.NextCursor == "" || len(page.Posts) == 0 {
			break
		}
		cursor = *page.NextCursor
	}
	return idx, nil
}

// apply fills a post's score fields when the ranked feed listed it. It
// reports whether it did; an unlisted post keeps null scores.
func (idx *scoreIndex) apply(p *feedPagePost) bool {
	if idx == nil {
		return false
	}
	s, ok := idx.byID[p.ID]
	if !ok {
		s, ok = idx.byShort[storyShortID(p.URL)]
	}
	if !ok {
		return false
	}
	score, sources, authors := s.diggScore, s.sourceCount, s.uniqueAuthors
	p.DiggScore, p.SourceCount, p.UniqueAuthors = &score, &sources, &authors
	return true
}

// rankedRefusal reads a ranked-feed 429 as a diggmcp.Refusal, so both
// sources report a refusal the same way.
func rankedRefusal(err error, now time.Time) (*diggmcp.Refusal, bool) {
	var fe *client.FeedRefusedError
	if !errors.As(err, &fe) {
		return nil, false
	}
	return &diggmcp.Refusal{
		Tool:    "ranked feed",
		ResetAt: diggmcp.ResetFromRetryAfter(fe.RetryAfter, now),
		Message: strings.TrimSpace(fe.Body),
	}, true
}
