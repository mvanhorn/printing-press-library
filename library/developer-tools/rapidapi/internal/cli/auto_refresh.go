// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: auto-refresh of the local cache — schema-gated staleness check.

package cli

import (
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/rapidapi/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/rapidapi/internal/store"
	"github.com/spf13/cobra"
)

// autoRefreshLimit and autoRefreshMaxPages bound autoRefreshIfStale's call
// into syncResource to a single small page per resource — this hook runs
// synchronously in PersistentPreRunE ahead of nearly every command, so it
// must stay a quick check, not a full sync. autoRefreshMaxPages is
// deliberately 1 (not maxSyncPages): the full pagination loop belongs to
// the explicit `sync` command.
const (
	autoRefreshLimit    = 20
	autoRefreshMaxPages = 1
)

// autoRefreshIfStale refreshes the local store when the cached schema/data is
// older than the freshness window. Wired into the root PersistentPreRunE so
// every invocation keeps the local datastore current without an explicit
// sync step. No-op when the store is unavailable (offline or fresh install).
// The freshness marker only advances once a real refresh has actually
// succeeded for every default resource — on any resource error it is left
// stale so a later explicit `sync` or the next invocation's auto-refresh
// attempt retries, rather than falsely reporting the cache as current.
func autoRefreshIfStale(cmd *cobra.Command, flags *rootFlags) error {
	ctx := cmd.Context()
	fresh, err := cliutil.EnsureFresh(ctx, storePath(nil))
	if err != nil || fresh {
		return nil // fresh or store unavailable — nothing to do
	}
	s, err := store.OpenWithContext(ctx, storePath(nil))
	if err != nil {
		return nil // offline: stale-but-usable is better than failing
	}
	defer s.Close()

	// Use an isolated command for the internal GraphQL calls below, not the
	// invoking cmd itself: gqlExec inspects cmd.Flags().Changed("query"/
	// "variables") to support a raw-GraphQL-override escape hatch, and most
	// commands (including `teach`, whose --query is a required, unrelated
	// natural-language field) declare their own --query/--variables flags.
	// Passing the invoking cmd straight through would let e.g.
	// `teach --query "<question>"` bleed its raw text into these internal
	// getCategoriesByCtx/GetCollectionsCollapsed/searchApis calls as a bogus
	// GraphQL document override.
	refreshCmd := &cobra.Command{}
	refreshCmd.SetContext(ctx)

	allOK := true
	for _, res := range defaultSyncResources() {
		// The capped return is intentionally not treated as a failure here:
		// autoRefreshLimit/autoRefreshMaxPages are small by design (a quick
		// heartbeat, not a full sync — see the const doc above), so a large
		// catalog like `api` will routinely hit the one-page cap. Gating
		// freshness on "never capped" would mean the marker never advances
		// for such resources, turning every single command invocation into
		// a live background fetch forever — exactly the "slow down every
		// invocation" outcome this hook must avoid. Completeness is the
		// explicit `sync` command's job; auto-refresh only promises a
		// recent, real (if partial) check.
		if _, _, err := syncResource(refreshCmd, flags, s, res, autoRefreshLimit, autoRefreshMaxPages); err != nil {
			allOK = false
		}
	}
	if !allOK {
		return nil // advisory; leave the marker stale so the next attempt retries
	}
	return cliutil.MarkFresh(ctx, storePath(nil), time.Now())
}
