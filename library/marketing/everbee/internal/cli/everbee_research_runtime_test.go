package cli

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/everbee/internal/research"
)

type fakeResearchFetcher struct {
	calls    int
	snapshot research.Snapshot
	err      error
}

func (f *fakeResearchFetcher) Fetch(ctx context.Context, scope research.ResearchScope, resources []string, limit int) (research.Snapshot, error) {
	f.calls++
	return f.snapshot, f.err
}

func TestResearchRuntimeUsesFreshLocalSnapshot(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	scope := research.ResearchScope{Kind: research.ScopeQuery, Value: "teacher gift"}
	local := research.Snapshot{
		Scope:     scope,
		Resources: []string{"product_analytics"},
		FetchedAt: now.Add(-time.Hour),
		RawRecords: []json.RawMessage{
			json.RawMessage(`{"listing_id":55043301,"title":"Teacher Gift Mug"}`),
		},
		Evidence: []research.EvidenceRecord{{ID: "55043301", Title: "Teacher Gift Mug"}},
	}
	fetcher := &fakeResearchFetcher{}
	runtime := researchRuntime{fetcher: fetcher, now: func() time.Time { return now }}

	result, err := runtime.resolve(context.Background(), scope, researchOptions{
		maxAge:    6 * time.Hour,
		resources: []string{"product_analytics"},
		limit:     25,
	}, []research.Snapshot{local})
	if err != nil {
		t.Fatalf("resolve error = %v", err)
	}
	if fetcher.calls != 0 {
		t.Fatalf("fetch calls = %d, want 0", fetcher.calls)
	}
	if result.Plan.Decision != research.DecisionUseLocal {
		t.Fatalf("decision = %s, want use_local", result.Plan.Decision)
	}
	if result.Plan.DataSource != research.DataSourceLocal {
		t.Fatalf("data source = %s, want local", result.Plan.DataSource)
	}
	if len(result.Snapshot.RawRecords) != 1 {
		t.Fatalf("raw records = %d, want 1", len(result.Snapshot.RawRecords))
	}
}

func TestResearchRuntimeRefreshesInsteadOfUsingFreshEmptySnapshot(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	scope := research.ResearchScope{Kind: research.ScopeQuery, Value: "teacher gift"}
	emptyLocal := research.Snapshot{
		Scope:     scope,
		Resources: []string{"product_analytics"},
		FetchedAt: now.Add(-time.Hour),
	}
	refreshed := research.Snapshot{
		Scope:     scope,
		Resources: []string{"product_analytics"},
		FetchedAt: now,
		Evidence:  []research.EvidenceRecord{{ID: "55043301", Title: "Teacher Gift Mug"}},
		Coverage: research.Coverage{
			ResourceCounts:      map[string]int{"product_analytics": 1},
			RawRecordCount:      1,
			EvidenceRecordCount: 1,
		},
	}
	fetcher := &fakeResearchFetcher{snapshot: refreshed}
	runtime := researchRuntime{fetcher: fetcher, now: func() time.Time { return now }}

	result, err := runtime.resolve(context.Background(), scope, researchOptions{
		maxAge:    6 * time.Hour,
		resources: []string{"product_analytics"},
		limit:     25,
	}, []research.Snapshot{emptyLocal})
	if err != nil {
		t.Fatalf("resolve error = %v", err)
	}
	if fetcher.calls != 1 {
		t.Fatalf("fetch calls = %d, want 1", fetcher.calls)
	}
	if result.Plan.Decision != research.DecisionRefreshTargeted {
		t.Fatalf("decision = %s, want refresh_targeted", result.Plan.Decision)
	}
	if result.Plan.DataSource != research.DataSourceRefreshed {
		t.Fatalf("data source = %s, want refreshed", result.Plan.DataSource)
	}
}

