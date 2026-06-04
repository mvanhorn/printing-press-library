package research

import "testing"

func TestListingAuditFiltersUnrelatedEvidence(t *testing.T) {
	scope := ResearchScope{Kind: ScopeListing, Value: "123456789"}
	plan := ResearchPlan{Scope: scope, DataSource: DataSourceLocal, Warnings: []string{"partial coverage"}}
	evidence := []EvidenceRecord{
		{ID: "p1", Resource: "product_analytics", ListingID: "123456789", Title: "Teacher Gift Mug", Tags: []string{"teacher gift"}, SearchableText: "listing 123456789 teacher gift"},
		{ID: "p2", Resource: "product_analytics", ListingID: "987654321", Title: "Wedding Sign", Tags: []string{"wedding"}, SearchableText: "listing 987654321 wedding sign"},
	}

	out := BuildListingAudit(scope, evidence, 10, plan)

	if out.Summary == "" {
		t.Fatal("summary is empty")
	}
	if len(out.NextActions) == 0 {
		t.Fatal("next_actions is empty")
	}
	if len(out.Warnings) != 1 || out.Warnings[0] != "partial coverage" {
		t.Fatalf("warnings = %+v, want plan warning", out.Warnings)
	}
	assertOnlyInsightRecord(t, out, "p1")
	if out.Confidence <= 0 {
		t.Fatalf("confidence = %f, want positive", out.Confidence)
	}
}
