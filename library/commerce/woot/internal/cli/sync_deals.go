// Copyright 2026 Matthew Vassallo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/woot/internal/store"
)

const (
	wootSyncPageSize         = 100
	wootSyncFallbackMaxPages = 100
)

type wootDealsSyncClient interface {
	Get(context.Context, string, map[string]string) (json.RawMessage, error)
	RateLimit() float64
}

type wootDealsSyncStore interface {
	GetSyncState(string) (string, time.Time, int, error)
	UpsertBatchWithSyncState(string, []json.RawMessage, string) (int, int, int, error)
	SaveSyncState(string, string, int) error
	PruneResource(string, []string) (int, error)
	Count(string) (int, error)
}

type wootDealSnapshot struct {
	IDs       []string
	TotalHits int
	Reliable  bool
}

func buildWootSyncParams(offset int, userParams *syncUserParams) (map[string]string, error) {
	query, err := buildWootDealsQuery(nil, nil, "BestSelling", wootSyncPageSize, offset, true, false)
	if err != nil {
		return nil, err
	}
	params := map[string]string{"query": query}
	userParams.applyTo("deals", params, false)
	if params["query"] != query {
		return nil, fmt.Errorf("the query parameter is reserved by deals sync")
	}
	return params, nil
}

// readWootDealSnapshot performs a verification-only catalog enumeration. Woot
// does not expose a snapshot token, so a destructive reconcile is safe only
// after two consecutive head-to-tail passes return the same complete ID set.
func readWootDealSnapshot(ctx context.Context, c wootDealsSyncClient, userParams *syncUserParams) (wootDealSnapshot, error) {
	snapshot := wootDealSnapshot{Reliable: true}
	seen := make(map[string]struct{})
	offset := 0
	pagesFetched := 0
	reportedTotalSeen := false
	complete := false

	for {
		if snapshot.TotalHits == 0 && pagesFetched >= wootSyncFallbackMaxPages {
			snapshot.Reliable = false
			break
		}
		params, err := buildWootSyncParams(offset, userParams)
		if err != nil {
			return snapshot, err
		}
		data, err := c.Get(ctx, "/graphql", params)
		if err != nil {
			return snapshot, fmt.Errorf("fetching verification snapshot: %w", err)
		}
		if isDryRunResponse(data) {
			return snapshot, fmt.Errorf("verification snapshot unexpectedly returned a dry-run response")
		}
		batch, reportedTotal, err := decodeWootDealsPage(data)
		if err != nil {
			return snapshot, err
		}
		if !reportedTotalSeen {
			snapshot.TotalHits = reportedTotal
			reportedTotalSeen = true
		} else if reportedTotal != snapshot.TotalHits {
			snapshot.Reliable = false
			if reportedTotal > snapshot.TotalHits {
				snapshot.TotalHits = reportedTotal
			}
		}
		pagesFetched++
		offset += wootSyncPageSize

		for _, deal := range batch {
			if deal.ID == "" {
				snapshot.Reliable = false
				continue
			}
			if _, duplicate := seen[deal.ID]; duplicate {
				snapshot.Reliable = false
				continue
			}
			seen[deal.ID] = struct{}{}
			snapshot.IDs = append(snapshot.IDs, deal.ID)
		}

		if snapshot.TotalHits == 0 {
			if len(batch) == 0 {
				complete = true
				break
			}
			continue
		}
		if offset >= snapshot.TotalHits {
			complete = true
			break
		}
	}

	snapshot.Reliable = snapshot.Reliable && complete && len(snapshot.IDs) == snapshot.TotalHits
	return snapshot, nil
}

func sameWootDealIDs(first map[string]struct{}, second []string) bool {
	if len(first) != len(second) {
		return false
	}
	for _, id := range second {
		if _, ok := first[id]; !ok {
			return false
		}
	}
	return true
}

