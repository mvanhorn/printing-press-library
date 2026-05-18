// Copyright 2026 conduyt. Licensed under Apache-2.0. See LICENSE.

// PATCH: imports changed from cliutil.Policy/FreshnessMeta to local cli package types
package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/store"
)

var readCommandResources = map[string][]string{
	"conduyt-crm-pp-cli contacts":        {"contacts"},
	"conduyt-crm-pp-cli contacts list":   {"contacts"},
	"conduyt-crm-pp-cli contacts get":    {"contacts"},
	"conduyt-crm-pp-cli contacts search": {"contacts"},
	"conduyt-crm-pp-cli deals":           {"deals"},
	"conduyt-crm-pp-cli deals list":      {"deals"},
	"conduyt-crm-pp-cli deals get":       {"deals"},
	"conduyt-crm-pp-cli deals search":    {"deals"},
	"conduyt-crm-pp-cli companies":       {"companies"},
	"conduyt-crm-pp-cli companies list":  {"companies"},
	"conduyt-crm-pp-cli companies get":   {"companies"},
	"conduyt-crm-pp-cli tasks":           {"tasks"},
	"conduyt-crm-pp-cli tasks list":      {"tasks"},
	"conduyt-crm-pp-cli tasks get":       {"tasks"},
	"conduyt-crm-pp-cli activities":        {"activities"},
	"conduyt-crm-pp-cli activities list":   {"activities"},
	"conduyt-crm-pp-cli automations":       {"automations"},
	"conduyt-crm-pp-cli automations list":  {"automations"},
}

func cachePolicy() Policy {
	staleAfter := 6 * time.Hour
	envOptOut := "CONDUYT_CRM_NO_AUTO_REFRESH"
	return Policy{
		StaleAfter:   staleAfter,
		PerResource:  map[string]time.Duration{},
		EnvOptOut:    envOptOut,
		ShareEnabled: false,
	}
}

func refreshTimeout() time.Duration {
	return 30 * time.Second
}

func autoRefreshIfStale(ctx context.Context, flags *rootFlags, resources []string) (meta FreshnessMeta) {
	started := time.Now()
	meta = FreshnessMeta{
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
		meta.Decision = "skipped"
		meta.Reason = "env_opt_out"
		return meta
	}
	dbPath := defaultDBPath("conduyt-crm-pp-cli")
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: auto-refresh skipped (open: %v)\n", err)
		meta.Decision = "error"
		meta.Reason = "open_store"
		meta.Error = err.Error()
		return meta
	}
	defer db.Close()

	decision, err := EnsureFresh(ctx, db.DB(), resources, policy)
	meta.Decision = decision.String()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: auto-refresh decision failed: %v\n", err)
		meta.Decision = "error"
		meta.Reason = "decision_failed"
		meta.Error = err.Error()
		return meta
	}
	if decision == DecisionFresh || decision == DecisionNoStore {
		meta.Reason = decision.String()
		return meta
	}

	refreshCtx, cancel := context.WithTimeout(ctx, refreshTimeout())
	defer cancel()
	meta.Ran = true
	if err := runAutoRefresh(refreshCtx, flags, db, resources); err != nil {
		fmt.Fprintf(os.Stderr, "warning: using stale conduyt-crm cache (refresh failed: %v)\n", err)
		meta.Reason = "refresh_failed"
		meta.Error = err.Error()
		return meta
	}
	meta.Reason = "refreshed"
	return meta
}

func runAutoRefresh(ctx context.Context, flags *rootFlags, db *store.Store, resources []string) error {
	c, err := flags.newClient()
	if err != nil {
		return fmt.Errorf("build client: %w", err)
	}
	c.NoCache = true
	var failures []string
	for _, resource := range resources {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		result := syncResource(c, db, resource, "", false, 1)
		if result.Err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", resource, result.Err))
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("%s", strings.Join(failures, "; "))
	}
	return nil
}
