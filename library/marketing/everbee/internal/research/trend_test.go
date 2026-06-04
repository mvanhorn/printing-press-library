package research

import (
	"testing"
	"time"
)

func TestTrendsDiffComparesTwoMatchingSnapshots(t *testing.T) {
	scope := ResearchScope{Kind: ScopeQuery, Value: "teacher gift"}
	plan := ResearchPlan{Scope: scope, DataSource: DataSourceLocal}
	older := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	newer := older.Add(24 * time.Hour)
	snapshots := []Snapshot{
		{Scope: scope, FetchedAt: older, Evidence: []EvidenceRecord{{ID: "old", Title: "Teacher Gift Mug", SearchableText: "teacher gift"}}},
		{Scope: scope, FetchedAt: newer, Evidence: []EvidenceRecord{{ID: "new", Title: "Teacher Gift Bundle", SearchableText: "teacher gift bundle"}}},
		{Scope: ResearchScope{Kind: ScopeQuery, Value: "wedding sign"}, FetchedAt: newer, Evidence: []EvidenceRecord{{ID: "other", Title: "Wedding Sign"}}},
	}

	out := BuildTrendsDiff(scope, snapshots, 7, plan)

	if out.Summary == "" {
		t.Fatal("summary is empty")
	}
	if len(out.NextActions) == 0 {
		t.Fatal("next_actions is empty")
	}
	if len(out.Records) != 1 || out.Records[0].ID != "trend:new" {
		t.Fatalf("records = %+v, want trend:new only", out.Records)
	}
	if len(out.Evidence) != 1 || out.Evidence[0].ID != "new" {
		t.Fatalf("evidence = %+v, want new only", out.Evidence)
	}
	if out.Confidence <= 0 {
		t.Fatalf("confidence = %f, want positive", out.Confidence)
	}
}

func TestTrendsDiffWarnsOnSingleSnapshot(t *testing.T) {
	scope := ResearchScope{Kind: ScopeQuery, Value: "teacher gift"}
	plan := ResearchPlan{Scope: scope, DataSource: DataSourceLocal}
	snapshots := []Snapshot{
		{Scope: scope, FetchedAt: time.Now().UTC(), Evidence: []EvidenceRecord{
			{ID: "only", Title: "Teacher Gift Mug", SearchableText: "teacher gift"},
			{ID: "other", Title: "Wedding Sign", SearchableText: "wedding sign"},
		}},
	}

	out := BuildTrendsDiff(scope, snapshots, 7, plan)

	if out.Confidence >= 0.5 {
		t.Fatalf("confidence = %f, want low confidence", out.Confidence)
	}
	if len(out.Warnings) == 0 {
		t.Fatal("warnings is empty")
	}
	if len(out.Evidence) != 1 || out.Evidence[0].ID != "only" {
		t.Fatalf("evidence = %+v, want only matching evidence", out.Evidence)
	}
}

func TestTrendsDiffDropsNewestSnapshotWithNoMatchingEvidence(t *testing.T) {
	scope := ResearchScope{Kind: ScopeQuery, Value: "teacher gift"}
	plan := ResearchPlan{Scope: scope, DataSource: DataSourceLocal}
	newest := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	snapshots := []Snapshot{
		{Scope: scope, FetchedAt: newest, Evidence: []EvidenceRecord{{ID: "other", Title: "Wedding Sign", SearchableText: "wedding sign"}}},
		{Scope: scope, FetchedAt: newest.Add(-24 * time.Hour), Evidence: []EvidenceRecord{{ID: "new", Title: "Teacher Gift Bundle", SearchableText: "teacher gift bundle"}}},
		{Scope: scope, FetchedAt: newest.Add(-48 * time.Hour), Evidence: []EvidenceRecord{{ID: "old", Title: "Teacher Gift Mug", SearchableText: "teacher gift"}}},
	}

	out := BuildTrendsDiff(scope, snapshots, 7, plan)

	if len(out.Records) != 1 || out.Records[0].ID != "trend:new" {
		t.Fatalf("records = %+v, want trend:new after dropping unrelated newest snapshot", out.Records)
	}
	if len(out.Evidence) != 1 || out.Evidence[0].ID != "new" {
		t.Fatalf("evidence = %+v, want new only", out.Evidence)
	}
}

func TestTrendsDiffIgnoresOutOfWindowPreviousSnapshot(t *testing.T) {
	scope := ResearchScope{Kind: ScopeQuery, Value: "teacher gift"}
	plan := ResearchPlan{Scope: scope, DataSource: DataSourceLocal}
	newest := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	snapshots := []Snapshot{
		{Scope: scope, FetchedAt: newest, Evidence: []EvidenceRecord{{ID: "new", Title: "Teacher Gift Bundle", SearchableText: "teacher gift bundle"}}},
		{Scope: scope, FetchedAt: newest.Add(-30 * 24 * time.Hour), Evidence: []EvidenceRecord{{ID: "old", Title: "Teacher Gift Mug", SearchableText: "teacher gift"}}},
	}

	out := BuildTrendsDiff(scope, snapshots, 7, plan)

	if len(out.Records) != 0 {
		t.Fatalf("records = %+v, want no diff when previous snapshot is outside lookback", out.Records)
	}
	if len(out.Warnings) == 0 {
		t.Fatal("warnings is empty")
	}
	if len(out.Evidence) != 1 || out.Evidence[0].ID != "new" {
		t.Fatalf("evidence = %+v, want newest matching evidence only", out.Evidence)
	}
}
