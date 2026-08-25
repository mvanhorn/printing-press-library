package cli

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/foodpanda/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/foodpanda/internal/store"
)

// autoRefreshResources lists the resources a refresh may re-sync. Only
// "vendors" is bulk-listable; every other surface is a per-vendor read that a
// blanket refresh has no sensible scope for.
var autoRefreshResources = []string{"vendors"}

// autoRefreshMaxPages bounds an unattended refresh. A user-initiated sync may
// page an entire city; a refresh that runs before some other command must not,
// or a routine `fees` call inherits a multi-minute stall.
const autoRefreshMaxPages = 3

// autoRefreshTimeout caps the whole refresh. On expiry the command proceeds
// against existing local data rather than failing: stale results beat no
// results, and the caller asked for the underlying command, not for a sync.
const autoRefreshTimeout = 30 * time.Second

// autoRefreshIfStale re-syncs stale local data before a read command runs.
//
// It is opt-in (FOODPANDA_AUTO_REFRESH=1 or --refresh-stale) because these
// commands read a local SQLite mirror and a user running one does not expect
// network traffic. Every failure path is non-fatal — a refresh is an
// optimisation, and the command behind it must still run when the network,
// the credential, or the upstream API is unavailable.
//
// A resource is refreshed only when it has recorded the parameters its last
// sync used. foodpanda's listing endpoints are geo-scoped, so re-running one
// without the original coordinates would silently mirror a different place
// than the user synced; skipping is the only correct alternative.
func autoRefreshIfStale(ctx context.Context, flags *rootFlags, progress io.Writer) {
	if !flags.refreshStale && !cliutil.AutoRefreshEnabled() {
		return
	}
	if progress == nil {
		progress = io.Discard
	}

	dbPath := defaultDBPath("foodpanda-pp-cli")
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return
	}
	defer s.Close()

	staleAfter := cliutil.StaleAfterFromEnv()
	for _, resource := range autoRefreshResources {
		_, lastSynced, _, gerr := s.GetSyncState(resource)
		if gerr != nil {
			continue
		}
		verdict := cliutil.EnsureFresh(lastSynced, staleAfter)
		if !verdict.Stale {
			continue
		}
		params := s.GetSyncParams(resource)
		if len(params) == 0 {
			fmt.Fprintf(progress, "%s is %s old but has no recorded sync scope; run 'foodpanda-pp-cli sync --resources %s --global-param latitude=<lat> --global-param longitude=<lng>' to refresh it.\n",
				resource, verdict.Age.Round(time.Minute), resource)
			continue
		}
		refreshResource(ctx, flags, s, resource, params, verdict, progress)
	}
}

// refreshResource performs one resource's refresh under its own timeout, so a
// single slow resource cannot consume the budget of the ones after it.
func refreshResource(ctx context.Context, flags *rootFlags, s *store.Store, resource string, params map[string]string, verdict cliutil.FreshnessVerdict, progress io.Writer) {
	c, err := flags.newClient()
	if err != nil {
		return
	}

	refreshCtx, cancel := context.WithTimeout(ctx, autoRefreshTimeout)
	defer cancel()

	fmt.Fprintf(progress, "Refreshing %s (%s old)…\n", resource, verdict.Age.Round(time.Minute))

	userParams := &syncUserParams{
		flatGlobal:  map[string]string{},
		trueGlobal:  params,
		perResource: map[string]map[string]string{},
	}

	result := syncResource(refreshCtx, c, s, resource, "", false, autoRefreshMaxPages, true, false, userParams, io.Discard)
	if result.Err != nil {
		// Report and continue. The command the user actually asked for is
		// still about to run against the data already on disk.
		fmt.Fprintf(progress, "Refresh of %s failed (%v); continuing with local data.\n", resource, result.Err)
		return
	}
	_ = s.SaveSyncParams(resource, params)
}
