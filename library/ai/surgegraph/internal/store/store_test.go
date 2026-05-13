package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func mustOpen(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestOpenMigratesIdempotently(t *testing.T) {
	st := mustOpen(t)
	if st.Path == "" {
		t.Fatal("Path should be set")
	}
	// Re-open the same path; migrations should be no-ops.
	st2, err := Open(st.Path)
	if err != nil {
		t.Fatalf("re-open: %v", err)
	}
	_ = st2.Close()
}

func TestUpsertProject(t *testing.T) {
	st := mustOpen(t)
	ctx := context.Background()
	now := time.Now().UTC()
	p := Project{
		ID:           "proj_1",
		Name:         "Acme",
		BrandName:    "Acme Inc",
		RawJSON:      `{"id":"proj_1"}`,
		LastSyncedAt: now,
	}
	if err := st.UpsertProject(ctx, p); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	// Upsert with different name should overwrite, not duplicate.
	p.Name = "Acme v2"
	if err := st.UpsertProject(ctx, p); err != nil {
		t.Fatalf("Upsert v2: %v", err)
	}
	got, err := st.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 project, got %d", len(got))
	}
	if got[0].Name != "Acme v2" {
		t.Errorf("name not updated: got %q", got[0].Name)
	}
}

func TestVisibilitySnapshotPair(t *testing.T) {
	st := mustOpen(t)
	ctx := context.Background()
	// Insert two snapshots for the same metric on different dates.
	for _, date := range []string{"2026-05-01", "2026-05-08"} {
		err := st.UpsertVisibilitySnapshot(ctx, VisibilitySnapshot{
			ProjectID: "p1", BrandName: "Acme", SnapshotDate: date, MetricType: "overview",
			Payload: json.RawMessage(`{"score":1}`),
		})
		if err != nil {
			t.Fatalf("upsert %s: %v", date, err)
		}
	}
	newer, older, err := st.LatestSnapshotPair(ctx, "p1", "Acme", "overview")
	if err != nil {
		t.Fatalf("pair: %v", err)
	}
	if newer == nil || older == nil {
		t.Fatalf("expected both snapshots; got newer=%v older=%v", newer, older)
	}
	// modernc.org/sqlite returns DATE columns with an appended "T00:00:00Z"
	// suffix; we match by prefix so the test stays decoupled from that detail.
	if !startsWith(newer.SnapshotDate, "2026-05-08") || !startsWith(older.SnapshotDate, "2026-05-01") {
		t.Errorf("ordering wrong: newer=%s older=%s", newer.SnapshotDate, older.SnapshotDate)
	}
	// Re-upsert the newer date with a different payload — should overwrite, not duplicate.
	if err := st.UpsertVisibilitySnapshot(ctx, VisibilitySnapshot{
		ProjectID: "p1", BrandName: "Acme", SnapshotDate: "2026-05-08", MetricType: "overview",
		Payload: json.RawMessage(`{"score":2}`),
	}); err != nil {
		t.Fatalf("upsert overwrite: %v", err)
	}
	newer, _, _ = st.LatestSnapshotPair(ctx, "p1", "Acme", "overview")
	if string(newer.Payload) != `{"score":2}` {
		t.Errorf("payload not overwritten: %s", newer.Payload)
	}
}

func TestFTSSearchRoundTrip(t *testing.T) {
	st := mustOpen(t)
	ctx := context.Background()
	docs := []FTSDoc{
		{Kind: "prompt", ID: "pr1", ProjectID: "p1", Title: "AI search optimization for ecommerce", Body: "How to rank in AI answer engines"},
		{Kind: "doc", ID: "d1", ProjectID: "p1", Title: "Brownie recipe", Body: "Soft fudgy brownies"},
		{Kind: "citation", ID: "c1", ProjectID: "p1", Title: "openai.com/blog/answer-engines", Body: "AI answer engines compared"},
	}
	for _, d := range docs {
		if err := st.UpsertFTS(ctx, d); err != nil {
			t.Fatalf("UpsertFTS: %v", err)
		}
	}
	// Term query.
	hits, err := st.Search(ctx, "answer", nil, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 2 {
		t.Errorf("expected 2 hits for 'answer', got %d (%+v)", len(hits), hits)
	}
	// Kind filter.
	hits, err = st.Search(ctx, "answer", []string{"prompt"}, 10)
	if err != nil {
		t.Fatalf("Search filtered: %v", err)
	}
	if len(hits) != 1 || hits[0].Kind != "prompt" {
		t.Errorf("expected exactly one prompt hit, got %+v", hits)
	}
	// Re-upsert same id should not duplicate.
	if err := st.UpsertFTS(ctx, docs[0]); err != nil {
		t.Fatalf("UpsertFTS re-insert: %v", err)
	}
	hits, _ = st.Search(ctx, "ecommerce", nil, 10)
	if len(hits) != 1 {
		t.Errorf("FTS de-dup failed: got %d", len(hits))
	}
}

func TestBumpCursorIdempotent(t *testing.T) {
	st := mustOpen(t)
	ctx := context.Background()
	if err := st.BumpCursor(ctx, "prompts", "p1", 10); err != nil {
		t.Fatal(err)
	}
	if err := st.BumpCursor(ctx, "prompts", "p1", 15); err != nil {
		t.Fatal(err)
	}
	cursors, err := st.Cursors(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(cursors) != 1 {
		t.Fatalf("want 1 cursor, got %d", len(cursors))
	}
	if cursors[0].RowCount != 15 {
		t.Errorf("row_count not updated: got %d", cursors[0].RowCount)
	}
}

func startsWith(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}