func syncWootDeals(ctx context.Context, c wootDealsSyncClient, db wootDealsSyncStore, sinceTS string, full bool, maxPages int, latestOnly, prune bool, userParams *syncUserParams, syncEvents io.Writer) syncResult {
	started := time.Now()
	if syncEvents == nil {
		syncEvents = io.Discard
	}
	if sinceTS != "" {
		if humanFriendly {
			fmt.Fprintln(os.Stderr, "  deals: incremental sync ignored (Woot's All Deals feed has no temporal filter; performing snapshot pagination)")
		} else {
			fmt.Fprintln(syncEvents, `{"event":"sync_warning","resource":"deals","reason":"resource_not_incremental","message":"Woot's All Deals feed has no temporal filter; performing snapshot pagination"}`)
		}
	}

	var totalCount int
	var totalHits int
	var seenIDs []string
	seenIDSet := make(map[string]struct{})
	snapshotReliable := true
	reportedTotalSeen := false
	complete := false
	offset := 0
	if !full && !latestOnly {
		cursor, _, _, err := db.GetSyncState("deals")
		if err != nil {
			return syncResult{Resource: "deals", Err: fmt.Errorf("reading deals sync state: %w", err), Duration: time.Since(started)}
		}
		if cursor != "" {
			parsed, err := strconv.Atoi(cursor)
			if err != nil || parsed < 0 {
				return syncResult{Resource: "deals", Err: fmt.Errorf("invalid saved deals cursor %q", cursor), Duration: time.Since(started)}
			}
			offset = parsed
		}
	}
	initialOffset := offset
	pagesFetched := 0
	capHit := false

	for {
		if maxPages > 0 && pagesFetched >= maxPages {
			capHit = totalHits == 0 || offset < totalHits
			break
		}
		if maxPages == 0 && totalHits == 0 && pagesFetched >= wootSyncFallbackMaxPages {
			capHit = true
			break
		}
		params, err := buildWootSyncParams(offset, userParams)
		if err != nil {
			return syncResult{Resource: "deals", Count: totalCount, Err: err, Duration: time.Since(started)}
		}
		data, err := c.Get(ctx, "/graphql", params)
		if err != nil {
			return syncResult{Resource: "deals", Count: totalCount, Err: fmt.Errorf("fetching deals: %w", err), Duration: time.Since(started)}
		}
		if isDryRunResponse(data) {
			fmt.Fprintln(syncEvents, `{"event":"sync_dryrun","resource":"deals"}`)
			return syncResult{Resource: "deals", Duration: time.Since(started)}
		}

		batch, reportedTotal, err := decodeWootDealsPage(data)
		if err != nil {
			return syncResult{Resource: "deals", Count: totalCount, Err: err, Duration: time.Since(started)}
		}
		if !reportedTotalSeen {
			totalHits = reportedTotal
			reportedTotalSeen = true
		} else if reportedTotal != totalHits {
			snapshotReliable = false
			if reportedTotal > totalHits {
				totalHits = reportedTotal
			}
		}
		pagesFetched++
		offset += wootSyncPageSize

		var items []json.RawMessage
		if len(batch) > 0 {
			normalized := normalizeWootDeals(batch)
			items = make([]json.RawMessage, 0, len(normalized))
			for _, deal := range normalized {
				item, err := json.Marshal(deal)
				if err != nil {
					return syncResult{Resource: "deals", Count: totalCount, Err: fmt.Errorf("encoding deal %s: %w", deal.ID, err), Duration: time.Since(started)}
				}
				items = append(items, item)
				obj, err := store.DecodeJSONObject(item)
				if err != nil {
					snapshotReliable = false
					continue
				}
				id := store.ExtractResourceID("deals", obj)
				if id == "" {
					snapshotReliable = false
					continue
				}
				if _, duplicate := seenIDSet[id]; duplicate {
					snapshotReliable = false
					continue
				}
				seenIDSet[id] = struct{}{}
				seenIDs = append(seenIDs, id)
			}
		}

		stored, extractFailures, _, err := db.UpsertBatchWithSyncState("deals", items, strconv.Itoa(offset))
		if err != nil {
			return syncResult{Resource: "deals", Count: totalCount, Err: fmt.Errorf("storing deals with sync checkpoint: %w", err), Duration: time.Since(started)}
		}
		if extractFailures > 0 {
			snapshotReliable = false
			if humanFriendly {
				fmt.Fprintf(os.Stderr, "\nwarning: deals had %d item(s) with no extractable primary key; those rows were skipped.\n", extractFailures)
			} else {
				fmt.Fprintf(syncEvents, `{"event":"sync_warning","resource":"deals","reason":"primary_key_unresolved","count":%d}`+"\n", extractFailures)
			}
		}
		totalCount += stored
		if humanFriendly {
			if rate := c.RateLimit(); rate > 0 {
				fmt.Fprintf(os.Stderr, "\r  deals: %d synced of %d reported [%.1f req/s]", totalCount, totalHits, rate)
			} else {
				fmt.Fprintf(os.Stderr, "\r  deals: %d synced of %d reported", totalCount, totalHits)
			}
		} else {
			fmt.Fprintf(syncEvents, `{"event":"sync_progress","resource":"deals","fetched":%d,"total_hits":%d,"rate_rps":%.1f}`+"\n", totalCount, totalHits, c.RateLimit())
		}

		if totalHits == 0 {
			complete = len(batch) == 0
			if complete {
				break
			}
			continue
		}
		if offset >= totalHits {
			complete = true
			break
		}
	}

	if capHit && !latestOnly {
		message := fmt.Sprintf("deals reached the page cap after %d pages; data may be truncated", pagesFetched)
		if humanFriendly {
			fmt.Fprintf(os.Stderr, "\n  deals: warning: %s\n", message)
		} else {
			fmt.Fprintf(syncEvents, `{"event":"sync_warning","resource":"deals","reason":"max_pages_cap_hit","message":%q}`+"\n", message)
		}
	}
	storedCount, err := db.Count("deals")
	if err != nil {
		return syncResult{Resource: "deals", Count: totalCount, Err: fmt.Errorf("counting final stored deals: %w", err), Duration: time.Since(started)}
	}

	firstPassComplete := complete && snapshotReliable && len(seenIDs) == totalHits
	snapshotVerified := false
	if !capHit && initialOffset == 0 && firstPassComplete {
		// Keep an incomplete marker while the second pass runs. If this process
		// exits mid-verification, the next sync restarts at the catalog head.
		if err := db.SaveSyncState("deals", "0", storedCount); err != nil {
			return syncResult{Resource: "deals", Count: totalCount, Err: fmt.Errorf("marking deals snapshot for verification: %w", err), Duration: time.Since(started)}
		}
		verification, err := readWootDealSnapshot(ctx, c, userParams)
		if err != nil {
			return syncResult{Resource: "deals", Count: totalCount, Err: err, Duration: time.Since(started)}
		}
		snapshotVerified = verification.Reliable && verification.TotalHits == totalHits && sameWootDealIDs(seenIDSet, verification.IDs)
		if !snapshotVerified {
			message := fmt.Sprintf("Woot's catalog changed between consecutive scans (%d then %d unique IDs); the archive remains incomplete", len(seenIDs), len(verification.IDs))
			if humanFriendly {
				fmt.Fprintf(os.Stderr, "\n  deals: warning: %s\n", message)
			} else {
				fmt.Fprintf(syncEvents, `{"event":"sync_warning","resource":"deals","reason":"snapshot_changed_during_sync","message":%q}`+"\n", message)
			}
		}
	}

	reconcileComplete := firstPassComplete && snapshotVerified
	if prune {
		if reconcileComplete {
			deleted, err := db.PruneResource("deals", seenIDs)
			if err != nil {
				return syncResult{Resource: "deals", Count: totalCount, Err: fmt.Errorf("pruning deals: %w", err), Duration: time.Since(started)}
			}
			if humanFriendly {
				if deleted > 0 {
					fmt.Fprintf(os.Stderr, "\n  deals: removed %d expired local record(s)\n", deleted)
				}
			} else {
				fmt.Fprintf(syncEvents, `{"event":"reconcile","resource":"deals","deleted":%d}`+"\n", deleted)
			}
			storedCount, err = db.Count("deals")
			if err != nil {
				return syncResult{Resource: "deals", Count: totalCount, Err: fmt.Errorf("counting reconciled deals: %w", err), Duration: time.Since(started)}
			}
		} else {
			message := fmt.Sprintf("snapshot returned %d unique IDs for %d reported deals; stale-row pruning skipped", len(seenIDs), totalHits)
			if humanFriendly {
				fmt.Fprintf(os.Stderr, "\n  deals: warning: %s\n", message)
			} else {
				fmt.Fprintf(syncEvents, `{"event":"reconcile_skipped","resource":"deals","reason":"incomplete-snapshot","message":%q}`+"\n", message)
			}
		}
	}
	localSnapshotReady := reconcileComplete && storedCount == len(seenIDs)

	finalCursor := ""
	if capHit {
		finalCursor = strconv.Itoa(offset)
	}
	var syncWarn error
	if !capHit && !localSnapshotReady {
		// A non-empty cursor is the existing incomplete-store marker. Zero
		// deliberately restarts the next ordinary sync at the catalog head.
		finalCursor = "0"
		message := fmt.Sprintf("snapshot finished with %d stored rows and %d unique IDs from this pass for %d reported deals; next sync will restart at the catalog head", storedCount, len(seenIDs), totalHits)
		if initialOffset > 0 {
			message = fmt.Sprintf("resumed scan finished at the catalog tail; %d rows are stored, but a new head-to-tail scan is required to verify the mutable catalog", storedCount)
		} else if reconcileComplete {
			message = fmt.Sprintf("the live catalog was verified at %d IDs, but the local store contains %d rows; run sync --full without --no-prune to reconcile it", len(seenIDs), storedCount)
		}
		if humanFriendly {
			fmt.Fprintf(os.Stderr, "\n  deals: warning: %s\n", message)
		} else {
			fmt.Fprintf(syncEvents, `{"event":"sync_warning","resource":"deals","reason":"snapshot_incomplete","message":%q}`+"\n", message)
		}
		syncWarn = fmt.Errorf("catalog snapshot is not yet verified; rerun sync to restart at the catalog head")
	}
	if err := db.SaveSyncState("deals", finalCursor, storedCount); err != nil {
		return syncResult{Resource: "deals", Count: totalCount, Err: fmt.Errorf("saving final deals sync state: %w", err), Duration: time.Since(started)}
	}
	if !humanFriendly {
		fmt.Fprintf(syncEvents, `{"event":"sync_complete","resource":"deals","total":%d,"catalog_verified":%t,"store_ready":%t,"incomplete":%t,"duration_ms":%d}`+"\n", totalCount, reconcileComplete, localSnapshotReady, finalCursor != "", time.Since(started).Milliseconds())
	}
	return syncResult{Resource: "deals", Count: totalCount, Warn: syncWarn, Duration: time.Since(started)}
}
