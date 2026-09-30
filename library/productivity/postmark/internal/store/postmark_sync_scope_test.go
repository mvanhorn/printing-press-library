// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestClaimPostmarkSyncScopeResetsOtherServersCheckpoints(t *testing.T) {
	ctx := context.Background()
	db := openLedgerTestStore(t, filepath.Join(t.TempDir(), "scope.db"))
	stamp := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	seed := func() {
		t.Helper()
		for _, r := range []string{"messages", "bounces"} {
			if err := db.SaveSyncStateAt(r, "offset-500", 500, stamp); err != nil {
				t.Fatal(err)
			}
		}
	}

	if reset, err := db.ClaimPostmarkSyncScope(ctx, "server-a"); err != nil || reset {
		t.Fatalf("first claim on an empty archive = %v, %v", reset, err)
	}
	seed()
	if reset, err := db.ClaimPostmarkSyncScope(ctx, "server-a"); err != nil || reset {
		t.Fatalf("same server claim = %v, %v; checkpoints must be kept", reset, err)
	}
	if cursor, last, _, _ := db.GetSyncState("messages"); cursor != "offset-500" || last.IsZero() {
		t.Fatalf("same-server checkpoint changed: %q %v", cursor, last)
	}

	reset, err := db.ClaimPostmarkSyncScope(ctx, "server-b")
	if err != nil || !reset {
		t.Fatalf("switching servers = %v, %v; want a reset", reset, err)
	}
	for _, r := range []string{"messages", "bounces"} {
		cursor, last, count, err := db.GetSyncState(r)
		if err != nil || cursor != "" || !last.IsZero() {
			t.Fatalf("%s after switch = %q %v %v; want an empty checkpoint", r, cursor, last, err)
		}
		if count != 500 {
			t.Errorf("%s total_count = %d; the reset must not touch archived row counts", r, count)
		}
	}
}