func TestResearchRuntimeRefreshesStaleSnapshotOnce(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	scope := research.ResearchScope{Kind: research.ScopeKeyword, Value: "teacher mug"}
	stale := research.Snapshot{
		Scope:     scope,
		Resources: []string{"keyword_research"},
		FetchedAt: now.Add(-24 * time.Hour),
	}
	refreshed := research.Snapshot{
		Scope:     scope,
		Resources: []string{"keyword_research"},
		FetchedAt: now,
		RawRecords: []json.RawMessage{
			json.RawMessage(`{"id":"kw-1","keyword":"teacher mug"}`),
		},
		Evidence: []research.EvidenceRecord{{ID: "kw-1", Keywords: []string{"teacher mug"}}},
		Coverage: research.Coverage{
			ResourceCounts:      map[string]int{"keyword_research": 1},
			RawRecordCount:      1,
			EvidenceRecordCount: 1,
		},
	}
	fetcher := &fakeResearchFetcher{snapshot: refreshed}
	runtime := researchRuntime{fetcher: fetcher, now: func() time.Time { return now }}

	result, err := runtime.resolve(context.Background(), scope, researchOptions{
		maxAge:    6 * time.Hour,
		resources: []string{"keyword_research"},
		limit:     10,
	}, []research.Snapshot{stale})
	if err != nil {
		t.Fatalf("resolve error = %v", err)
	}
	if fetcher.calls != 1 {
		t.Fatalf("fetch calls = %d, want 1", fetcher.calls)
	}
	if result.Plan.Decision != research.DecisionRefreshTargeted {
		t.Fatalf("decision = %s, want refresh_targeted", result.Plan.Decision)
	}
	if result.Plan.DataSource != research.DataSourceRefreshed {
		t.Fatalf("data source = %s, want refreshed", result.Plan.DataSource)
	}
	if result.Snapshot.FetchedAt != now {
		t.Fatalf("snapshot fetched at = %s, want %s", result.Snapshot.FetchedAt, now)
	}
}

func TestResearchRuntimeFallsBackToStaleSnapshotAfterRefreshFailure(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	scope := research.ResearchScope{Kind: research.ScopeShop, Value: "gift-shop"}
	stale := research.Snapshot{
		Scope:     scope,
		Resources: []string{"shops"},
		FetchedAt: now.Add(-24 * time.Hour),
		RawRecords: []json.RawMessage{
			json.RawMessage(`{"id":"shop-1","name":"Gift Shop"}`),
		},
		Evidence: []research.EvidenceRecord{{ID: "shop-1", Title: "Gift Shop"}},
	}
	fetcher := &fakeResearchFetcher{err: errors.New("network unavailable")}
	runtime := researchRuntime{fetcher: fetcher, now: func() time.Time { return now }}

	result, err := runtime.resolve(context.Background(), scope, researchOptions{
		maxAge:    6 * time.Hour,
		resources: []string{"shops"},
		limit:     10,
	}, []research.Snapshot{stale})
	if err != nil {
		t.Fatalf("resolve error = %v", err)
	}
	if fetcher.calls != 1 {
		t.Fatalf("fetch calls = %d, want 1", fetcher.calls)
	}
	if result.Plan.Decision != research.DecisionFallbackLocal {
		t.Fatalf("decision = %s, want fallback_local", result.Plan.Decision)
	}
	if result.Plan.DataSource != research.DataSourceStaleLocalFallback {
		t.Fatalf("data source = %s, want stale-local-fallback", result.Plan.DataSource)
	}
	if len(result.Plan.Warnings) == 0 {
		t.Fatalf("warnings = %#v, want refresh failure warning", result.Plan.Warnings)
	}
}

func TestResearchRuntimeUsesFreshSnapshotWhenPersistenceFails(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	scope := research.ResearchScope{Kind: research.ScopeShop, Value: "gift-shop"}
	refreshed := research.Snapshot{
		Scope:     scope,
		Resources: []string{"shops"},
		FetchedAt: now,
		RawRecords: []json.RawMessage{
			json.RawMessage(`{"id":"shop-2","name":"Fresh Shop"}`),
		},
		Evidence: []research.EvidenceRecord{{ID: "shop-2", Title: "Fresh Shop"}},
	}
	fetcher := &fakeResearchFetcher{
		snapshot: refreshed,
		err:      errors.New("save denied"),
	}
	runtime := researchRuntime{fetcher: fetcher, now: func() time.Time { return now }}

	result, err := runtime.resolve(context.Background(), scope, researchOptions{
		maxAge:    6 * time.Hour,
		resources: []string{"shops"},
		limit:     10,
	}, nil)
	if err != nil {
		t.Fatalf("resolve error = %v", err)
	}
	if result.Plan.Decision != research.DecisionRefreshTargeted {
		t.Fatalf("decision = %s, want refresh_targeted", result.Plan.Decision)
	}
	if result.Plan.DataSource != research.DataSourceRefreshed {
		t.Fatalf("data source = %s, want refreshed", result.Plan.DataSource)
	}
	if len(result.Snapshot.Evidence) != 1 || result.Snapshot.Evidence[0].ID != "shop-2" {
		t.Fatalf("snapshot evidence = %#v, want fresh fetched evidence", result.Snapshot.Evidence)
	}
	if !warningsContain(result.Plan.Warnings, "persistence failed") {
		t.Fatalf("warnings = %#v, want persistence warning", result.Plan.Warnings)
	}
}

