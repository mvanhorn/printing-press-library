// Copyright 2026 david. Licensed under Apache-2.0. See LICENSE.
// Hand-written for slickdeals-pp-cli v0.3.
//
// v0.3 rewrite: replaced v0.2's client-side keyword-filter kludge with the
// real Slickdeals forumchoice[]=N RSS endpoint. The kludge existed because
// v0.2 probed the wrong URL parameter (mode=frontpage&forumid=N is silently
// ignored — but forumchoice[]=N actually works). See lesson
// 2026-05-14-slickdeals-rss-q-parameter-and-popdeals-endpoint.

package rss

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// CategoryMap maps friendly names (lowercase) to Slickdeals forum IDs.
//
// The five forum IDs below are the ones Slickdeals' own /forums/forumdisplay.php?f=9
// HTML advertises as canonical RSS feeds. Each was verified to return ~25 items
// at HTTP 200 with the correct forum-specific channel title:
//
//	forumchoice[]=4  -> "Slickdeals Freebies Forum"
//	forumchoice[]=9  -> "Slickdeals Hot Deals Forum"
//	forumchoice[]=10 -> "Slickdeals Coupons Forum"
//	forumchoice[]=25 -> "Slickdeals Contests & Sweepstakes Forum"
//	forumchoice[]=38 -> "Slickdeals Drugstore/Grocery B&M Deals + Discussion Forum"
//
// Numeric forum IDs not in this map can still be passed to LiveCategory directly;
// Slickdeals' RSS will return whatever the forum publishes (or an empty feed
// for an unknown ID). The map is for friendly-name resolution only.
var CategoryMap = map[string]int{
	"freebies":   4,
	"freebie":    4,
	"free":       4,
	"hot":        9,
	"hot-deals":  9,
	"hotdeals":   9,
	"deals":      9,
	"coupons":    10,
	"coupon":     10,
	"codes":      10,
	"contests":   25,
	"contest":    25,
	"sweepstakes": 25,
	"giveaways":  25,
	"giveaway":   25,
	"grocery":    38,
	"drugstore":  38,
	"food":       38,
	"bm":         38,
}

// ResolveCategory accepts a numeric ID string OR a friendly name and returns
// the forum ID. Returns -1 and an error for unrecognised inputs.
func ResolveCategory(input string) (int, error) {
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "" {
		return -1, fmt.Errorf("category input is empty")
	}
	// Try numeric first — accepts any forum ID, even ones not aliased.
	if n, err := strconv.Atoi(input); err == nil {
		if n <= 0 {
			return -1, fmt.Errorf("forum ID must be positive, got %d", n)
		}
		return n, nil
	}
	if id, ok := CategoryMap[input]; ok {
		return id, nil
	}
	known := make([]string, 0, len(CategoryMap))
	seen := map[int]bool{}
	for name, id := range CategoryMap {
		if !seen[id] {
			known = append(known, name)
			seen[id] = true
		}
	}
	return -1, fmt.Errorf("unknown category %q; use a numeric forum ID or one of: %s",
		input, strings.Join(known, ", "))
}

// CategoryURL returns the canonical Slickdeals RSS URL for a given forum ID
// using the forumchoice[]= parameter that Slickdeals' own HTML advertises.
func CategoryURL(forumID int) string {
	return fmt.Sprintf("https://slickdeals.net/newsearch.php?searchin=first&forumchoice%%5B%%5D=%d&rss=1", forumID)
}

// LiveCategory fetches the forum-scoped RSS feed for forumID and returns up to
// `limit` items. Unlike v0.2 (which kludged a frontpage fetch + client-side
// keyword filter), v0.3 hits Slickdeals' real forum-scoped endpoint.
func LiveCategory(ctx context.Context, hc *http.Client, forumID, limit int) ([]Item, error) {
	items, err := FetchURL(ctx, CategoryURL(forumID), hc)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}
