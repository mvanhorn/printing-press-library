// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func openLedgerTestStore(t *testing.T, path string) *Store {
	t.Helper()
	db, err := OpenWithContext(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// reserveAndComplete records a confirmed send the way email send-once does.
func reserveAndComplete(t *testing.T, db *Store, e PostmarkLedgerEntry, window time.Duration, messageID string) {
	t.Helper()
	ctx := context.Background()
	reservation, prior, err := db.PostmarkLedgerReserve(ctx, e, window)
	if err != nil || prior != nil || reservation == "" {
		t.Fatalf("reserve %s = %q, %+v, %v", e.Key, reservation, prior, err)
	}
	if err := db.PostmarkLedgerComplete(ctx, reservation, messageID); err != nil {
		t.Fatalf("complete %s: %v", e.Key, err)
	}
}

func TestPostmarkLedgerRoundTrip(t *testing.T) {
	ctx := context.Background()
	db := openLedgerTestStore(t, filepath.Join(t.TempDir(), "ledger.db"))

	window := 15 * time.Minute
	miss, err := db.PostmarkLedgerLookup(ctx, "otp-4821", "scope-a", false, time.Now().Add(-window))
	if err != nil || miss != nil {
		t.Fatalf("empty ledger lookup = %+v, %v", miss, err)
	}
	entry := PostmarkLedgerEntry{Key: "otp-4821", Recipient: "jane@example.com", Server: "scope-a", Stream: "outbound"}
	reserveAndComplete(t, db, entry, window, "m-1")

	since := time.Now().Add(-window)
	cases := []struct {
		name    string
		key     string
		server  string
		sandbox bool
		since   time.Time
		want    string
	}{
		{"hit inside window", "otp-4821", "scope-a", false, since, "m-1"},
		{"outside window", "otp-4821", "scope-a", false, time.Now().Add(time.Minute), ""},
		{"other key", "otp-9999", "scope-a", false, since, ""},
		{"other server", "otp-4821", "scope-b", false, since, ""},
		{"sandbox does not match live", "otp-4821", "scope-a", true, since, ""},
	}
	for _, tc := range cases {
		got, err := db.PostmarkLedgerLookup(ctx, tc.key, tc.server, tc.sandbox, tc.since)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		gotID := ""
		if got != nil {
			gotID = got.MessageID
		}
		if gotID != tc.want {
			t.Errorf("%s: message = %q, want %q", tc.name, gotID, tc.want)
		}
	}

	// A second reservation inside the window returns the confirmed send.
	_, prior, err := db.PostmarkLedgerReserve(ctx, entry, window)
	if err != nil || prior == nil || prior.MessageID != "m-1" || prior.Pending() {
		t.Fatalf("reserve over a confirmed send = %+v, %v", prior, err)
	}
	// Once the earlier send is outside the window the key can be reserved again.
	time.Sleep(2 * time.Millisecond)
	reserveAndComplete(t, db, entry, time.Millisecond, "m-2")
	got, err := db.PostmarkLedgerLookup(ctx, "otp-4821", "scope-a", false, since)
	if err != nil || got == nil || got.MessageID != "m-2" {
		t.Fatalf("newest entry = %+v, %v", got, err)
	}
}

func TestPostmarkLedgerReservationOwnership(t *testing.T) {
	ctx := context.Background()
	db := openLedgerTestStore(t, filepath.Join(t.TempDir(), "ledger.db"))
	now := time.Now()
	since := now.Add(-15 * time.Minute)

	a, _, err := db.PostmarkLedgerReserve(ctx, PostmarkLedgerEntry{Key: "k", Server: "scope-a"}, 15*time.Minute)
	if err != nil || a == "" {
		t.Fatalf("reserve scope-a: %q %v", a, err)
	}
	// Same key on another server and in sandbox mode are separate scopes.
	b, prior, err := db.PostmarkLedgerReserve(ctx, PostmarkLedgerEntry{Key: "k", Server: "scope-b"}, 15*time.Minute)
	if err != nil || prior != nil || b == "" || b == a {
		t.Fatalf("reserve scope-b = %q, %+v, %v", b, prior, err)
	}
	sb, prior, err := db.PostmarkLedgerReserve(ctx, PostmarkLedgerEntry{Key: "k", Server: "scope-a", Sandbox: true}, 15*time.Minute)
	if err != nil || prior != nil || sb == "" {
		t.Fatalf("reserve sandbox = %q, %+v, %v", sb, prior, err)
	}
	// A pending reservation blocks the key and reads as delivery unknown.
	_, prior, err = db.PostmarkLedgerReserve(ctx, PostmarkLedgerEntry{Key: "k", Server: "scope-a"}, 15*time.Minute)
	if err != nil || prior == nil || !prior.Pending() {
		t.Fatalf("second reserve on scope-a = %+v, %v; want the pending reservation", prior, err)
	}
	// Releasing one scope leaves the others alone; completing needs the exact ID.
	if err := db.PostmarkLedgerRelease(ctx, b); err != nil {
		t.Fatal(err)
	}
	if err := db.PostmarkLedgerComplete(ctx, b, "m-b"); err == nil {
		t.Fatal("completing a released reservation must fail")
	}
	if err := db.PostmarkLedgerComplete(ctx, a, "m-a"); err != nil {
		t.Fatalf("complete scope-a: %v", err)
	}
	got, err := db.PostmarkLedgerLookup(ctx, "k", "scope-a", true, since)
	if err != nil || got == nil || !got.Pending() {
		t.Fatalf("sandbox reservation should still be pending: %+v, %v", got, err)
	}
	if err := db.PostmarkLedgerComplete(ctx, "not-a-reservation", "m"); err == nil {
		t.Fatal("completion without a reservation ID must fail")
	}
}

// Separate Store handles on one file stand in for separate processes: each
// has its own connection pool and write mutex, so only SQLite's BEGIN
// IMMEDIATE lock serializes them.
func TestPostmarkLedgerConcurrentReserveAcrossHandles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.db")
	openLedgerTestStore(t, path) // create the schema file once
	const racers = 8
	handles := make([]*Store, racers)
	for i := range handles {
		handles[i] = openLedgerTestStore(t, path)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	won, errs := 0, 0
	start := make(chan struct{})
	for _, h := range handles {
		wg.Add(1)
		go func(h *Store) {
			defer wg.Done()
			<-start
			reservation, prior, err := h.PostmarkLedgerReserve(context.Background(), PostmarkLedgerEntry{Key: "race", Server: "scope-a"}, time.Minute)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err != nil:
				errs++
				t.Errorf("reserve: %v", err)
			case reservation != "" && prior == nil:
				won++
			}
		}(h)
	}
	close(start)
	wg.Wait()
	if won != 1 || errs != 0 {
		t.Fatalf("reservations won = %d (errors %d), want exactly 1", won, errs)
	}
}
