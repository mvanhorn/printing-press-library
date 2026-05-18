// Copyright 2026 david. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/slickdeals/internal/store"
)

// seedSnapshots inserts canned deal_snapshots rows for digest tests via the
// canonical store.InsertSnapshot API. That call manages schema creation, so
// the test path mirrors the production path exactly.
func seedSnapshots(t *testing.T, db *store.Store, rows []digestSnapshot) {
	t.Helper()
	for _, r := range rows {
		raw, _ := json.Marshal(r)
		snap := &store.DealSnapshot{
			DealID:     r.DealID,
			CapturedAt: r.CapturedAt,
			Thumbs:     r.Thumbs,
			Merchant:   r.Merchant,
			Category:   r.Category,
			Title:      r.Title,
			Raw:        string(raw),
		}
		if err := db.InsertSnapshot(snap); err != nil {
			t.Fatalf("inserting snapshot %s: %v", r.DealID, err)
		}
	}
}

// queryDigestSnapshotsCompat is the test-side shim that mirrors the
// pre-integration helper name. It calls the canonical store API and projects
// the rows through projectSnapshots so existing tests keep their shape.
func queryDigestSnapshotsCompat(db *store.Store, cutoff time.Time) ([]digestSnapshot, error) {
	rows, err := db.QuerySnapshotsSince(cutoff, 0)
	if err != nil {
		return nil, err
	}
	return projectSnapshots(rows), nil
}

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestQueryDigestSnapshots_EmptyStore(t *testing.T) {
	db := openTestStore(t)

	snaps, err := queryDigestSnapshotsCompat(db,time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("query on empty store should succeed, got %v", err)
	}
	if len(snaps) != 0 {
		t.Errorf("want 0 snapshots, got %d", len(snaps))
	}
}

func TestQueryDigestSnapshots_TopByThumbs(t *testing.T) {
	db := openTestStore(t)
	now := time.Now().UTC()
	seedSnapshots(t, db, []digestSnapshot{
		{DealID: "1", CapturedAt: now.Add(-1 * time.Hour), Thumbs: 100, Merchant: "acme", Category: "tech"},
		{DealID: "2", CapturedAt: now.Add(-2 * time.Hour), Thumbs: 50, Merchant: "bigstore", Category: "home"},
		{DealID: "3", CapturedAt: now.Add(-3 * time.Hour), Thumbs: 200, Merchant: "acme", Category: "tech"},
		{DealID: "4", CapturedAt: now.Add(-4 * time.Hour), Thumbs: 25, Merchant: "thirdparty", Category: "tech"},
	})

	snaps, err := queryDigestSnapshotsCompat(db,now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snaps) != 4 {
		t.Fatalf("want 4 snapshots, got %d", len(snaps))
	}
	// Sort order: thumbs DESC.
	wantOrder := []string{"3", "1", "2", "4"}
	for i, s := range snaps {
		if s.DealID != wantOrder[i] {
			t.Errorf("position %d: want DealID %s, got %s", i, wantOrder[i], s.DealID)
		}
	}
}

func TestQueryDigestSnapshots_SinceCutoff(t *testing.T) {
	db := openTestStore(t)
	now := time.Now().UTC()
	seedSnapshots(t, db, []digestSnapshot{
		{DealID: "recent", CapturedAt: now.Add(-1 * time.Hour), Thumbs: 10},
		{DealID: "stale", CapturedAt: now.Add(-48 * time.Hour), Thumbs: 999},
	})

	snaps, err := queryDigestSnapshotsCompat(db,now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("want 1 snapshot inside cutoff window, got %d", len(snaps))
	}
	if snaps[0].DealID != "recent" {
		t.Errorf("want stale deal excluded; got DealID %q", snaps[0].DealID)
	}
}

func TestQueryDigestSnapshots_LatestPerDeal(t *testing.T) {
	db := openTestStore(t)
	now := time.Now().UTC()
	seedSnapshots(t, db, []digestSnapshot{
		{DealID: "1", CapturedAt: now.Add(-3 * time.Hour), Thumbs: 50, Title: "old"},
		{DealID: "1", CapturedAt: now.Add(-1 * time.Hour), Thumbs: 80, Title: "new"},
		{DealID: "2", CapturedAt: now.Add(-2 * time.Hour), Thumbs: 30, Title: "only"},
	})

	snaps, err := queryDigestSnapshotsCompat(db,now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snaps) != 2 {
		t.Fatalf("want 2 deduplicated snapshots, got %d", len(snaps))
	}
	// Deal 1 must be the latest row (thumbs=80, title=new).
	for _, s := range snaps {
		if s.DealID == "1" && (s.Thumbs != 80 || s.Title != "new") {
			t.Errorf("deal 1: want latest (thumbs=80,title=new), got thumbs=%d title=%q", s.Thumbs, s.Title)
		}
	}
}