func TestResearchRuntimeTreatsEmptyRefreshAsInsufficientData(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	scope := research.ResearchScope{Kind: research.ScopeQuery, Value: "teacher gift"}
	fetcher := &fakeResearchFetcher{snapshot: research.Snapshot{
		Scope:     scope,
		Resources: []string{"product_analytics"},
		FetchedAt: now,
	}}
	runtime := researchRuntime{fetcher: fetcher, now: func() time.Time { return now }}

	result, err := runtime.resolve(context.Background(), scope, researchOptions{
		maxAge:    6 * time.Hour,
		resources: []string{"product_analytics"},
		limit:     25,
	}, nil)
	if err != nil {
		t.Fatalf("resolve error = %v", err)
	}
	if result.Plan.Decision != research.DecisionInsufficientData {
		t.Fatalf("decision = %s, want insufficient_data", result.Plan.Decision)
	}
	if result.Plan.DataSource != research.DataSourceNone {
		t.Fatalf("data source = %s, want none", result.Plan.DataSource)
	}
	if len(result.Plan.Warnings) == 0 {
		t.Fatalf("warnings = %#v, want empty refresh warning", result.Plan.Warnings)
	}
}

func warningsContain(warnings []string, needle string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, needle) {
			return true
		}
	}
	return false
}

func TestResearchRuntimeFallsBackWhenRefreshReturnsNoEvidence(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	scope := research.ResearchScope{Kind: research.ScopeQuery, Value: "teacher gift"}
	stale := research.Snapshot{
		Scope:     scope,
		Resources: []string{"product_analytics"},
		FetchedAt: now.Add(-24 * time.Hour),
		Evidence:  []research.EvidenceRecord{{ID: "55043301", Title: "Teacher Gift Mug"}},
	}
	fetcher := &fakeResearchFetcher{snapshot: research.Snapshot{
		Scope:     scope,
		Resources: []string{"product_analytics"},
		FetchedAt: now,
		RawRecords: []json.RawMessage{
			json.RawMessage(`{"total":0}`),
		},
	}}
	runtime := researchRuntime{fetcher: fetcher, now: func() time.Time { return now }}

	result, err := runtime.resolve(context.Background(), scope, researchOptions{
		maxAge:    6 * time.Hour,
		resources: []string{"product_analytics"},
		limit:     25,
	}, []research.Snapshot{stale})
	if err != nil {
		t.Fatalf("resolve error = %v", err)
	}
	if result.Plan.Decision != research.DecisionFallbackLocal {
		t.Fatalf("decision = %s, want fallback_local", result.Plan.Decision)
	}
	if result.Plan.DataSource != research.DataSourceStaleLocalFallback {
		t.Fatalf("data source = %s, want stale-local-fallback", result.Plan.DataSource)
	}
	if result.Snapshot.FetchedAt != stale.FetchedAt {
		t.Fatalf("snapshot fetched at = %s, want stale %s", result.Snapshot.FetchedAt, stale.FetchedAt)
	}
}

