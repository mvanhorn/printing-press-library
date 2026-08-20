// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// Hand-built Suno clip library sync. The generated generic syncer (sync.go) is
// GET-only in spirit and cannot reliably drive Suno's cursor-paginated
// POST /api/feed/v3 (it routes the POST but treats clips as non-paginated, so
// it stops after one page). This file implements the real walk: POST with a
// {cursor,limit} body, follow next_cursor until has_more is false, upsert each
// clip as resource_type 'clips' via the store's typed UpsertClips path so
// title/tags/prompt land in the clips_fts index.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/config"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/store"
)

const (
	sunoFeedPath        = "/api/feed/v3"
	sunoClipsResource   = "clips"
	sunoFeedPageLimit   = 20
	sunoFeedMaxPagesCap = 200
)

// sunoClipsClient is the minimal client surface the clip sync needs. *client.Client
// satisfies it via PostWithParamsAndHeaders.
type sunoClipsClient interface {
	PostWithParamsAndHeaders(ctx context.Context, path string, params map[string]string, body any, headers map[string]string) (json.RawMessage, int, error)
}

// sunoFeedResponse is the /api/feed/v3 envelope.
type sunoFeedResponse struct {
	Clips      []json.RawMessage `json:"clips"`
	NextCursor *string           `json:"next_cursor"`
	HasMore    bool              `json:"has_more"`
}

// syncSunoClips walks the Suno feed and upserts every clip into the store as
// resource_type 'clips'. maxPages caps the walk (0 -> default cap). When --full
// is false it resumes from the stored sync cursor. Returns the number of clips
// upserted. The caller is responsible for verifying auth before calling.
//
// The feed is reverse-chronological and declares no server temporal filter, so
// incremental sync is client-side via early-stop: with a boundary (an explicit
// sinceTS, or the stored last_synced_at on the automatic path) the walk starts
// at the head and stops once a page's oldest clip predates the boundary, rather
// than draining the whole library every run.
func syncSunoClips(ctx context.Context, c sunoClipsClient, db *store.Store, deviceID string, full bool, maxPages int, sinceTS string, events io.Writer) (int, error) {
	if events == nil {
		events = io.Discard
	}
	if maxPages <= 0 || maxPages > sunoFeedMaxPagesCap {
		maxPages = sunoFeedMaxPagesCap
	}
	// Dogfood: cap to a single page so the live matrix's per-command timeout
	// doesn't trip on a large library.
	if cliutil.IsDogfoodEnv() {
		maxPages = 1
	}

	existingCursor, lastSynced, _, _ := db.GetSyncState(sunoClipsResource)
	cursor := ""
	if !full {
		cursor = existingCursor
	}

	// Resolve the early-stop boundary. An explicit --since (sinceTS) wins;
	// otherwise the automatic path uses the stored last_synced_at minus a
	// clock-skew margin. A parse failure leaves earlyStop false so the walk
	// degrades to a full drain (fail-open, never silently skipping records).
	tsField := syncResourceTimestampField(sunoClipsResource)
	earlyStopSince, earlyStop := clipsEarlyStopBoundary(sinceTS, lastSynced, full, tsField)
	if earlyStop {
		// Incremental walks start at the head; a stale resume cursor would
		// point mid-feed and skip the newest records.
		cursor = ""
	}

	headers := client.SunoDynamicHeaders(deviceID)
	total := 0
	completed := false
	for page := 1; page <= maxPages; page++ {
		body := map[string]any{"limit": sunoFeedPageLimit}
		if cursor != "" {
			body["cursor"] = cursor
		}

		data, _, err := c.PostWithParamsAndHeaders(ctx, sunoFeedPath, nil, body, headers)
		if err != nil {
			return total, err
		}
		// Under --dry-run the client returns a sentinel with no clips.
		if isDryRunResponse(data) {
			return total, nil
		}

		var feed sunoFeedResponse
		if err := json.Unmarshal(data, &feed); err != nil {
			return total, fmt.Errorf("parsing feed page %d: %w", page, err)
		}

		for _, clip := range feed.Clips {
			if err := db.UpsertClips(clip); err != nil {
				// Skip a single malformed clip rather than aborting the run.
				fmt.Fprintf(events, "warning: skipping a clip on page %d: %v\n", page, err)
				continue
			}
			total++
		}

		if humanFriendly {
			fmt.Fprintf(events, "  clips: page %d, %d clips synced\n", page, total)
		} else {
			fmt.Fprintf(events, `{"event":"suno_sync_progress","resource":"clips","page":%d,"synced":%d}`+"\n", page, total)
		}

		next := ""
		if feed.NextCursor != nil {
			next = *feed.NextCursor
		}
		// Persist resume cursor after each page.
		_ = db.SaveSyncState(sunoClipsResource, next, total)

		// Client-side incremental: the feed is descending, so once this page's
		// oldest clip predates the boundary every later clip is older too. The
		// page is already upserted above, so boundary-straddling clips are kept.
		if earlyStop && pageOldestBefore(feed.Clips, tsField, earlyStopSince) {
			completed = true
			break
		}

		if !feed.HasMore || next == "" {
			completed = true
			break
		}
		cursor = next
	}

	// Clear the resume cursor only after a clean full walk (the feed reported no
	// more pages). A page-capped (--max-pages) walk leaves the last cursor saved
	// inside the loop so the next incremental run resumes from where it stopped.
	if completed {
		_ = db.SaveSyncState(sunoClipsResource, "", total)
	}
	return total, nil
}

