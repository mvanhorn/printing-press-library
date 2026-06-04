package research

import "testing"

func TestNicheScoreFiltersUnrelatedEvidence(t *testing.T) {
	scope := ResearchScope{Kind: ScopeKeyword, Value: "teacher gift"}
	plan := ResearchPlan{Scope: scope, DataSource: DataSourceLocal, Warnings: []string{"stale data"}}
	evidence := []EvidenceRecord{
		{ID: "p1", Resource: "product_analytics", Title: "Teacher Gift Mug", Keywords: []string{"teacher gift"}, SearchableText: "teacher gift mug"},
		{ID: "p2", Resource: "product_analytics", Title: "Wedding Sign", Keywords: []string{"wedding sign"}, SearchableText: "wedding sign"},
	}

	out := BuildNicheScore(scope, evidence, plan)

	if out.Summary == "" {
		t.Fatal("summary is empty")
	}
	if len(out.NextActions) == 0 {
		t.Fatal("next_actions is empty")
	}
	if len(out.Warnings) != 1 || out.Warnings[0] != "stale data" {
		t.Fatalf("warnings = %+v, want plan warning", out.Warnings)
	}
	assertOnlyInsightRecord(t, out, "p1")
	if out.Confidence <= 0 {
		t.Fatalf("confidence = %f, want positive", out.Confidence)
	}
}

func TestKeywordClustersFilterUnrelatedEvidence(t *testing.T) {
	scope := ResearchScope{Kind: ScopeKeyword, Value: "teacher gift"}
	plan := ResearchPlan{Scope: scope, DataSource: DataSourceLocal}
	evidence := []EvidenceRecord{
		{ID: "k1", Resource: "keyword_research", Keywords: []string{"teacher gift mug", "teacher appreciation"}, SearchableText: "teacher gift keyword"},
		{ID: "k2", Resource: "keyword_research", Keywords: []string{"wedding sign"}, SearchableText: "wedding sign keyword"},
	}

	out := BuildKeywordClusters(scope, evidence, 10, plan)

	if out.Summary == "" {
		t.Fatal("summary is empty")
	}
	if len(out.NextActions) == 0 {
		t.Fatal("next_actions is empty")
	}
	if len(out.Records) == 0 {
		t.Fatal("records is empty")
	}
	for _, record := range out.Records {
		for _, evidenceID := range record.EvidenceIDs {
			if evidenceID == "k2" {
				t.Fatalf("unrelated evidence appeared in cluster: %+v", out.Records)
			}
		}
	}
	if out.Confidence <= 0 {
		t.Fatalf("confidence = %f, want positive", out.Confidence)
	}
}

func TestTagGapFiltersUnrelatedEvidence(t *testing.T) {
	scope := ResearchScope{Kind: ScopeQuery, Value: "teacher gift"}
	plan := ResearchPlan{Scope: scope, DataSource: DataSourceLocal}
	evidence := []EvidenceRecord{
		{ID: "t1", Resource: "product_analytics", Title: "Teacher Gift Mug", Tags: []string{"teacher gift", "school"}, SearchableText: "teacher gift mug"},
		{ID: "t2", Resource: "product_analytics", Title: "Wedding Sign", Tags: []string{"wedding"}, SearchableText: "wedding sign"},
	}

	out := BuildTagGap(scope, evidence, 10, plan)

	if out.Summary == "" {
		t.Fatal("summary is empty")
	}
	if len(out.NextActions) == 0 {
		t.Fatal("next_actions is empty")
	}
	if len(out.Records) == 0 {
		t.Fatal("records is empty")
	}
	for _, record := range out.Records {
		for _, evidenceID := range record.EvidenceIDs {
			if evidenceID == "t2" {
				t.Fatalf("unrelated evidence appeared in tags: %+v", out.Records)
			}
		}
	}
	if out.Confidence <= 0 {
		t.Fatalf("confidence = %f, want positive", out.Confidence)
	}
}

func assertOnlyInsightRecord(t *testing.T, out ResponseEnvelope, wantID string) {
	t.Helper()
	if len(out.Records) != 1 {
		t.Fatalf("records = %d, want 1", len(out.Records))
	}
	if out.Records[0].ID != wantID {
		t.Fatalf("record id = %s, want %s", out.Records[0].ID, wantID)
	}
	if len(out.Evidence) != 1 || out.Evidence[0].ID != wantID {
		t.Fatalf("evidence = %+v, want %s only", out.Evidence, wantID)
	}
}