func TestResearchRuntimeDoesNotFallbackToStaleEmptySnapshot(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	scope := research.ResearchScope{Kind: research.ScopeShop, Value: "gift-shop"}
	emptyStale := research.Snapshot{
		Scope:     scope,
		Resources: []string{"shops"},
		FetchedAt: now.Add(-24 * time.Hour),
	}
	fetcher := &fakeResearchFetcher{err: errors.New("network unavailable")}
	runtime := researchRuntime{fetcher: fetcher, now: func() time.Time { return now }}

	result, err := runtime.resolve(context.Background(), scope, researchOptions{
		maxAge:    6 * time.Hour,
		resources: []string{"shops"},
		limit:     10,
	}, []research.Snapshot{emptyStale})
	if err != nil {
		t.Fatalf("resolve error = %v", err)
	}
	if result.Plan.Decision != research.DecisionInsufficientData {
		t.Fatalf("decision = %s, want insufficient_data", result.Plan.Decision)
	}
	if result.Plan.DataSource != research.DataSourceNone {
		t.Fatalf("data source = %s, want none", result.Plan.DataSource)
	}
}

func TestResearchRuntimeExposesPartialRefreshWarningsOnPlan(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	scope := research.ResearchScope{Kind: research.ScopeQuery, Value: "teacher gift"}
	fetcher := liveResearchFetcher{
		maxAge: 6 * time.Hour,
		now:    func() time.Time { return now },
		get: func(ctx context.Context, path string, params map[string]string) (json.RawMessage, error) {
			switch path {
			case "/product_analytics/default_product_analytics":
				return json.RawMessage(`[{"listing_id":55043301,"title":"Teacher Gift Mug"}]`), nil
			case "/keyword_research/default_keyword_suggestion":
				return nil, errors.New("keyword endpoint unavailable")
			default:
				return nil, errors.New("unexpected path")
			}
		},
	}
	runtime := researchRuntime{fetcher: fetcher, now: func() time.Time { return now }}

	result, err := runtime.resolve(context.Background(), scope, researchOptions{
		maxAge:    6 * time.Hour,
		resources: []string{"product_analytics", "keyword_research"},
		limit:     25,
	}, nil)
	if err != nil {
		t.Fatalf("resolve error = %v", err)
	}
	if result.Plan.Decision != research.DecisionRefreshTargeted {
		t.Fatalf("decision = %s, want refresh_targeted", result.Plan.Decision)
	}
	if result.Plan.DataSource != research.DataSourceRefreshed {
		t.Fatalf("data source = %s, want refreshed", result.Plan.DataSource)
	}
	if len(result.Plan.Warnings) == 0 {
		t.Fatalf("plan warnings = %#v, want partial fetch warning", result.Plan.Warnings)
	}
}

func TestResearchRuntimeLiveFetcherKeepsPartialDataWhenOneResourceFails(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	scope := research.ResearchScope{Kind: research.ScopeQuery, Value: "teacher gift"}
	calls := 0
	fetcher := liveResearchFetcher{
		maxAge: 6 * time.Hour,
		now:    func() time.Time { return now },
		get: func(ctx context.Context, path string, params map[string]string) (json.RawMessage, error) {
			calls++
			switch path {
			case "/product_analytics/default_product_analytics":
				return json.RawMessage(`[{"listing_id":55043301,"title":"Teacher Gift Mug"}]`), nil
			case "/keyword_research/default_keyword_suggestion":
				return nil, errors.New("keyword endpoint unavailable")
			default:
				return nil, errors.New("unexpected path")
			}
		},
	}

	snapshot, err := fetcher.Fetch(context.Background(), scope, []string{"product_analytics", "keyword_research"}, 25)
	if err != nil {
		t.Fatalf("fetch error = %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
	if len(snapshot.Evidence) != 1 {
		t.Fatalf("evidence = %#v, want one product evidence record", snapshot.Evidence)
	}
	if snapshot.Coverage.ResourceCounts["product_analytics"] != 1 {
		t.Fatalf("product coverage = %d, want 1", snapshot.Coverage.ResourceCounts["product_analytics"])
	}
	if snapshot.Coverage.ResourceCounts["keyword_research"] != 0 {
		t.Fatalf("keyword coverage = %d, want 0", snapshot.Coverage.ResourceCounts["keyword_research"])
	}
	if len(snapshot.Warnings) == 0 {
		t.Fatalf("warnings = %#v, want partial fetch warning", snapshot.Warnings)
	}
}
