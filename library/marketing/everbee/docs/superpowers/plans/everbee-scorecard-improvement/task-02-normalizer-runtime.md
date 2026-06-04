# Task 02: Normalizer And CLI Runtime

**Files:**
- Create: `internal/research/normalize.go`
- Create: `internal/research/normalize_test.go`
- Create: `internal/cli/everbee_research_runtime.go`
- Create: `internal/cli/everbee_research_runtime_test.go`

## Goal

Normalize raw EverBee records into typed evidence and add a CLI runtime bridge that plans freshness, fetches targeted data, stores snapshots, and returns data for engines.

## Steps

- [ ] **Step 1: Write normalizer tests**

Create `internal/research/normalize_test.go` with tests for ID preservation, tags, numeric metrics, and coverage.

```go
func TestNormalizeRecordsPreservesEvidenceFields(t *testing.T) {
	raw := []json.RawMessage{json.RawMessage(`{"listing_id":55043301,"title":"Teacher Gift Mug","tags":["teacher","gift"],"price":19.95}`)}

	evidence, coverage := NormalizeRecords("product_analytics", raw)

	if len(evidence) != 1 {
		t.Fatalf("evidence = %d, want 1", len(evidence))
	}
	if evidence[0].ID != "55043301" {
		t.Fatalf("id = %q, want 55043301", evidence[0].ID)
	}
	if coverage.RawRecords != 1 || coverage.EvidenceRecords != 1 {
		t.Fatalf("coverage = %#v, want one raw and one evidence", coverage)
	}
}
```

Run: `go test ./internal/research -run TestNormalize -count=1`

Expected: fails because `NormalizeRecords` is not created yet.

- [ ] **Step 2: Add normalizer**

Create `internal/research/normalize.go` with:

- `func NormalizeRecords(resourceType string, rawRecords []json.RawMessage) ([]EvidenceRecord, Coverage)`
- `func NoDataEnvelope(scope ResearchScope, plan ResearchPlan, nextActions []string) ResponseEnvelope`
- `func ConfidenceForCoverage(coverage Coverage) float64`

Use `store.FormatResourceID` for IDs and `store.LookupFieldValue` for snake/camel field lookup. Extract these fields:

- title/name
- shop name
- tags
- keywords
- price
- estimated sales
- estimated revenue
- rank/position
- searchable text

Run: `go test ./internal/research -run 'TestNormalize|TestConfidence' -count=1`

Expected: pass.

- [ ] **Step 3: Write runtime tests**

Create `internal/cli/everbee_research_runtime_test.go` with a fake fetcher. Test that fresh local snapshots avoid live calls and stale snapshots trigger one fetch.

```go
type fakeResearchFetcher struct {
	calls int
	snapshot research.Snapshot
	err error
}

func (f *fakeResearchFetcher) Fetch(ctx context.Context, scope research.ResearchScope, resources []string, limit int) (research.Snapshot, error) {
	f.calls++
	return f.snapshot, f.err
}
```

Run: `go test ./internal/cli -run TestResearchRuntime -count=1`

Expected: fails because the runtime is not created yet.

- [ ] **Step 4: Add runtime option and flag helpers**

Create `internal/cli/everbee_research_runtime.go` with:

```go
type researchOptions struct {
	maxAge time.Duration
	refresh bool
	noRefresh bool
	limit int
	resources []string
}

func addResearchFlags(cmd *cobra.Command, opts *researchOptions) {
	cmd.Flags().DurationVar(&opts.maxAge, "max-age", 6*time.Hour, "Maximum age for local EverBee research data")
	cmd.Flags().BoolVar(&opts.refresh, "refresh", false, "Force a targeted EverBee refresh before analysis")
	cmd.Flags().BoolVar(&opts.noRefresh, "no-refresh", false, "Use local EverBee research data only")
}
```

- [ ] **Step 5: Add runtime resolver**

Add:

- `type researchResult struct { Plan research.ResearchPlan; Snapshot research.Snapshot }`
- `type researchFetcher interface { Fetch(context.Context, research.ResearchScope, []string, int) (research.Snapshot, error) }`
- `func (r researchRuntime) resolve(ctx context.Context, scope research.ResearchScope, opts researchOptions, snapshots []research.Snapshot) (researchResult, error)`

Resolver rules:

- call `research.PlanFreshness`
- fetch only for `DecisionRefreshTargeted`
- on fetch success, return refreshed snapshot
- on fetch failure with matching stale snapshot, return fallback with warning
- on no data, return `DecisionInsufficientData`

- [ ] **Step 6: Add live targeted fetcher**

Add `liveResearchFetcher` in `internal/cli/everbee_research_runtime.go`. It maps:

- `product_analytics` to `/product_analytics/default_product_analytics`
- `keyword_research` to `/keyword_research/default_keyword_suggestion`
- `shops` to `/shops`

It sends scoped params:

- `q`, `query`, and `keyword` for query/keyword scopes
- `shop` and `shop_name` for shop scopes
- `listing_id` for listing scopes
- `per_page` from the command limit

It must store successful fetches through `research.SnapshotStore.Save`.

- [ ] **Step 7: Run focused tests**

Run:

```bash
go test ./internal/research ./internal/cli -run 'TestNormalize|TestResearchRuntime' -count=1
```

Expected: pass.

- [ ] **Step 8: Commit**

```bash
git add internal/research/normalize.go internal/research/normalize_test.go internal/cli/everbee_research_runtime.go internal/cli/everbee_research_runtime_test.go
git commit -m "feat: add everbee research runtime"
```
