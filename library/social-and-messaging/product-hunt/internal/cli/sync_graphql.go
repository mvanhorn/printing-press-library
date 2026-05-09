// Copyright 2026 actionsslave. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/product-hunt/internal/phgraphql"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/product-hunt/internal/store"
)

// syncGQLResource syncs a single resource using the Product Hunt GraphQL API.
// It fetches all pages via cursor pagination and upserts each node into the store.
func syncGQLResource(ctx context.Context, phc *phgraphql.Client, db *store.Store, resource, sinceTS string, full bool, maxPages int) syncResult {
	started := time.Now()

	if !humanFriendly {
		fmt.Fprintf(os.Stdout, `{"event":"sync_start","resource":"%s"}`+"\n", resource)
	}

	// Resume cursor from sync_state (unless --full already cleared it).
	existingCursor, _, _, _ := db.GetSyncState(resource)
	cursor := existingCursor

	var totalCount int
	pagesFetched := 0
	pageSize := 100
	lastNextCursor := ""

	for {
		var items []json.RawMessage
		var nextCursor string
		var hasNextPage bool
		var fetchErr error

		switch resource {
		case "posts":
			conn, err := phc.GetPosts(ctx, pageSize, cursor, "", "NEWEST", false, sinceTS, "")
			fetchErr = err
			if err == nil {
				for _, e := range conn.Edges {
					b, _ := json.Marshal(e.Node)
					items = append(items, b)
				}
				nextCursor = conn.PageInfo.EndCursor
				hasNextPage = conn.PageInfo.HasNextPage
			}

		case "topics":
			conn, err := phc.GetTopics(ctx, pageSize, cursor, "", "FOLLOWERS_COUNT")
			fetchErr = err
			if err == nil {
				for _, e := range conn.Edges {
					b, _ := json.Marshal(e.Node)
					items = append(items, b)
				}
				nextCursor = conn.PageInfo.EndCursor
				hasNextPage = conn.PageInfo.HasNextPage
			}

		case "collections":
			conn, err := phc.GetCollections(ctx, pageSize, cursor, false, "FOLLOWERS_COUNT")
			fetchErr = err
			if err == nil {
				for _, e := range conn.Edges {
					b, _ := json.Marshal(e.Node)
					items = append(items, b)
				}
				nextCursor = conn.PageInfo.EndCursor
				hasNextPage = conn.PageInfo.HasNextPage
			}

		default:
			return syncResult{
				Resource: resource,
				Err:      fmt.Errorf("unknown sync resource %q", resource),
				Duration: time.Since(started),
			}
		}

		if fetchErr != nil {
			if !humanFriendly {
				fmt.Fprintf(os.Stdout, `{"event":"sync_error","resource":"%s","error":"%s"}`+"\n",
					resource, fetchErr.Error())
			}
			return syncResult{
				Resource: resource,
				Count:    totalCount,
				Err:      fmt.Errorf("fetching %s: %w", resource, fetchErr),
				Duration: time.Since(started),
			}
		}

		if len(items) > 0 {
			stored, _, err := db.UpsertBatch(resource, items)
			if err != nil {
				return syncResult{
					Resource: resource,
					Count:    totalCount,
					Err:      fmt.Errorf("upserting batch for %s: %w", resource, err),
					Duration: time.Since(started),
				}
			}
			totalCount += stored
		}

		if humanFriendly {
			fmt.Fprintf(os.Stderr, "\r  %s: %d synced", resource, totalCount)
		} else {
			fmt.Fprintf(os.Stdout, `{"event":"sync_progress","resource":"%s","fetched":%d}`+"\n",
				resource, totalCount)
		}

		if err := db.SaveSyncState(resource, nextCursor, totalCount); err != nil {
			fmt.Fprintf(os.Stderr, "\nwarning: failed to save sync state for %s: %v\n", resource, err)
		}

		pagesFetched++

		if maxPages > 0 && pagesFetched >= maxPages {
			if !humanFriendly {
				fmt.Fprintf(os.Stdout,
					`{"event":"sync_warning","resource":"%s","reason":"max_pages_cap_hit","message":"reached --max-pages cap of %d"}`+"\n",
					resource, maxPages)
			}
			break
		}

		// Sticky-cursor guard: if the API echoes the same cursor twice, abort.
		if nextCursor != "" && nextCursor == lastNextCursor {
			break
		}
		lastNextCursor = nextCursor

		if !hasNextPage || nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	// Clear cursor on full completion so the next incremental run starts fresh.
	_ = db.SaveSyncState(resource, "", totalCount)

	if !humanFriendly {
		fmt.Fprintf(os.Stdout, `{"event":"sync_complete","resource":"%s","total":%d,"duration_ms":%d}`+"\n",
			resource, totalCount, time.Since(started).Milliseconds())
	}

	return syncResult{Resource: resource, Count: totalCount, Duration: time.Since(started)}
}
