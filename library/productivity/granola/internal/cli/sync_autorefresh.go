// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0.

package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/store"
)

// PATCH(auto-refresh): new helper that lets the PersistentPreRunE hook call
// the public-API sync (sync-api) using sensible defaults, without going
// through Cobra's command dispatch or the worker-pool / output choreography
// in newSyncCmd's RunE.
//
// The user-facing `sync-api` command keeps all of its flags (--full, --since,
// --concurrency, --resources, --param, etc.) and its existing stdout/stderr
// shape. Auto-refresh deliberately bypasses that surface because:
//
//   1. The full ceremony (param parsing, worker pool, JSON event stream,
//      strict/critical exit-code policy) is not relevant to a per-command
//      refresh; the user already paid the cost of that choreography when
//      they ran `sync-api` themselves.
//   2. Auto-refresh must not write to stdout — pure JSON consumers should
//      see only the underlying command's output. The provenance line we do
//      emit goes to stderr from the autorefresh dispatcher (see U4).
//   3. Refactoring newSyncCmd's RunE in place would invite output-shape
//      drift the plan explicitly disallows.

// ApiSyncResult captures the headline numbers from a public-API sync.
// Per-resource success/warning/error counts mirror what newSyncCmd's
// exit-code policy uses, but auto-refresh treats anything that synced
// rows as "ok" — see runApiSync below.
type ApiSyncResult struct {
	// TotalRecords counts list-stage rows — the generic resources/notes
	// tables the generated sync writes.
	TotalRecords int
	// DomainRecords counts detail-stage rows: meetings, attendees,
	// transcript_segments, folders, folder_memberships. Kept separate from
	// TotalRecords so the two stages stay individually attributable.
	//
	// PATCH(autorefresh-api-hydrates-domain-tables): before the detail stage
	// was chained in, TotalRecords was the whole story and the provenance
	// line counted rows nothing reads.
	DomainRecords int
	Resources     int
	Success       int
	Warned        int
	Errored       int
	Duration      time.Duration
}

// TotalRows is the headline count used by the auto-refresh provenance line.
// It spans both stages so "api=ok (N rows)" describes what actually landed
// rather than only what the list stage wrote.
func (r ApiSyncResult) TotalRows() int { return r.TotalRecords + r.DomainRecords }

// PATCH(autorefresh-api-hydrates-domain-tables): the detail stage's bounds
// for the auto-refresh hook. sync-api is the unbounded backfill; this path
// runs ahead of an ordinary command, so it has to stay cheap enough that the
// user does not notice it.
const (
	// autoRefreshHydrateMaxPages caps the detail stage's list phase at one
	// page (NotesPageSizeMax = 30 notes). Combined with the incremental
	// window below, the steady state is one list request plus a detail fetch
	// per note that actually changed. Backfilling an account from scratch
	// stays the explicit `sync-api` command's job.
	autoRefreshHydrateMaxPages = 1

	// autoRefreshHydrateOverlap is re-scanned on every run. updated_after is
	// evaluated against the API's clock while the checkpoint is local wall
	// time, and notes can be updated while a run is in flight; a short
	// lookback absorbs both at the cost of re-fetching a handful of recent
	// notes.
	autoRefreshHydrateOverlap = 5 * time.Minute

	// autoRefreshHydrateCheckpoint is the sync_state key the detail stage
	// records its own high-water mark under. Deliberately NOT one of
	// defaultSyncResources(): the list stage owns "notes" and advances that
	// row as it pages, so sharing it would let a successful list walk the
	// window past notes whose detail fetch never landed.
	autoRefreshHydrateCheckpoint = "notes:auto_refresh_detail"
)

