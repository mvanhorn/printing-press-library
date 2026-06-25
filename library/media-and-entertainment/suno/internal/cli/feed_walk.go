// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/client"
)

// feedPage is the /api/feed/v3 response envelope (raw API shape).
type feedPage struct {
	Clips      []json.RawMessage `json:"clips"`
	NextCursor string            `json:"next_cursor"`
	HasMore    bool              `json:"has_more"`
}

// feedFetcher fetches one feed page for the given cursor. Indirected so walkFeed
// is unit-testable against a fake without a live client.
type feedFetcher func(ctx context.Context, cursor string, limit int) (feedPage, error)

// walkFeed drives fetch across pages from startCursor, calling visit per page.
// visit returns more=false to stop early (e.g. crossed a --since boundary).
// Stops on exhaustion (!has_more / empty next_cursor / short page), a sticky
// cursor (same next cursor twice), visit stopping it, or an error.
func walkFeed(ctx context.Context, fetch feedFetcher, limit int, startCursor string,
	visit func(clips []json.RawMessage) (more bool, err error)) error {
	cursor := startCursor
	lastNextCursor := ""
	for {
		page, err := fetch(ctx, cursor, limit)
		if err != nil {
			return err
		}
		more, err := visit(page.Clips)
		if err != nil {
			return err
		}
		if !more || !page.HasMore || page.NextCursor == "" || (limit > 0 && len(page.Clips) < limit) {
			return nil
		}
		if page.NextCursor == lastNextCursor {
			return nil // sticky-cursor guard: cursor not advancing
		}
		lastNextCursor = page.NextCursor
		cursor = page.NextCursor
	}
}

// feedFetcherFor returns a feedFetcher backed by a live POST to /api/feed/v3.
// feedFetcherFor is the live-client adapter (integration-tested via the command,
// not unit-tested; walkFeed's logic is covered against a fake fetcher).
func feedFetcherFor(c *client.Client) feedFetcher {
	return func(ctx context.Context, cursor string, limit int) (feedPage, error) {
		body := map[string]any{}
		if cursor != "" {
			body["cursor"] = cursor
		}
		if limit != 0 {
			body["limit"] = limit
		}
		data, _, err := c.PostQueryWithParams(ctx, "/api/feed/v3", map[string]string{}, body)
		if err != nil {
			return feedPage{}, err
		}
		var page feedPage
		if err := json.Unmarshal(data, &page); err != nil {
			return feedPage{}, fmt.Errorf("parsing feed page: %w", err)
		}
		return page, nil
	}
}
