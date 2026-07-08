package agent

import "testing"

func TestRenderBriefHappyPath(t *testing.T) {
	in := BriefInput{
		Opportunity: Opportunity{ID: "006O1", Name: "Acme Renewal", StageName: "Negotiation", Amount: 50000, CloseDate: "2026-06-30"},
		Account:     &Account{ID: "001A1", Name: "Acme Corp"},
		Contacts:    []Contact{{ID: "003C1", FirstName: "Ada", LastName: "Lovelace", Title: "VP Eng", Email: "ada@acme.example"}},
		Activities: []Activity{
			{ID: "00T1", Subject: "Kickoff call", ActivityDate: "2026-04-01"},
			{ID: "00T2", Subject: "Follow-up email", ActivityDate: "2026-04-10"},
		},
		Feed: []FeedItem{
			{ID: "0D51", Body: "Deal moving forward", CreatedAt: "2026-04-15"},
		},
	}
	md, j := RenderBrief(in)
	if !contains(md, "# Acme Renewal") {
		t.Fatalf("markdown missing title:\n%s", md)
	}
	if !contains(md, "Stage: Negotiation") {
		t.Fatalf("markdown missing stage:\n%s", md)
	}
	if !contains(md, "ada@acme.example") {
		t.Fatalf("markdown missing contact email:\n%s", md)
	}
	if j.OpportunityID != "006O1" {
		t.Fatalf("json id mismatch: %s", j.OpportunityID)
	}
	if len(j.RecentActivities) != 2 {
		t.Fatalf("expected 2 activities, got %d", len(j.RecentActivities))
	}
}

// TestRenderBriefEmpty confirms the renderer produces something reasonable
// even when contacts/activities/feed are all empty.
func TestRenderBriefEmpty(t *testing.T) {
	in := BriefInput{Opportunity: Opportunity{ID: "006X", Name: "", StageName: ""}}
	md, _ := RenderBrief(in)
	if !contains(md, "006X") {
		t.Fatalf("expected ID fallback in markdown, got:\n%s", md)
	}
	if !contains(md, "Stage: unknown") {
		t.Fatalf("expected 'Stage: unknown' fallback, got:\n%s", md)
	}
}
