// Copyright 2026 dstevens. Licensed under Apache-2.0.

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
	TotalRecords int
	Resources    int
	Success      int
	Warned       int
	Errored      int
	Duration     time.Duration
}

// TotalRows is the headline count used by the auto-refresh provenance line.
func (r ApiSyncResult) TotalRows() int { return r.TotalRecords }

// runApiSync hits the public REST API for the default resource set and
// upserts results into the local store. It is intentionally tiny: defaults
// for concurrency, no --since filter, no --full, no per-resource param
// flags, no stdout output. Errors per resource are collected into the
// result struct; the function returns a non-nil error only when no
// resource synced any rows AND every resource failed (the "totally broken"
// case the user should see), mirroring the strictest branch of newSyncCmd's
// exit-code policy.
func runApiSync(ctx context.Context, flags *rootFlags) (ApiSyncResult, error) {
	started := time.Now()

	c, err := flags.newClient()
	if err != nil {
		return ApiSyncResult{Duration: time.Since(started)}, err
	}
	c.NoCache = true

	db, err := store.OpenWithContext(ctx, defaultDBPath("granola-pp-cli"))
	if err != nil {
		return ApiSyncResult{Duration: time.Since(started)}, fmt.Errorf("opening local database: %w", err)
	}
	defer db.Close()

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
		res := syncResource(c, db, resource, "", false, 0, false, nil)
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
	// as a non-fatal refresh outcome.
	if successCount == 0 && warnCount == 0 && errCount > 0 {
		if firstErr == nil {
			firstErr = errors.New("api sync failed with no per-resource error captured")
		}
		return out, firstErr
	}
	return out, nil
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
