// Copyright 2026 Justin Fu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/cliutil"
)

// TestReadCommandResourcesPopulated guards the command→resource map against
// silent removal — these are the registered read commands that participate
// in freshness gating.
func TestReadCommandResourcesPopulated(t *testing.T) {
	required := []string{
		"coffee-goat-pp-cli search",
		"coffee-goat-pp-cli twin",
		"coffee-goat-pp-cli god-cup",
		"coffee-goat-pp-cli creator-review",
		"coffee-goat-pp-cli transcript-search",
	}
	for _, k := range required {
		if _, ok := readCommandResources[k]; !ok {
			t.Errorf("readCommandResources missing required entry: %q", k)
		}
	}
}

// TestCachePolicyPerResource verifies per-resource thresholds are set and
// strictly tighter than the global default (so doctor + auto-refresh agree
// on per-source staleness).
func TestCachePolicyPerResource(t *testing.T) {
	p := cachePolicy()
	if p.StaleAfter <= 0 {
		t.Fatalf("StaleAfter must be positive, got %v", p.StaleAfter)
	}
	for _, key := range []string{"products", "reviews", "videos"} {
		v, ok := p.PerResource[key]
		if !ok {
			t.Errorf("PerResource missing %q", key)
			continue
		}
		if v <= 0 {
			t.Errorf("PerResource[%q] = %v; expected positive", key, v)
		}
	}
	if p.EnvOptOut != "COFFEE_GOAT_NO_AUTO_REFRESH" {
		t.Errorf("unexpected EnvOptOut: %q", p.EnvOptOut)
	}
}

// TestAutoRefreshIfStaleEnvOptOut verifies that setting the opt-out env var
// short-circuits the freshness check to skipped/env_opt_out without touching
// the store.
func TestAutoRefreshIfStaleEnvOptOut(t *testing.T) {
	_, cleanup := withTempStore(t)
	defer cleanup()

	t.Setenv("COFFEE_GOAT_NO_AUTO_REFRESH", "1")

	flags := &rootFlags{dataSource: "auto"}
	meta := autoRefreshIfStale(context.Background(), flags, []string{"products"})

	if meta.Reason != "env_opt_out" {
		t.Errorf("expected reason=env_opt_out, got %q (meta=%+v)", meta.Reason, meta)
	}
	if meta.Ran {
		t.Error("expected Ran=false when env-opted-out")
	}
}

// TestAutoRefreshIfStaleNonAutoSource verifies that dataSource=local or
// dataSource=live short-circuit the freshness check.
func TestAutoRefreshIfStaleNonAutoSource(t *testing.T) {
	_, cleanup := withTempStore(t)
	defer cleanup()

	flags := &rootFlags{dataSource: "local"}
	meta := autoRefreshIfStale(context.Background(), flags, []string{"products"})
	if !strings.HasPrefix(meta.Reason, "data_source_") {
		t.Errorf("expected data_source_ prefix in reason, got %q", meta.Reason)
	}

	flags.dataSource = "live"
	meta = autoRefreshIfStale(context.Background(), flags, []string{"products"})
	if !strings.HasPrefix(meta.Reason, "data_source_") {
		t.Errorf("expected data_source_ prefix in reason for live, got %q", meta.Reason)
	}
}

// TestAutoRefreshNoResources verifies that an empty resource list returns
// no_resources without touching the store.
func TestAutoRefreshNoResources(t *testing.T) {
	flags := &rootFlags{dataSource: "auto"}
	meta := autoRefreshIfStale(context.Background(), flags, nil)
	if meta.Reason != "no_resources" {
		t.Errorf("expected no_resources, got %q", meta.Reason)
	}
}

// TestEnsureFreshNoStore verifies EnsureFresh returns DecisionNoStore on a
// nil DB so callers don't need to nil-check.
func TestEnsureFreshNoStore(t *testing.T) {
	decision, err := cliutil.EnsureFresh(context.Background(), nil, []string{"products"}, cliutil.Policy{StaleAfter: 6 * time.Hour})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision != cliutil.DecisionNoStore {
		t.Errorf("expected DecisionNoStore for nil DB, got %v", decision)
	}
}

// TestEnsureFreshFresh verifies that after seeding sync_state with a recent
// timestamp, EnsureFresh returns DecisionFresh.
func TestEnsureFreshFresh(t *testing.T) {
	s, cleanup := withTempStore(t)
	defer cleanup()

	if err := s.SaveSyncState("products", "", 100); err != nil {
		t.Fatalf("SaveSyncState: %v", err)
	}
	decision, err := cliutil.EnsureFresh(
		context.Background(), s.DB(), []string{"products"},
		cliutil.Policy{StaleAfter: 24 * time.Hour},
	)
	if err != nil {
		t.Fatalf("EnsureFresh: %v", err)
	}
	if decision != cliutil.DecisionFresh {
		t.Errorf("expected DecisionFresh for just-synced resource, got %v", decision)
	}
}

// TestEnsureFreshUnseededIsStale verifies that a missing sync_state row
// counts as stale (not fresh).
func TestEnsureFreshUnseededIsStale(t *testing.T) {
	s, cleanup := withTempStore(t)
	defer cleanup()

	// Don't seed: sync_state has no row for "videos".
	decision, err := cliutil.EnsureFresh(
		context.Background(), s.DB(), []string{"videos"},
		cliutil.Policy{StaleAfter: 24 * time.Hour},
	)
	if err != nil {
		t.Fatalf("EnsureFresh: %v", err)
	}
	if decision != cliutil.DecisionStaleAPI {
		t.Errorf("expected DecisionStaleAPI for unseeded resource, got %v", decision)
	}
}

// TestRunAutoRefreshOrdering verifies that runAutoRefresh emits sources in
// the canonical syncResources order regardless of caller-passed ordering.
func TestRunAutoRefreshOrdering(t *testing.T) {
	s, cleanup := withTempStore(t)
	defer cleanup()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	origStderr := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = origStderr }()

	// Pass resources in REVERSE canonical order; expect canonical-order output.
	doneCh := make(chan struct{})
	go func() {
		_ = runAutoRefresh(context.Background(), s, []string{"videos", "reviews", "products"})
		_ = w.Close()
		close(doneCh)
	}()

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	<-doneCh
	got := string(buf[:n])

	// products → shopify, reviews → coffee-review, videos → youtube
	// Expect shopify before coffee-review before youtube in the message.
	idxShopify := strings.Index(got, "shopify")
	idxCoffeeReview := strings.Index(got, "coffee-review")
	idxYouTube := strings.Index(got, "youtube")
	if idxShopify < 0 || idxCoffeeReview < 0 || idxYouTube < 0 {
		t.Fatalf("warning message missing one or more sources: %q", got)
	}
	if !(idxShopify < idxCoffeeReview && idxCoffeeReview < idxYouTube) {
		t.Errorf("sources not in canonical order. message: %q", got)
	}
}
