// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package deepline

import "testing"

func TestToolPersonEnrichRoutesToApolloPeopleMatch(t *testing.T) {
	if ToolPersonEnrich != "apollo_people_match" {
		t.Fatalf("ToolPersonEnrich = %q, want apollo_people_match", ToolPersonEnrich)
	}
}

func TestToolEmailFindRoutesToDropleads(t *testing.T) {
	if ToolEmailFind != "dropleads_email_finder" {
		t.Fatalf("ToolEmailFind = %q, want dropleads_email_finder", ToolEmailFind)
	}
}

func TestAiArkPersonalityConstantPreserved(t *testing.T) {
	if ToolPersonalityAnalysis != "ai_ark_personality_analysis" {
		t.Fatalf("ToolPersonalityAnalysis = %q, want ai_ark_personality_analysis", ToolPersonalityAnalysis)
	}
}

func TestAiArkFindEmailsConstantPreserved(t *testing.T) {
	if ToolPersonSearchToEmailWaterfall != "ai_ark_find_emails" {
		t.Fatalf("ToolPersonSearchToEmailWaterfall = %q, want ai_ark_find_emails",
			ToolPersonSearchToEmailWaterfall)
	}
}

func TestNewProvidersInCatalog(t *testing.T) {
	want := []string{
		ToolDropleadsEmailFinder,
		ToolHunterEmailFinder,
		ToolDatagmaFindEmail,
		ToolIcypeasEmailSearch,
		ToolHunterDomainSearch,
		ToolApolloPeopleMatch,
		ToolHunterPeopleFind,
		ToolContactOutEnrichPerson,
		ToolPersonalityAnalysis,
	}
	for _, id := range want {
		info, ok := LookupTool(id)
		if !ok {
			t.Errorf("LookupTool(%q) missing from catalog", id)
			continue
		}
		if info.ID != id {
			t.Errorf("Catalog[%q].ID = %q, want %q", id, info.ID, id)
		}
		if info.Label == "" {
			t.Errorf("Catalog[%q].Label is empty", id)
		}
		if info.DefaultCredits <= 0 {
			t.Errorf("Catalog[%q].DefaultCredits = %d, want > 0", id, info.DefaultCredits)
		}
	}
}

func TestLookupToolUnknown(t *testing.T) {
	_, ok := LookupTool("not_a_real_tool")
	if ok {
		t.Fatal("LookupTool returned ok for unknown tool id")
	}
}
