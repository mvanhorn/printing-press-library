package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func seed(t *testing.T, s *Store) {
	t.Helper()
	ctx := context.Background()
	now := time.Now()
	rows := []struct {
		typ, id, name, data string
	}{
		{"advertisers", "1", "Acme Co", `{"id":"1","name":"Acme Co"}`},
		{"advertisers", "2", "Globex", `{"id":"2","name":"Globex"}`},
		{"campaigns", "10", "Acme Spring Push", `{"id":"10","name":"Acme Spring Push"}`},
	}
	for _, r := range rows {
		if err := s.Upsert(ctx, r.typ, r.id, r.name, json.RawMessage(r.data), now); err != nil {
			t.Fatalf("Upsert %s/%s: %v", r.typ, r.id, err)
		}
	}
}

func TestUpsertAndCount(t *testing.T) {
	s := newTestStore(t)
	seed(t, s)
	ctx := context.Background()

	cases := []struct {
		typ  string
		want int
	}{
		{"advertisers", 2},
		{"campaigns", 1},
		{"segments", 0},
	}
	for _, c := range cases {
		got, err := s.Count(ctx, c.typ)
		if err != nil {
			t.Fatalf("Count(%s): %v", c.typ, err)
		}
		if got != c.want {
			t.Errorf("Count(%s) = %d, want %d", c.typ, got, c.want)
		}
	}

	total, err := s.Total(ctx)
	if err != nil {
		t.Fatalf("Total: %v", err)
	}
	if total != 3 {
		t.Errorf("Total = %d, want 3", total)
	}
}

func TestUpsertReplaces(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now()
	if err := s.Upsert(ctx, "advertisers", "1", "Old", json.RawMessage(`{"id":"1","name":"Old"}`), now); err != nil {
		t.Fatal(err)
	}
	if err := s.Upsert(ctx, "advertisers", "1", "New", json.RawMessage(`{"id":"1","name":"New"}`), now); err != nil {
		t.Fatal(err)
	}
	got, err := s.Count(ctx, "advertisers")
	if err != nil {
		t.Fatal(err)
	}
	if got != 1 {
		t.Errorf("Count after replace = %d, want 1 (upsert must not duplicate)", got)
	}
	hits, err := s.Search(ctx, "New", "advertisers", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Name != "New" {
		t.Errorf("Search after replace = %+v, want single hit named New", hits)
	}
}

func TestSearch(t *testing.T) {
	s := newTestStore(t)
	seed(t, s)
	ctx := context.Background()

	cases := []struct {
		name    string
		term    string
		typ     string
		wantLen int
	}{
		{"name match across types", "acme", "", 2},
		{"name match scoped to type", "acme", "campaigns", 1},
		{"no match", "nonexistent", "", 0},
		{"data field match", "Globex", "", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hits, err := s.Search(ctx, c.term, c.typ, 50)
			if err != nil {
				t.Fatalf("Search: %v", err)
			}
			if len(hits) != c.wantLen {
				t.Errorf("Search(%q, %q) returned %d hits, want %d", c.term, c.typ, len(hits), c.wantLen)
			}
		})
	}
}

func TestListAndTypes(t *testing.T) {
	s := newTestStore(t)
	seed(t, s)
	ctx := context.Background()

	items, err := s.List(ctx, "advertisers", 100)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("List(advertisers) = %d items, want 2", len(items))
	}

	types, err := s.Types(ctx)
	if err != nil {
		t.Fatalf("Types: %v", err)
	}
	if types["advertisers"] != 2 || types["campaigns"] != 1 {
		t.Errorf("Types = %v, want advertisers:2 campaigns:1", types)
	}
}

func TestSaveSyncState(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.SaveSyncState(ctx, "advertisers", 27, time.Now()); err != nil {
		t.Fatalf("SaveSyncState: %v", err)
	}
	var count int
	err := s.DB().QueryRowContext(ctx, `SELECT row_count FROM sync_state WHERE resource_type = ?`, "advertisers").Scan(&count)
	if err != nil {
		t.Fatalf("reading sync_state: %v", err)
	}
	if count != 27 {
		t.Errorf("sync_state row_count = %d, want 27", count)
	}
}

func TestDefaultPathEnvOverride(t *testing.T) {
	t.Setenv("STACKADAPT_DB", "/tmp/custom-stackadapt.db")
	if got := DefaultPath(); got != "/tmp/custom-stackadapt.db" {
		t.Errorf("DefaultPath with env = %q, want /tmp/custom-stackadapt.db", got)
	}
}