// runApiSync hits the public REST API for the default resource set and
// upserts results into the local store. It is intentionally tiny: defaults
// for concurrency, no --since filter, no --full, no per-resource param
// flags, no stdout output. Errors per resource are collected into the
// result struct; the function returns a non-nil error only when no
// resource synced any rows AND every resource failed (the "totally broken"
// case the user should see), mirroring the strictest branch of newSyncCmd's
// exit-code policy.
//
// PATCH(autorefresh-api-hydrates-domain-tables): this used to run the LIST
// stage only. syncResource writes the generic resources/notes tables, which
// no read command in this CLI queries — so auto-refresh reported
// "api=ok (N rows)" on stderr while `meetings`, `attendees`,
// `transcript_segments`, and `folder_memberships` (the store every read
// command serves from since the desktop cache became unreadable) went stale
// indefinitely. The detail stage that populates those tables now runs here
// too, exactly as newSyncApiCmd chains it, and a detail-stage failure is
// reported as api=failed instead of being papered over by a successful list.
func runApiSync(ctx context.Context, flags *rootFlags) (ApiSyncResult, error) {
	started := time.Now()

	c, err := flags.newClient()
	if err != nil {
		return ApiSyncResult{Duration: time.Since(started)}, err
	}
	c.NoCache = true

	dbPath := defaultDBPath("granola-pp-cli")
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return ApiSyncResult{Duration: time.Since(started)}, fmt.Errorf("opening local database: %w", err)
	}
	defer db.Close()

	// Read the detail-stage checkpoint before the list stage runs. Both
	// stages hit /v1/notes, and the list stage rewrites sync_state as it
	// pages, so anything read afterwards is already "now".
	hydrateSince := autoRefreshHydrateWindow(db)

	resources := defaultSyncResources()
	var (
		totalSynced  int
		successCount int
		warnCount    int
		errCount     int
		firstErr     error
	)
	for _, resource := range resources {
		select {
		case <-ctx.Done():
			return ApiSyncResult{
				TotalRecords: totalSynced,
				Resources:    successCount + warnCount + errCount,
				Success:      successCount,
				Warned:       warnCount,
				Errored:      errCount,
				Duration:     time.Since(started),
			}, ctx.Err()
		default:
		}
		// Auto-refresh defaults: no --since cursor reset, no --full,
		// no max-pages override (newSyncCmd's default 100 wins), no
		// latest-only, no user params.
		//
		// PATCH(autorefresh-no-stdout): quiet streams. syncResource's
		// ndjson goes to the process stdout by default, which would land
		// ahead of the user's own `--json` payload on every command.
		res := syncResourceTo(quietSyncStreams(), c, db, resource, "", false, 0, false, nil)
		switch {
		case res.Err != nil:
			errCount++
			if firstErr == nil {
				firstErr = res.Err
			}
		case res.Warn != nil:
			// PATCH(autorefresh-warn-rows-count): include warned-resource
			// rows in TotalRecords. A warning ("partial success", e.g. a
			// rate-limit notice) does not mean zero rows were written;
			// excluding res.Count made provenance read "(0 rows)" for any
			// API sync where every resource warned even when real data
			// landed in the store.
			totalSynced += res.Count
			warnCount++
		default:
			totalSynced += res.Count
			successCount++
		}
	}

	out := ApiSyncResult{
		TotalRecords: totalSynced,
		Resources:    successCount + warnCount + errCount,
		Success:      successCount,
		Warned:       warnCount,
		Errored:      errCount,
		Duration:     time.Since(started),
	}

	// "Totally broken" branch: nothing synced and no warnings — surface
	// the first concrete error so the provenance line can show a real
	// reason. Anything else (partial success, warnings only) is treated
	// as a non-fatal refresh outcome. The detail stage is skipped here on
	// purpose: it calls the same API with the same credential, so it would
	// fail identically and only add latency to a command that is about to
	// run on stale data anyway.
	if successCount == 0 && warnCount == 0 && errCount > 0 {
		if firstErr == nil {
			firstErr = errors.New("api sync failed with no per-resource error captured")
		}
		return out, firstErr
	}

	// Detail stage — the half that reaches the tables the read commands
	// query. Bounded by autoRefreshHydrateMaxPages and the incremental
	// window; the deadline on ctx (autoRefreshContext) bounds it in wall
	// time. The second store handle is fine: the store opens SQLite in WAL
	// mode with a 5s busy timeout, and this one holds no open transaction.
	hyd, hydErr := runAPIHydrate(ctx, flags, apiHydrateOptions{
		UpdatedAfter: hydrateSince,
		DBPath:       dbPath,
		MaxPages:     autoRefreshHydrateMaxPages,
	})
	out.DomainRecords = hyd.domainRows()
	out.Duration = time.Since(started)
	if hydErr != nil {
		// Non-fatal by construction: the dispatcher captures this into the
		// refresh result and renders "api=failed: …" on the provenance
		// line. The user's command still runs. Returning nil here is what
		// produced the false-positive "api=ok" on migrated installs.
		return out, fmt.Errorf("api detail hydrate: %w", hydErr)
	}
	// Advance the checkpoint only once the rows are committed, so a failed
	// detail stage re-scans the same window on the next command instead of
	// silently skipping the notes it never fetched.
	_ = db.SaveSyncState(autoRefreshHydrateCheckpoint, "", hyd.NotesFetched)
	return out, nil
}

// autoRefreshHydrateWindow renders the updated_after filter for the detail
// stage from its own checkpoint, or "" (everything, bounded by
// autoRefreshHydrateMaxPages) when this install has never completed one.
func autoRefreshHydrateWindow(db *store.Store) string {
	_, lastSynced, _, err := db.GetSyncState(autoRefreshHydrateCheckpoint)
	if err != nil || lastSynced.IsZero() {
		return ""
	}
	// PATCH(api-list-stage-matches-live-contract): must be the UTC Z form;
	// a numeric offset 400s the whole detail stage.
	return granolaAPITimestamp(lastSynced.Add(-autoRefreshHydrateOverlap))
}

// apiKeyConfigured returns true when the CLI has a usable public-API
// credential set up — either GRANOLA_API_KEY in the environment or a
// stored API key / access token in the config file. Used by the
// auto-refresh dispatcher to decide whether to run runApiSync.
//
// Treating both env and config-file credentials as "configured" matches
// how config.Load surfaces them (cfg.GranolaApiKey is set in either
// case) and how doctor distinguishes auth_source.
func apiKeyConfigured(flags *rootFlags) bool {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return false
	}
	if cfg.GranolaApiKey != "" {
		return true
	}
	if cfg.AuthHeaderVal != "" || cfg.AccessToken != "" {
		return true
	}
	return false
}
