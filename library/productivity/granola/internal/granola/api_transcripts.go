// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0.

package granola

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"
)

// PATCH(api-transcript-hydrate): fetch transcripts over the internal API so the
// degraded path carries meeting content, not just titles.
//
// The document hydrate alone leaves every transcript-dependent command --
// talktime, memo run, attendee brief, collect, folder stream -- returning empty
// on a migrated install, which reads to a user (and to an agent) as "you had no
// transcripts" rather than "these were never synced". The internal API's
// per-document transcript endpoint accepts a CLI-owned session, so the content
// is reachable; it just has to be pulled one document at a time.
//
// That per-document shape is what forces the rest of this file's design. There
// is no bulk endpoint, the client is rate limited, and a full backfill is one
// call per meeting -- so the work has to be resumable across runs, bounded per
// run, and must not re-ask for transcripts that legitimately do not exist.

// DefaultTranscriptBudget bounds an explicit `sync`. Roughly two minutes at the
// client's rate limit: long enough to make real progress on a first backfill,
// short enough that the command still feels like a command.
const DefaultTranscriptBudget = 250

// AutoRefreshTranscriptBudget bounds the per-invocation auto-refresh hook.
// Small on purpose: this fires before every read command, and a user running
// `meetings list` will not wait on a backfill.
const AutoRefreshTranscriptBudget = 10

// transcriptFetchStateDDL records which meetings have already been asked about.
//
// Without it, every meeting that genuinely has no transcript -- a cancelled
// call, a note with no recording -- looks identical to one not yet fetched, and
// the CLI would re-request all of them on every sync forever. Storing the
// attempt rather than only the result is the difference between converging and
// paying the same cost each run.
const transcriptFetchStateDDL = `CREATE TABLE IF NOT EXISTS transcript_fetch_state (
	meeting_id TEXT PRIMARY KEY,
	fetched_at TEXT NOT NULL,
	segment_count INTEGER NOT NULL DEFAULT 0
)`

// TranscriptHydrateResult reports what one pass did.
type TranscriptHydrateResult struct {
	// Fetched is the number of documents asked about this run.
	Fetched int
	// WithSegments is how many of those actually carried a transcript.
	WithSegments int
	// Segments is the total segment count pulled.
	Segments int
	// Remaining is how many documents still have no recorded attempt, so the
	// caller can tell the user whether re-running will do more work.
	Remaining int
	// Skipped names documents whose fetch failed in a way worth reporting but
	// not worth aborting the run for.
	Skipped []string
}

// TranscriptFetcher is the one method the hydrator needs. Narrowing it to an
// interface keeps the polling, budgeting and bookkeeping logic testable without
// a live Granola account or an HTTP stub for the whole internal client.
// *InternalClient satisfies it.
type TranscriptFetcher interface {
	GetDocumentTranscript(id string) ([]TranscriptSegment, error)
}

// EnsureTranscriptFetchState creates the bookkeeping table. Idempotent.
func EnsureTranscriptFetchState(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, transcriptFetchStateDDL); err != nil {
		return fmt.Errorf("transcript fetch state: %w", err)
	}
	return nil
}

// HydrateTranscriptsFromAPI fills cache.Transcripts for documents that have no
// recorded fetch attempt, newest first, up to budget.
//
// It writes nothing to the domain tables itself -- SyncFromCache's existing
// segment loop does that, which keeps transcript writes on one code path
// regardless of whether the cache was readable. A budget of 0 means unbounded.
//
// Per-document failures are skipped rather than fatal: on a corpus of hundreds,
// one unreadable document must not cost the caller every other transcript in
// the run.
func HydrateTranscriptsFromAPI(ctx context.Context, db *sql.DB, cache *Cache, client TranscriptFetcher, budget int) (TranscriptHydrateResult, error) {
	var res TranscriptHydrateResult
	if cache == nil {
		return res, fmt.Errorf("nil cache")
	}
	if err := EnsureTranscriptFetchState(ctx, db); err != nil {
		return res, err
	}
	if client == nil {
		ic, err := NewInternalClient()
		if err != nil {
			return res, fmt.Errorf("hydrate transcripts: %w", err)
		}
		client = ic
	}
	if cache.Transcripts == nil {
		cache.Transcripts = map[string][]TranscriptSegment{}
	}

	attempted, err := transcriptAttemptedIDs(ctx, db)
	if err != nil {
		return res, err
	}

	candidates := transcriptCandidates(cache, attempted)
	res.Remaining = len(candidates)
	if budget > 0 && len(candidates) > budget {
		candidates = candidates[:budget]
	}

	for _, id := range candidates {
		segs, err := client.GetDocumentTranscript(id)
		if err != nil {
			if skippableTranscriptError(err) {
				// Record the attempt anyway. A document the API will never
				// return a transcript for should stop being asked about.
				_ = recordTranscriptAttempt(ctx, db, id, 0)
				res.Fetched++
				res.Remaining--
				res.Skipped = append(res.Skipped, id)
				continue
			}
			// A non-skippable failure is usually auth or rate limiting, which
			// will hit every remaining document too. Stop and report what
			// landed rather than grinding through hundreds of identical errors.
			return res, fmt.Errorf("hydrate transcripts at %s: %w", id, err)
		}
		res.Fetched++
		res.Remaining--
		if len(segs) > 0 {
			cache.Transcripts[id] = segs
			res.WithSegments++
			res.Segments += len(segs)
		}
		if err := recordTranscriptAttempt(ctx, db, id, len(segs)); err != nil {
			return res, err
		}
	}
	return res, nil
}

// transcriptCandidates returns document ids with no recorded attempt, newest
// first. Recency ordering matters because a bounded run should spend its budget
// on the meetings a user is most likely to ask about next.
func transcriptCandidates(cache *Cache, attempted map[string]bool) []string {
	type cand struct {
		id   string
		sort string
	}
	var out []cand
	for id, doc := range cache.Documents {
		if attempted[id] {
			continue
		}
		if doc.DeletedAt != nil && *doc.DeletedAt != "" {
			continue
		}
		key := doc.CreatedAt
		if doc.UpdatedAt > key {
			key = doc.UpdatedAt
		}
		out = append(out, cand{id: id, sort: key})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].sort != out[j].sort {
			return out[i].sort > out[j].sort
		}
		return out[i].id < out[j].id
	})
	ids := make([]string, 0, len(out))
	for _, c := range out {
		ids = append(ids, c.id)
	}
	return ids
}

func transcriptAttemptedIDs(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT meeting_id FROM transcript_fetch_state`)
	if err != nil {
		return nil, fmt.Errorf("transcript fetch state: read: %w", err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

func recordTranscriptAttempt(ctx context.Context, db *sql.DB, id string, n int) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO transcript_fetch_state(meeting_id, fetched_at, segment_count) VALUES (?,?,?)
		 ON CONFLICT(meeting_id) DO UPDATE SET fetched_at = excluded.fetched_at, segment_count = excluded.segment_count`,
		id, time.Now().UTC().Format(time.RFC3339), n)
	if err != nil {
		return fmt.Errorf("transcript fetch state: record %s: %w", id, err)
	}
	return nil
}

// skippableTranscriptError names the per-document failures that mean "not this
// one" rather than "the run is broken". Mirrors the note-level policy in the
// public-API hydrate: 404 and 403 are properties of one document, everything
// else (401, 429, transport) will recur on the next one.
func skippableTranscriptError(err error) bool {
	return errors.Is(err, ErrNoteNotFound) || errors.Is(err, ErrAPIForbidden)
}
