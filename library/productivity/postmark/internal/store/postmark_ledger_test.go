// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestPostmarkLedgerRoundTrip(t *testing.T) {
	ctx := context.Background()
	db, err := OpenWithContext(ctx, filepath.Join(t.TempDir(), "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Date(2026, 9, 30, 17, 0, 0, 0, time.UTC)
	miss, err := db.PostmarkLedgerLookup(ctx, "otp-4821", "Main App", false, now.Add(-15*time.Minute))
	if err != nil || miss != nil {
		t.Fatalf("empty ledger lookup = %+v, %v", miss, err)
	}
	entry := PostmarkLedgerEntry{Key: "otp-4821", MessageID: "m-1", Recipient: "jane@example.com", Server: "Main App", Stream: "outbound", SentAt: now.Add(-5 * time.Minute)}
	if err := db.PostmarkLedgerRecord(ctx, entry); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		key     string
		server  string
		sandbox bool
		since   time.Time
		want    string
	}{
		{"hit inside window", "otp-4821", "Main App", false, now.Add(-15 * time.Minute), "m-1"},
		{"outside window", "otp-4821", "Main App", false, now.Add(-1 * time.Minute), ""},
		{"other key", "otp-9999", "Main App", false, now.Add(-15 * time.Minute), ""},
		{"other server", "otp-4821", "Staging", false, now.Add(-15 * time.Minute), ""},
		{"sandbox does not match live", "otp-4821", "Main App", true, now.Add(-15 * time.Minute), ""},
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

	newer := entry
	newer.MessageID, newer.SentAt = "m-2", now.Add(-time.Minute)
	if err := db.PostmarkLedgerRecord(ctx, newer); err != nil {
		t.Fatal(err)
	}
	got, err := db.PostmarkLedgerLookup(ctx, "otp-4821", "Main App", false, now.Add(-15*time.Minute))
	if err != nil || got == nil || got.MessageID != "m-2" || !got.SentAt.Equal(newer.SentAt) {
		t.Fatalf("newest entry = %+v, %v", got, err)
	}
	if err := db.PostmarkLedgerRecord(ctx, PostmarkLedgerEntry{Key: "k"}); err == nil {
		t.Fatal("entry without a message ID must be rejected")
	}
}
