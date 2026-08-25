// Copyright 2026 Felix Banuchi and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/health/peloton/internal/store"
)

// fixtureSyncClient satisfies the minimal interface syncResource needs
// (Get + RateLimit) so this test can drive a real sync without any HTTP
// server, auth, or clientHooks involved — it exists purely to prove the
// write path lands rows where the offline read path (internal/cli/offline.go)
// looks for them.
type fixtureSyncClient struct {
	items []json.RawMessage
}

func (f *fixtureSyncClient) Get(_ context.Context, _ string, _ map[string]string) (json.RawMessage, error) {
	page, err := json.Marshal(f.items)
	if err != nil {
		return nil, err
	}
	return page, nil
}

func (f *fixtureSyncClient) RateLimit() float64 { return 0 }

// TestSyncWorkoutsPopulatesOfflineHistory guards the fix for the bug where
// `sync --resources workouts` landed rows in the generic `resources` table
// (the sync write path) while `offline history` read from `provider_payloads`
// (a table nothing wrote to). It syncs a fixture and asserts the offline
// reader sees the same rows sync just stored.
func TestSyncWorkoutsPopulatesOfflineHistory(t *testing.T) {
	home := t.TempDir()
	dbPath := filepath.Join(home, "data", "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	fixture := &fixtureSyncClient{items: []json.RawMessage{
		json.RawMessage(`{"id":"w1","ride_id":"ride-a","start_time":"2026-01-01T10:00:00Z"}`),
		json.RawMessage(`{"id":"w2","ride_id":"ride-a","start_time":"2026-01-08T10:00:00Z"}`),
		json.RawMessage(`{"id":"w3","ride_id":"ride-b","start_time":"2026-01-09T10:00:00Z"}`),
	}}

	res := syncResource(context.Background(), fixture, db, "workouts", "", false, 0, false, false, nil, io.Discard)
	if res.Err != nil {
		t.Fatalf("sync failed: %v", res.Err)
	}
	if res.Count != len(fixture.items) {
		t.Fatalf("synced count = %d, want %d", res.Count, len(fixture.items))
	}

	// Sanity: the sync write path's own system of record has the rows.
	resourceCount, err := db.Count("workouts")
	if err != nil {
		t.Fatal(err)
	}
	if resourceCount != len(fixture.items) {
		t.Fatalf("resources table count = %d, want %d", resourceCount, len(fixture.items))
	}

	// The bug: offline reads from provider_payloads via ListProviderFacts,
	// a table the sync write path never populated. Assert it now does.
	facts, err := db.ListProviderFacts("workouts", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != len(fixture.items) {
		t.Fatalf("offline provider facts = %d, want %d (sync populated `resources` but not `provider_payloads`)", len(facts), len(fixture.items))
	}
	seen := map[string]bool{}
	for _, fact := range facts {
		seen[fact.ProviderID] = true
	}
	for _, id := range []string{"w1", "w2", "w3"} {
		if !seen[id] {
			t.Fatalf("offline provider facts missing %q: %+v", id, facts)
		}
	}

	// End-to-end: the actual `offline history` command reads the synced rows.
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := executeOffline(t, home, "offline", "history")
	if err != nil {
		t.Fatalf("offline history: %v", err)
	}
	items := offlineItems(t, got)
	if len(items) != len(fixture.items) {
		t.Fatalf("offline history returned %d items, want %d: %+v", len(items), len(fixture.items), got)
	}
}
