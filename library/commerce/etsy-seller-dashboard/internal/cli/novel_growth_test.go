// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy-seller-dashboard/internal/store"
)

func TestListingActionQueueCommandReadsLocalStore(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	database, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	observedAt := time.Now().UTC().Format(time.RFC3339Nano)
	mustUpsertFixture(t, database, "marketplace-insights", "insight:1", map[string]any{
		"_observed_at": observedAt,
		"keyword":      "shirt",
		"listings":     []any{map[string]any{"listingId": 1}},
	})
	mustUpsertFixture(t, database, "ads", "ads:1", map[string]any{
		"_observed_at": observedAt,
		"listing":      map[string]any{"listingId": 1},
		"totalStats":   map[string]any{"spentTotal": 100, "conversions": 0},
	})
	for _, resource := range []string{"marketplace-insights", "ads", "offsite-ads", "promotions"} {
		if err := database.SaveSyncState(resource, "", 1); err != nil {
			t.Fatal(err)
		}
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	root := RootCmd()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"--data-source", "local", "--json",
		"listing", "action-queue", "--db", dbPath,
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("command failed: %v\nstderr: %s", err, stderr.String())
	}

	var results []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &results); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, stdout.String())
	}
	if len(results) != 1 || results[0]["action"] != "review-ads" {
		t.Fatalf("unexpected results: %#v", results)
	}
}

func TestLocalAnalysisRejectsLiveDataSource(t *testing.T) {
	root := RootCmd()
	var stderr bytes.Buffer
	root.SetErr(&stderr)
	root.SetArgs([]string{"--data-source", "live", "growth", "anomalies"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "no live equivalent") {
		t.Fatalf("got %v, want no-live-equivalent error", err)
	}
}

func TestLocalAnalysisTreatsMissingDatabaseAsEmptyHistory(t *testing.T) {
	root := RootCmd()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"--data-source", "local", "--json",
		"economics", "reconcile", "--db", filepath.Join(t.TempDir(), "missing.db"),
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("command failed: %v\nstderr: %s", err, stderr.String())
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, stdout.String())
	}
	if result["status"] != "insufficient-data" {
		t.Fatalf("unexpected first-run status: %#v", result)
	}
}

func mustUpsertFixture(t *testing.T, database *store.Store, resource, identifier string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Upsert(resource, identifier, data); err != nil {
		t.Fatal(err)
	}
}