// clipsEarlyStopBoundary resolves the incremental early-stop boundary for the
// clips feed. An explicit --since (sinceTS) takes precedence; otherwise the
// automatic path derives the boundary from the stored last_synced_at minus a
// 5-minute clock-skew margin (a fast local clock must not skip boundary records;
// over-fetch is idempotent via upsert). Returns earlyStop=false when there is no
// usable boundary (no since, no stored cursor, --full, no timestamp field, or an
// unparseable timestamp) so the caller falls back to a full drain.
func clipsEarlyStopBoundary(sinceTS string, lastSynced time.Time, full bool, tsField string) (time.Time, bool) {
	if tsField == "" {
		return time.Time{}, false
	}
	boundary := sinceTS
	if boundary == "" && !lastSynced.IsZero() && !full {
		boundary = lastSynced.Format(time.RFC3339)
	}
	if boundary == "" {
		return time.Time{}, false
	}
	ts, err := time.Parse(time.RFC3339Nano, boundary)
	if err != nil {
		return time.Time{}, false
	}
	if sinceTS == "" {
		ts = ts.Add(-5 * time.Minute)
	}
	return ts, true
}

// runSunoClipsSync runs the dedicated clip sync if "clips" is among the
// requested resources, prints a progress summary, and returns the remaining
// resources (clips removed) for the generic sync pool to handle. sinceTS, when
// set, bounds the clip walk via client-side early-stop (see syncSunoClips).
func runSunoClipsSync(ctx context.Context, c sunoClipsClient, db *store.Store, configPath string, resources []string, full bool, maxPages int, sinceTS string, events io.Writer) ([]string, error) {
	remaining := make([]string, 0, len(resources))
	wantClips := false
	for _, r := range resources {
		if r == sunoClipsResource {
			wantClips = true
			continue
		}
		remaining = append(remaining, r)
	}
	if !wantClips {
		return remaining, nil
	}

	deviceID := config.DeviceIDFor(configPath)
	count, err := syncSunoClips(ctx, c, db, deviceID, full, maxPages, sinceTS, events)
	if err != nil {
		return remaining, fmt.Errorf("syncing clips: %w", err)
	}
	if humanFriendly {
		fmt.Fprintf(events, "clips: %d synced (done)\n", count)
	} else {
		fmt.Fprintf(events, `{"event":"suno_sync_complete","resource":"clips","total":%d}`+"\n", count)
	}
	return remaining, nil
}

// sunoAuthConfigured reports whether any usable credential is present. Used by
// the sync command to fail with a clear auth error instead of dialing out (or,
// under verify mode, instead of short-circuiting silently).
func sunoAuthConfigured(configPath string) (bool, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return false, err
	}
	return cfg.AuthHeader() != "", nil
}
