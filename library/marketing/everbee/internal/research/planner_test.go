package research

import (
	"testing"
	"time"
)

func TestPlannerUsesFreshLocalSnapshot(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	scope := ResearchScope{Kind: ScopeQuery, Value: "teacher gift"}
	snapshot := Snapshot{Scope: scope, FetchedAt: now.Add(-time.Hour)}

	plan := PlanFreshness(scope, []Snapshot{snapshot}, PlanOptions{
		Now:    now,
		MaxAge: 6 * time.Hour,
	})

	if plan.Decision != DecisionUseLocal {
		t.Fatalf("decision = %s, want %s", plan.Decision, DecisionUseLocal)
	}
	if plan.Snapshot == nil {
		t.Fatal("snapshot = nil, want matching snapshot")
	}
}

func TestPlannerRefreshesStaleSnapshotWhenLiveAccessAllowed(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	scope := ResearchScope{Kind: ScopeQuery, Value: "teacher gift"}
	snapshot := Snapshot{Scope: scope, FetchedAt: now.Add(-8 * time.Hour)}

	plan := PlanFreshness(scope, []Snapshot{snapshot}, PlanOptions{
		Now:        now,
		MaxAge:     6 * time.Hour,
		CanRefresh: true,
	})

	if plan.Decision != DecisionRefreshTargeted {
		t.Fatalf("decision = %s, want %s", plan.Decision, DecisionRefreshTargeted)
	}
	if plan.Snapshot == nil {
		t.Fatal("snapshot = nil, want stale matching snapshot")
	}
}

func TestPlannerRejectsUnrelatedLocalSnapshots(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	scope := ResearchScope{Kind: ScopeQuery, Value: "teacher gift"}
	snapshot := Snapshot{
		Scope:     ResearchScope{Kind: ScopeQuery, Value: "bridesmaid gift"},
		FetchedAt: now.Add(-time.Hour),
	}

	plan := PlanFreshness(scope, []Snapshot{snapshot}, PlanOptions{
		Now:    now,
		MaxAge: 6 * time.Hour,
	})

	if plan.Decision != DecisionInsufficientData {
		t.Fatalf("decision = %s, want %s", plan.Decision, DecisionInsufficientData)
	}
	if plan.Snapshot != nil {
		t.Fatalf("snapshot = %#v, want nil", plan.Snapshot)
	}
}

func TestPlannerRefreshesWhenNoMatchingSnapshotAndLiveAccessAllowed(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	scope := ResearchScope{Kind: ScopeQuery, Value: "teacher gift"}
	snapshot := Snapshot{
		Scope:     ResearchScope{Kind: ScopeQuery, Value: "bridesmaid gift"},
		FetchedAt: now.Add(-time.Hour),
	}

	plan := PlanFreshness(scope, []Snapshot{snapshot}, PlanOptions{
		Now:        now,
		MaxAge:     6 * time.Hour,
		CanRefresh: true,
	})

	if plan.Decision != DecisionRefreshTargeted {
		t.Fatalf("decision = %s, want %s", plan.Decision, DecisionRefreshTargeted)
	}
	if plan.DataSource != DataSourceRefreshed {
		t.Fatalf("data source = %s, want %s", plan.DataSource, DataSourceRefreshed)
	}
	if plan.Snapshot != nil {
		t.Fatalf("snapshot = %#v, want nil", plan.Snapshot)
	}
}

func TestPlannerTreatsZeroMaxAgeAsNoExpiration(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	scope := ResearchScope{Kind: ScopeQuery, Value: "teacher gift"}
	snapshot := Snapshot{Scope: scope, FetchedAt: now.Add(-30 * 24 * time.Hour)}

	plan := PlanFreshness(scope, []Snapshot{snapshot}, PlanOptions{Now: now})

	if plan.Decision != DecisionUseLocal {
		t.Fatalf("decision = %s, want %s", plan.Decision, DecisionUseLocal)
	}
	if !plan.Freshness.Fresh {
		t.Fatal("freshness = stale, want fresh with zero max age")
	}
	if plan.Freshness.MaxAgeSeconds != 0 {
		t.Fatalf("max age seconds = %d, want 0", plan.Freshness.MaxAgeSeconds)
	}
}

func TestPlannerFallsBackToStaleSnapshotWhenRefreshDisabled(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	scope := ResearchScope{Kind: ScopeQuery, Value: "teacher gift"}
	snapshot := Snapshot{Scope: scope, FetchedAt: now.Add(-8 * time.Hour)}

	plan := PlanFreshness(scope, []Snapshot{snapshot}, PlanOptions{
		Now:       now,
		MaxAge:    6 * time.Hour,
		NoRefresh: true,
	})

	if plan.Decision != DecisionFallbackLocal {
		t.Fatalf("decision = %s, want %s", plan.Decision, DecisionFallbackLocal)
	}
	if plan.DataSource != DataSourceStaleLocalFallback {
		t.Fatalf("data source = %s, want %s", plan.DataSource, DataSourceStaleLocalFallback)
	}
}
