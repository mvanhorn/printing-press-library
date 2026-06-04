package research

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/everbee/internal/store"
)

func TestSnapshotStoreRoundTripByScope(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	scope := ResearchScope{Kind: ScopeQuery, Value: "teacher gift"}
	researchStore := NewSnapshotStore(db)
	err = researchStore.Save(ctx, Snapshot{
		Scope:     scope,
		Resources: []string{"product_analytics"},
		FetchedAt: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
		FreshFor:  6 * time.Hour,
		RawRecords: []json.RawMessage{
			json.RawMessage(`{"id":"p1","title":"Teacher Gift Mug"}`),
		},
		Evidence: []EvidenceRecord{
			{ID: "p1", Resource: "product_analytics", Title: "Teacher Gift Mug"},
		},
		Coverage: Coverage{
			ResourceCounts:      map[string]int{"product_analytics": 1},
			RawRecordCount:      1,
			EvidenceRecordCount: 1,
		},
		Warnings: []string{"partial keyword coverage"},
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := researchStore.List(ctx, scope, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("snapshots = %d, want 1", len(got))
	}
	if got[0].Scope != scope {
		t.Fatalf("scope = %#v, want %#v", got[0].Scope, scope)
	}
	if got[0].Resources[0] != "product_analytics" {
		t.Fatalf("resources = %#v, want product_analytics", got[0].Resources)
	}
	if string(got[0].RawRecords[0]) != `{"id":"p1","title":"Teacher Gift Mug"}` {
		t.Fatalf("raw record = %s", got[0].RawRecords[0])
	}
	if got[0].Coverage.RawRecordCount != 1 {
		t.Fatalf("raw record count = %d, want 1", got[0].Coverage.RawRecordCount)
	}
	if got[0].Warnings[0] != "partial keyword coverage" {
		t.Fatalf("warnings = %#v", got[0].Warnings)
	}
}

func TestSnapshotStoreListFiltersByExactScopeAndLimit(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	scope := ResearchScope{Kind: ScopeQuery, Value: "teacher gift"}
	researchStore := NewSnapshotStore(db)
	for _, snapshot := range []Snapshot{
		{Scope: scope, FetchedAt: time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)},
		{Scope: ResearchScope{Kind: ScopeQuery, Value: "bridesmaid gift"}, FetchedAt: time.Date(2026, 6, 1, 11, 0, 0, 0, time.UTC)},
		{Scope: scope, FetchedAt: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)},
	} {
		if err := researchStore.Save(ctx, snapshot); err != nil {
			t.Fatal(err)
		}
	}

	got, err := researchStore.List(ctx, scope, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("snapshots = %d, want 1", len(got))
	}
	wantFetchedAt := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	if !got[0].FetchedAt.Equal(wantFetchedAt) {
		t.Fatalf("fetched at = %s, want %s", got[0].FetchedAt, wantFetchedAt)
	}
}