func TestApplyMerchantCap(t *testing.T) {
	input := []digestSnapshot{
		{DealID: "1", Merchant: "acme", Thumbs: 100},
		{DealID: "2", Merchant: "acme", Thumbs: 90},
		{DealID: "3", Merchant: "acme", Thumbs: 80},
		{DealID: "4", Merchant: "bigstore", Thumbs: 70},
		{DealID: "5", Merchant: "", Thumbs: 60}, // empty merchant: never capped
	}
	got := applyMerchantCap(input, 2)
	if len(got) != 4 {
		t.Fatalf("want 4 results after cap, got %d", len(got))
	}
	// Deal 3 (third acme) should have been dropped.
	for _, s := range got {
		if s.DealID == "3" {
			t.Errorf("third acme deal should have been capped out, got %+v", s)
		}
	}
}

func TestApplyMerchantCap_Disabled(t *testing.T) {
	input := []digestSnapshot{
		{DealID: "1", Merchant: "acme"},
		{DealID: "2", Merchant: "acme"},
	}
	got := applyMerchantCap(input, 0)
	if len(got) != 2 {
		t.Errorf("cap=0 should pass everything through, got %d/%d", len(got), len(input))
	}
}

func TestGroupSnapshots(t *testing.T) {
	in := []digestSnapshot{
		{DealID: "1", Merchant: "acme", Category: "tech"},
		{DealID: "2", Merchant: "acme", Category: "home"},
		{DealID: "3", Merchant: "bigstore", Category: "tech"},
	}
	byMerchant := groupSnapshots(in, "merchant")
	if len(byMerchant["acme"]) != 2 {
		t.Errorf("want 2 acme deals, got %d", len(byMerchant["acme"]))
	}
	if len(byMerchant["bigstore"]) != 1 {
		t.Errorf("want 1 bigstore deal, got %d", len(byMerchant["bigstore"]))
	}

	byCategory := groupSnapshots(in, "category")
	if len(byCategory["tech"]) != 2 {
		t.Errorf("want 2 tech deals, got %d", len(byCategory["tech"]))
	}
}

func TestGroupSnapshots_EmptyKeyBecomesUnspecified(t *testing.T) {
	in := []digestSnapshot{{DealID: "1", Merchant: ""}}
	got := groupSnapshots(in, "merchant")
	if len(got["(unspecified)"]) != 1 {
		t.Errorf("expected empty-merchant rows under (unspecified), got %+v", got)
	}
}

func TestNewDigestCmd_EmptyStoreHint(t *testing.T) {
	dir := t.TempDir()
	flags := &rootFlags{asJSON: true}
	cmd := newDigestCmd(flags)
	cmd.SetArgs([]string{"--since", "24h", "--db", filepath.Join(dir, "empty.db")})

	var stdout, stderr strings.Builder
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("digest on empty store should return cleanly, got %v", err)
	}
	if !strings.Contains(stderr.String(), "No snapshots in the local store yet") {
		t.Errorf("expected empty-store hint in stderr, got %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), `"meta"`) || !strings.Contains(stdout.String(), `"local"`) {
		t.Errorf("expected provenance envelope on stdout, got %q", stdout.String())
	}
}

func TestNewDigestCmd_InvalidGroupedBy(t *testing.T) {
	flags := &rootFlags{}
	cmd := newDigestCmd(flags)
	cmd.SetArgs([]string{"--grouped-by", "bogus"})
	cmd.SetOut(discardWriter{})
	cmd.SetErr(discardWriter{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid --grouped-by, got nil")
	}
	if !strings.Contains(err.Error(), "grouped-by") {
		t.Errorf("expected message about grouped-by, got %q", err.Error())
	}
}

func TestNewDigestCmd_DryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newDigestCmd(flags)
	cmd.SetArgs([]string{"--since", "24h"})
	cmd.SetOut(discardWriter{})
	cmd.SetErr(discardWriter{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run should short-circuit, got %v", err)
	}
}
