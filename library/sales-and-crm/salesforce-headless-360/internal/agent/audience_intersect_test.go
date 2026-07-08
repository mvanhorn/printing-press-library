package agent

import (
	"strings"
	"testing"
)

func TestIntersectAudienceFLSKeepsOnlyCommonFields(t *testing.T) {
	result, err := IntersectAudienceFLS([]AudienceMember{
		{SalesforceUserID: "005A", Fields: map[string][]string{"Account": {"Name", "Industry"}, "Opportunity": {"Name", "Amount"}}},
		{SalesforceUserID: "005B", Fields: map[string][]string{"Account": {"Name"}, "Opportunity": {"Name", "StageName"}}},
	}, false)
	if err != nil {
		t.Fatalf("IntersectAudienceFLS: %v", err)
	}
	if !result.Fields["Account"]["Name"] || result.Fields["Account"]["Industry"] {
		t.Fatalf("unexpected account intersection: %+v", result.Fields["Account"])
	}
	if !result.Fields["Opportunity"]["Name"] || result.Fields["Opportunity"]["Amount"] {
		t.Fatalf("unexpected opportunity intersection: %+v", result.Fields["Opportunity"])
	}
}

func TestIntersectAudienceFLSRejectsExternalUnlessWaived(t *testing.T) {
	_, err := IntersectAudienceFLS([]AudienceMember{{SlackUserID: "UEXT", External: true}}, false)
	if err == nil {
		t.Fatal("expected external channel member to abort")
	}
	result, err := IntersectAudienceFLS([]AudienceMember{{SlackUserID: "UEXT", External: true}}, true)
	if err != nil {
		t.Fatalf("waived IntersectAudienceFLS: %v", err)
	}
	if !result.Waived {
		t.Fatal("expected waiver recorded")
	}
}

func TestRenderInjectMarkdownUsesIntersectedFields(t *testing.T) {
	bundle := Bundle{Manifest: Manifest{
		Account:       &Account{Name: "Acme", Industry: "Manufacturing"},
		Opportunities: []Opportunity{{Name: "Renewal", Amount: 1000}},
	}}
	md, err := RenderInjectMarkdown(bundle, AudienceIntersection{Fields: map[string]map[string]bool{
		"Account":     {"Name": true},
		"Opportunity": {"Name": true},
	}})
	if err != nil {
		t.Fatalf("RenderInjectMarkdown: %v", err)
	}
	if !strings.Contains(md, "Acme") || strings.Contains(md, "Manufacturing") || strings.Contains(md, "1000") {
		t.Fatalf("markdown did not honor intersection:\n%s", md)
	}
}
