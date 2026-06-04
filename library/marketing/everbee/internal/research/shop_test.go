package research

import (
	"testing"
	"time"
)

func TestShopGapsFiltersUnrelatedEvidence(t *testing.T) {
	scope := ResearchScope{Kind: ScopeShop, Value: "brightbeeshop"}
	plan := ResearchPlan{Scope: scope, DataSource: DataSourceLocal, Warnings: []string{"shop snapshot stale"}}
	evidence := []EvidenceRecord{
		{ID: "s1", Resource: "shops", ShopName: "BrightBeeShop", Title: "Teacher Gift Mug", Tags: []string{"teacher"}, SearchableText: "brightbeeshop teacher gift"},
		{ID: "s2", Resource: "shops", ShopName: "OtherShop", Title: "Wedding Sign", Tags: []string{"wedding"}, SearchableText: "othershop wedding sign"},
	}

	out := BuildShopGaps(scope, evidence, plan)

	if out.Summary == "" {
		t.Fatal("summary is empty")
	}
	if len(out.NextActions) == 0 {
		t.Fatal("next_actions is empty")
	}
	if len(out.Warnings) != 1 || out.Warnings[0] != "shop snapshot stale" {
		t.Fatalf("warnings = %+v, want plan warning", out.Warnings)
	}
	assertOnlyInsightRecord(t, out, "s1")
	if out.Confidence <= 0 {
		t.Fatalf("confidence = %f, want positive", out.Confidence)
	}
}

func TestCompetitorWatchUsesLatestMatchingSnapshot(t *testing.T) {
	scope := ResearchScope{Kind: ScopeShop, Value: "brightbeeshop"}
	plan := ResearchPlan{Scope: scope, DataSource: DataSourceLocal}
	older := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	newer := older.Add(24 * time.Hour)
	snapshots := []Snapshot{
		{Scope: scope, FetchedAt: older, Evidence: []EvidenceRecord{{ID: "old", ShopName: "BrightBeeShop", SearchableText: "brightbeeshop"}}},
		{Scope: scope, FetchedAt: newer, Evidence: []EvidenceRecord{{ID: "new", ShopName: "BrightBeeShop", SearchableText: "brightbeeshop teacher gifts"}}},
		{Scope: ResearchScope{Kind: ScopeShop, Value: "othershop"}, FetchedAt: newer, Evidence: []EvidenceRecord{{ID: "other", ShopName: "OtherShop"}}},
	}

	out := BuildCompetitorWatch(scope, snapshots, plan)

	if out.Summary == "" {
		t.Fatal("summary is empty")
	}
	assertOnlyInsightRecord(t, out, "new")
}
