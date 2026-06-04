# Task 01: Research Core

**Files:**
- Create: `internal/research/types.go`
- Create: `internal/research/planner.go`
- Create: `internal/research/planner_test.go`
- Create: `internal/research/store.go`
- Create: `internal/research/store_test.go`

## Goal

Create scoped research types, freshness planning, and durable research snapshots. This task has no CLI command changes.

## Steps

- [ ] **Step 1: Write freshness planner tests**

Create `internal/research/planner_test.go` with tests for fresh local data, stale refresh, unrelated data rejection, and `--no-refresh` stale fallback.

```go
func TestPlannerUsesFreshLocalSnapshot(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	scope := ResearchScope{Kind: ScopeQuery, Value: "teacher gift"}
	snapshot := Snapshot{Scope: scope, FetchedAt: now.Add(-time.Hour)}

	plan := PlanFreshness(scope, []Snapshot{snapshot}, PlanOptions{Now: now, MaxAge: 6 * time.Hour})

	if plan.Decision != DecisionUseLocal {
		t.Fatalf("decision = %s, want %s", plan.Decision, DecisionUseLocal)
	}
}
```

Run: `go test ./internal/research -run TestPlanner -count=1`

Expected: fails because the package is not created yet.

- [ ] **Step 2: Add research types**

Create `internal/research/types.go` with these exported names:

```go
type ScopeKind string
const (
	ScopeQuery ScopeKind = "query"
	ScopeKeyword ScopeKind = "keyword"
	ScopeShop ScopeKind = "shop"
	ScopeListing ScopeKind = "listing_id"
)

type ResearchScope struct {
	Kind ScopeKind `json:"kind"`
	Value string `json:"value"`
}

type DataSource string
const (
	DataSourceLocal DataSource = "local"
	DataSourceRefreshed DataSource = "refreshed"
	DataSourceStaleLocalFallback DataSource = "stale-local-fallback"
	DataSourceNone DataSource = "none"
)

type Decision string
const (
	DecisionUseLocal Decision = "use_local"
	DecisionRefreshTargeted Decision = "refresh_targeted"
	DecisionFallbackLocal Decision = "fallback_local"
	DecisionInsufficientData Decision = "insufficient_data"
)
```

Also define `PlanOptions`, `ResearchPlan`, `Snapshot`, `Coverage`, `EvidenceRecord`, `InsightRecord`, and `ResponseEnvelope` as described in the design spec.

- [ ] **Step 3: Add planner implementation**

Create `internal/research/planner.go` with `PlanFreshness(scope ResearchScope, snapshots []Snapshot, opts PlanOptions) ResearchPlan`.

Required behavior:

- exact scope match only
- newest matching snapshot wins
- fresh snapshot returns `DecisionUseLocal`
- stale snapshot with live access returns `DecisionRefreshTargeted`
- stale snapshot without refresh returns `DecisionFallbackLocal`
- no matching snapshot returns `DecisionInsufficientData` unless live refresh is available

Run: `go test ./internal/research -run TestPlanner -count=1`

Expected: pass.

- [ ] **Step 4: Write snapshot store tests**

Create `internal/research/store_test.go`. Use `store.Open(filepath.Join(t.TempDir(), "data.db"))`, save a snapshot, then load it by matching scope.

```go
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
		Scope: scope,
		Resources: []string{"product_analytics"},
		FetchedAt: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
		FreshFor: 6 * time.Hour,
		RawRecords: []json.RawMessage{json.RawMessage(`{"id":"p1","title":"Teacher Gift Mug"}`)},
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
}
```

Run: `go test ./internal/research -run TestSnapshotStore -count=1`

Expected: fails because `NewSnapshotStore` is not created yet.

- [ ] **Step 5: Add snapshot store**

Create `internal/research/store.go` with:

- `type SnapshotStore struct { db *store.Store }`
- `func NewSnapshotStore(db *store.Store) *SnapshotStore`
- `func (s *SnapshotStore) Save(ctx context.Context, snapshot Snapshot) error`
- `func (s *SnapshotStore) List(ctx context.Context, scope ResearchScope, limit int) ([]Snapshot, error)`

The table name is `research_snapshots`. Columns:

- `id INTEGER PRIMARY KEY AUTOINCREMENT`
- `scope_kind TEXT NOT NULL`
- `scope_value TEXT NOT NULL`
- `resources TEXT NOT NULL`
- `fetched_at TEXT NOT NULL`
- `fresh_for_seconds INTEGER NOT NULL`
- `raw_records TEXT NOT NULL`
- `evidence TEXT NOT NULL`
- `coverage TEXT NOT NULL`
- `warnings TEXT NOT NULL`

Run: `go test ./internal/research -run 'TestPlanner|TestSnapshotStore' -count=1`

Expected: pass.

- [ ] **Step 6: Commit**

```bash
git add internal/research/types.go internal/research/planner.go internal/research/planner_test.go internal/research/store.go internal/research/store_test.go
git commit -m "feat: add everbee research core"
```
