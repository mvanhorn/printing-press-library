package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestStoreLifecycle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store.db")
	ctx := context.Background()

	s, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	// Upsert object + attribute + option roundtrip.
	if err := s.UpsertObject(ctx, Object{APIName: "company", Provider: "UNIFY", Category: "STANDARD", DisplayName: "Company", Raw: map[string]any{"api_name": "company"}}); err != nil {
		t.Fatalf("UpsertObject: %v", err)
	}
	objs, err := s.ListObjects(ctx)
	if err != nil {
		t.Fatalf("ListObjects: %v", err)
	}
	if len(objs) != 1 || objs[0].APIName != "company" {
		t.Fatalf("expected one object 'company', got %+v", objs)
	}

	if err := s.UpsertAttribute(ctx, Attribute{ObjectName: "company", APIName: "domain", Type: "URL", IsUnique: true, Raw: map[string]any{"api_name": "domain"}}); err != nil {
		t.Fatalf("UpsertAttribute: %v", err)
	}
	attrs, err := s.ListAttributes(ctx, "company")
	if err != nil || len(attrs) != 1 || !attrs[0].IsUnique {
		t.Fatalf("attrs roundtrip: %+v err=%v", attrs, err)
	}

	// Upsert a record + FTS lookup.
	if err := s.UpsertRecord(ctx, "company", "id-1", "2026-01-01T00:00:00Z", "2026-05-11T00:00:00Z", map[string]any{
		"name":   "Acme Corp",
		"domain": "acme.com",
	}); err != nil {
		t.Fatalf("UpsertRecord: %v", err)
	}
	n, err := s.CountRecords(ctx, "company")
	if err != nil || n != 1 {
		t.Fatalf("CountRecords: n=%d err=%v", n, err)
	}

	rows, err := s.DB.QueryContext(ctx, `SELECT object_name, id FROM records_fts WHERE body MATCH 'acme'`)
	if err != nil {
		t.Fatalf("FTS query: %v", err)
	}
	defer rows.Close()
	found := 0
	for rows.Next() {
		var obj, id string
		_ = rows.Scan(&obj, &id)
		if obj == "company" && id == "id-1" {
			found++
		}
	}
	if found != 1 {
		t.Fatalf("expected one FTS hit, got %d", found)
	}

	// Watchlist roundtrip.
	if err := s.AddWatch(ctx, WatchEntry{ObjectName: "company", MatchKey: "domain", MatchValue: "acme.com"}); err != nil {
		t.Fatalf("AddWatch: %v", err)
	}
	wl, err := s.ListWatch(ctx, "")
	if err != nil || len(wl) != 1 {
		t.Fatalf("ListWatch: %+v err=%v", wl, err)
	}
	removed, err := s.RemoveWatch(ctx, "company", "domain", "acme.com")
	if err != nil || removed != 1 {
		t.Fatalf("RemoveWatch: removed=%d err=%v", removed, err)
	}

	// Snapshot.
	id, err := s.Snapshot(ctx, "test")
	if err != nil || id <= 0 {
		t.Fatalf("Snapshot: id=%d err=%v", id, err)
	}
	snaps, err := s.LatestSnapshots(ctx, 5)
	if err != nil || len(snaps) != 1 {
		t.Fatalf("LatestSnapshots: %+v err=%v", snaps, err)
	}
}

func TestRecordTableName(t *testing.T) {
	cases := map[string]string{
		"company":            "record_company",
		"salesforce_account": "record_salesforce_account",
		"weird-name":         "record_weird_name",
		"WithCaps":           "record_withcaps",
	}
	for in, want := range cases {
		got := RecordTable(in)
		if got != want {
			t.Errorf("RecordTable(%q): got %q want %q", in, got, want)
		}
	}
}
