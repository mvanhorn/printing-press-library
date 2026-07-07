package cli

import "testing"

func TestAuditCSVContactsFindsInvalidAndDuplicateEmails(t *testing.T) {
	report := auditCSVContacts([]csvContact{
		{Email: "Reader@example.com", FirstName: "Jane"},
		{Email: "bad-email"},
		{Email: "reader@example.com", FirstName: "Dupe"},
	}, []int{123})

	if got := report["valid_emails"]; got != 2 {
		t.Fatalf("valid_emails = %v, want 2", got)
	}
	if got := report["invalid_count"]; got != 1 {
		t.Fatalf("invalid_count = %v, want 1", got)
	}
	if got := report["duplicate_count"]; got != 1 {
		t.Fatalf("duplicate_count = %v, want 1", got)
	}
}

func TestLaunchPlanDoesNotInventCampaignWriteAPI(t *testing.T) {
	plan := launchPlanPayload(123, "June newsletter", "https://example.com", "subscribers.csv", nil)
	if got := plan["api_gap"]; got == "" {
		t.Fatal("expected api_gap to call out dashboard-only campaign writes")
	}
	commands, ok := plan["commands"].([]string)
	if !ok || len(commands) == 0 {
		t.Fatalf("commands missing: %#v", plan["commands"])
	}
	for _, cmd := range commands {
		if cmd == "" {
			t.Fatal("empty launch-plan command")
		}
	}
}

func TestCapabilitiesIncludeDashboardOnlyHandoffs(t *testing.T) {
	var sawCampaigns, sawWebhooks bool
	for _, cap := range sendfoxCapabilities() {
		switch cap.Resource {
		case "campaigns":
			sawCampaigns = true
			if cap.Create || cap.MutationOK {
				t.Fatalf("campaigns should not advertise public create support: %#v", cap)
			}
		case "webhooks":
			sawWebhooks = true
			if cap.PublicAPI {
				t.Fatalf("webhooks should be dashboard handoff-only: %#v", cap)
			}
		}
	}
	if !sawCampaigns || !sawWebhooks {
		t.Fatalf("expected campaign and webhook capability rows")
	}
}
