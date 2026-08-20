// Copyright 2026 Justin Fu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
)

// readCommandResources maps command paths (`cmd.CommandPath()`) to the
// resource types those commands read. The auto-refresh hook consults this
// map to decide whether to refresh the local cache before serving.
//
// Coffee-goat's read commands span three remote sources:
//   - products: cross-roaster Shopify catalog (search, twin, compare, watch,
//     producer, refill-plan)
//   - reviews: Coffee Review scores (god-cup, review lookups)
//   - videos: Hoffmann/Hedrick YouTube transcripts (creator-review,
//     transcript-search)
//
// Hand-authored commands that read primarily from user-populated tables
// (brews, beans, cupping_sessions) are intentionally absent — there is no
// remote freshness signal for personal data.
var readCommandResources = map[string][]string{
	"coffee-goat-pp-cli search":            {"products"},
	"coffee-goat-pp-cli twin":              {"products"},
	"coffee-goat-pp-cli compare":           {"products"},
	"coffee-goat-pp-cli watch":             {"products"},
	"coffee-goat-pp-cli producer":          {"products"},
	"coffee-goat-pp-cli refill-plan":       {"products"},
	"coffee-goat-pp-cli whats-next":        {"products"},
	"coffee-goat-pp-cli god-cup":           {"products", "reviews", "videos"},
	"coffee-goat-pp-cli creator-review":    {"videos"},
	"coffee-goat-pp-cli transcript-search": {"videos"},
}

func cachePolicy() cliutil.Policy {
	return cliutil.Policy{
		StaleAfter: 24 * time.Hour,
		PerResource: map[string]time.Duration{
			"products": 6 * time.Hour,
			"reviews":  24 * time.Hour,
			"videos":   48 * time.Hour,
		},
		EnvOptOut:    "COFFEE_GOAT_NO_AUTO_REFRESH",
		ShareEnabled: false,
	}
}

// autoRefreshIfStale decides whether to refresh and runs the refresh in one
// call. Refresh failures become stderr warnings and the command proceeds
// with the stale cache. The returned metadata can be attached to JSON
// provenance envelopes under meta.freshness.
func autoRefreshIfStale(ctx context.Context, flags *rootFlags, resources []string) (meta cliutil.FreshnessMeta) {
	started := time.Now()
	meta = cliutil.FreshnessMeta{
		Decision:  "skipped",
		Resources: append([]string(nil), resources...),
		Source:    flags.dataSource,
	}
	defer func() {
		meta.ElapsedMS = time.Since(started).Milliseconds()
	}()
	if flags.dataSource != "auto" {
		meta.Reason = "data_source_" + flags.dataSource
		return meta
	}
	if len(resources) == 0 {
		meta.Reason = "no_resources"
		return meta
	}
	policy := cachePolicy()
	if policy.EnvOptOut != "" && os.Getenv(policy.EnvOptOut) == "1" {
		meta.Reason = "env_opt_out"
		return meta
	}
	dbPath := defaultDBPath("coffee-goat-pp-cli")
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: auto-refresh skipped (open: %v)\n", err)
		meta.Decision = "error"
		meta.Reason = "open_store"
		meta.Error = err.Error()
		return meta
	}
	defer db.Close()

	decision, err := cliutil.EnsureFresh(ctx, db.DB(), resources, policy)
	meta.Decision = decision.String()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: auto-refresh decision failed: %v\n", err)
		meta.Decision = "error"
		meta.Reason = "decision_failed"
		meta.Error = err.Error()
		return meta
	}
	if decision == cliutil.DecisionFresh {
		meta.Reason = "fresh"
		return meta
	}

	// Stale: emit a stderr warning naming the stale sources. We intentionally
	// do NOT transparently refresh — coffee-goat's sync touches 24 Shopify
	// roasters + Coffee Review + YouTube, easily 30s+. Running that on every
	// stale read would make `search` feel hung. The explicit `sync` command
	// is the canonical refresh path; auto-refresh's job is to surface the
	// stale state to the user and to JSON envelopes via meta.freshness.
	if err := runAutoRefresh(ctx, db, resources); err != nil {
		meta.Reason = "warn_failed"
		meta.Error = err.Error()
		return meta
	}
	meta.Reason = "stale_warned"
	return meta
}

// ensureFreshForResources lets hand-authored commands participate in the same
// freshness hook as generated resource commands. Custom commands should call
// this before reading from the store.
func ensureFreshForResources(ctx context.Context, flags *rootFlags, resources ...string) cliutil.FreshnessMeta {
	meta := autoRefreshIfStale(ctx, flags, resources)
	flags.freshnessMeta = meta
	return meta
}

// ensureFreshForCommand looks up a registered command path in
// readCommandResources and applies the same freshness hook used by root
// pre-run. Returns skipped metadata for unregistered commands.
func ensureFreshForCommand(ctx context.Context, flags *rootFlags, commandPath string) cliutil.FreshnessMeta {
	resources, ok := readCommandResources[commandPath]
	if !ok {
		meta := cliutil.FreshnessMeta{
			Decision: "skipped",
			Reason:   "unregistered_command",
			Source:   flags.dataSource,
		}
		flags.freshnessMeta = meta
		return meta
	}
	return ensureFreshForResources(ctx, flags, resources...)
}

// runAutoRefresh does NOT do a full multi-source sync when a read is stale.
// coffee-goat's sync is a deliberate operation — syncing 24 Shopify roasters
// + Coffee Review + YouTube transcripts takes 30s+ in the happy case. Doing
// that transparently on every stale read would make `search` feel hung.
//
// Instead, auto-refresh emits a stderr warning naming the stale sources and
// returns nil (success). The decision metadata flowing back through
// FreshnessMeta tells JSON callers exactly which resources are stale and
// what action is recommended. Explicit `coffee-goat-pp-cli sync` remains
// the canonical refresh path; auto-refresh's job is to surface staleness,
// not paper over it.
//
// The deterministic source order (products → reviews → videos) is preserved
// across runs so log output is stable for testing.
func runAutoRefresh(ctx context.Context, db *store.Store, resources []string) error {
	sources := []string{}
	seen := map[string]bool{}
	// Iterate resources in the canonical syncResources order so the warning
	// message is deterministic regardless of caller-passed ordering.
	resourceSet := map[string]bool{}
	for _, r := range resources {
		resourceSet[r] = true
	}
	for _, r := range syncResources {
		if !resourceSet[r] {
			continue
		}
		src, ok := syncSourceForResource[r]
		if !ok {
			continue
		}
		if !seen[src] {
			seen[src] = true
			sources = append(sources, src)
		}
	}
	if len(sources) == 0 {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	fmt.Fprintf(os.Stderr,
		"warning: data is stale for %s; serving cached results. Run 'coffee-goat-pp-cli sync --source %s' to refresh.\n",
		strings.Join(sources, ", "), sources[0],
	)
	return nil
}
