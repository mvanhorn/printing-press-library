// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/notebooklm/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/notebooklm/internal/store"
	"github.com/spf13/cobra"
)

var readCommandResources = map[string][]string{
	"notebooklm-pp-cli search": {"notebooks"},
}

func defaultSyncResources() []string {
	return []string{"notebooks"}
}

func cachePolicy() cliutil.Policy {
	return cliutil.Policy{
		StaleAfter:   6 * time.Hour,
		EnvOptOut:    "NOTEBOOKLM_NO_AUTO_REFRESH",
		ShareEnabled: false,
	}
}

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
	if flags.dataSource == "live" {
		meta.Reason = "data_source_live"
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
	dbPath, err := store.DefaultPath()
	if err != nil {
		meta.Decision = "error"
		meta.Error = err.Error()
		return meta
	}
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
		meta.Decision = "error"
		meta.Error = err.Error()
		return meta
	}
	if decision == cliutil.DecisionFresh || decision == cliutil.DecisionNoStore {
		meta.Reason = decision.String()
		return meta
	}

	refreshCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	meta.Ran = true
	if err := runAutoRefresh(refreshCtx, flags, db, resources); err != nil {
		fmt.Fprintf(os.Stderr, "warning: using stale notebooklm cache (refresh failed: %v)\n", err)
		meta.Reason = "refresh_failed"
		meta.Error = err.Error()
		return meta
	}
	meta.Reason = "refreshed"
	return meta
}

func runAutoRefresh(ctx context.Context, flags *rootFlags, db *store.Store, resources []string) error {
	client, err := newAPIClient(ctx, flags)
	if err != nil {
		return err
	}
	for _, resource := range resources {
		switch resource {
		case "notebooks":
			nbs, err := client.ListNotebooks(ctx)
			if err != nil {
				return err
			}
			now := time.Now().UTC().Format(time.RFC3339)
			for _, nb := range nbs {
				if err := db.UpsertNotebook(nb, now); err != nil {
					return err
				}
			}
			if err := db.SaveSyncState(store.SyncState{
				ResourceType: "notebooks",
				LastSyncedAt: time.Now().UTC(),
				TotalCount:   int64(len(nbs)),
			}); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown resource %q", resource)
		}
	}
	return nil
}

func ensureFreshForCommand(ctx context.Context, flags *rootFlags, commandPath string) cliutil.FreshnessMeta {
	resources, ok := readCommandResources[commandPath]
	if !ok {
		meta := cliutil.FreshnessMeta{Decision: "skipped", Reason: "unregistered_command", Source: flags.dataSource}
		flags.freshnessMeta = meta
		return meta
	}
	meta := autoRefreshIfStale(ctx, flags, resources)
	flags.freshnessMeta = meta
	return meta
}

func maybeAutoRefresh(cmd *cobra.Command, flags *rootFlags) {
	if flags == nil || flags.dryRun {
		return
	}
	path := cmd.CommandPath()
	resources, ok := readCommandResources[path]
	if !ok {
		return
	}
	flags.freshnessMeta = autoRefreshIfStale(cmd.Context(), flags, resources)
}
