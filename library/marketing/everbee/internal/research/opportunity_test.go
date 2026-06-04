package research

import "testing"

func TestOpportunityShortlistRanksMatchingEvidence(t *testing.T) {
	scope := ResearchScope{Kind: ScopeQuery, Value: "teacher gift"}
	plan := ResearchPlan{Scope: scope, DataSource: DataSourceLocal}
	evidence := []EvidenceRecord{
		{ID: "p1", Title: "Teacher Gift Mug", SearchableText: "teacher gift mug", Tags: []string{"teacher", "gift"}},
		{ID: "p2", Title: "Wedding Sign", SearchableText: "wedding sign"},
	}

	out := BuildOpportunityShortlist(scope, evidence, 10, plan)

	if len(out.Records) != 1 {
		t.Fatalf("records = %d, want 1", len(out.Records))
	}
	if out.Records[0].ID != "p1" {
		t.Fatalf("top id = %s, want p1", out.Records[0].ID)
	}
	if out.Confidence <= 0 {
		t.Fatalf("confidence = %f, want positive", out.Confidence)
	}
}

func TestOpportunityShortlistMatchesKeywords(t *testing.T) {
	scope := ResearchScope{Kind: ScopeQuery, Value: "teacher gift"}
	plan := ResearchPlan{Scope: scope, DataSource: DataSourceLocal}
	evidence := []EvidenceRecord{
		{ID: "p1", Title: "Desk Organizer", Keywords: []string{"teacher gift", "classroom"}},
		{ID: "p2", Title: "Wedding Sign", Keywords: []string{"wedding"}},
	}

	out := BuildOpportunityShortlist(scope, evidence, 10, plan)

	if len(out.Records) != 1 {
		t.Fatalf("records = %d, want 1", len(out.Records))
	}
	if out.Records[0].ID != "p1" {
		t.Fatalf("top id = %s, want p1", out.Records[0].ID)
	}
}

func TestOpportunityShortlistUsesDemandAndRankSignals(t *testing.T) {
	scope := ResearchScope{Kind: ScopeQuery, Value: "teacher gift"}
	plan := ResearchPlan{Scope: scope, DataSource: DataSourceLocal}
	sales := 120.0
	revenue := 1800.0
	evidence := []EvidenceRecord{
		{ID: "low", Title: "Teacher Gift Printable", SearchableText: "teacher gift"},
		{ID: "high", Title: "Teacher Gift Mug", SearchableText: "teacher gift", EstimatedSales: &sales, EstimatedRevenue: &revenue, Rank: 1},
	}

	out := BuildOpportunityShortlist(scope, evidence, 10, plan)

	if len(out.Records) != 2 {
		t.Fatalf("records = %d, want 2", len(out.Records))
	}
	if out.Records[0].ID != "high" {
		t.Fatalf("top id = %s, want high", out.Records[0].ID)
	}
}

func TestOpportunityShortlistAppliesLimit(t *testing.T) {
	scope := ResearchScope{Kind: ScopeQuery, Value: "teacher gift"}
	plan := ResearchPlan{Scope: scope, DataSource: DataSourceLocal}
	evidence := []EvidenceRecord{
		{ID: "p1", Title: "Teacher Gift Mug"},
		{ID: "p2", Title: "Teacher Gift Tote"},
		{ID: "p3", Title: "Teacher Gift Card"},
	}

	out := BuildOpportunityShortlist(scope, evidence, 2, plan)

	if len(out.Records) != 2 {
		t.Fatalf("records = %d, want 2", len(out.Records))
	}
}
