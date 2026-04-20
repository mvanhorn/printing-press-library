// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/deepline"
)

func TestSplitName(t *testing.T) {
	cases := []struct {
		in, first, last string
	}{
		{"Patrick Collison", "Patrick", "Collison"},
		{"Mike Craig", "Mike", "Craig"},
		{"Mary Anne Smith", "Mary Anne", "Smith"},
		{"Madonna", "Madonna", ""},
		{"  Jean-Paul Sartre  ", "Jean-Paul", "Sartre"},
		{"", "", ""},
	}
	for _, c := range cases {
		f, l := splitName(c.in)
		if f != c.first || l != c.last {
			t.Errorf("splitName(%q) = (%q, %q), want (%q, %q)", c.in, f, l, c.first, c.last)
		}
	}
}

func TestDeeplineProviderChainLinkedIn(t *testing.T) {
	chain := deeplineProviderChain("linkedin_url", "https://www.linkedin.com/in/patrickcollison", "")
	if len(chain) != 3 {
		t.Fatalf("linkedin_url chain length = %d, want 3", len(chain))
	}
	wantTools := []string{
		deepline.ToolApolloPeopleMatch,
		deepline.ToolHunterPeopleFind,
		deepline.ToolContactOutEnrichPerson,
	}
	for i, a := range chain {
		if a.toolID != wantTools[i] {
			t.Errorf("chain[%d].toolID = %q, want %q", i, a.toolID, wantTools[i])
		}
	}
	if v, ok := chain[0].payload["reveal_personal_emails"].(bool); !ok || !v {
		t.Errorf("apollo attempt missing reveal_personal_emails=true: %v", chain[0].payload)
	}
	if chain[0].payload["linkedin_url"] != "https://www.linkedin.com/in/patrickcollison" {
		t.Errorf("apollo attempt missing linkedin_url payload key: %v", chain[0].payload)
	}
}

func TestDeeplineProviderChainName(t *testing.T) {
	chain := deeplineProviderChain("name", "Mike Craig", "stripe.com")
	if len(chain) != 3 {
		t.Fatalf("name chain length = %d, want 3", len(chain))
	}
	// First provider = dropleads; payload must use company_domain, NOT domain.
	drop := chain[0]
	if drop.toolID != deepline.ToolDropleadsEmailFinder {
		t.Errorf("chain[0].toolID = %q, want dropleads_email_finder", drop.toolID)
	}
	if drop.payload["company_domain"] != "stripe.com" {
		t.Errorf("dropleads attempt missing company_domain=stripe.com: %v", drop.payload)
	}
	if _, wrong := drop.payload["domain"]; wrong {
		t.Errorf("dropleads attempt uses wrong key 'domain' (upstream rejects): %v", drop.payload)
	}
	if drop.payload["first_name"] != "Mike" || drop.payload["last_name"] != "Craig" {
		t.Errorf("dropleads attempt bad name split: %v", drop.payload)
	}
	// Hunter uses `domain` (not `company_domain`).
	hunter := chain[1]
	if hunter.toolID != deepline.ToolHunterEmailFinder {
		t.Errorf("chain[1].toolID = %q, want hunter_email_finder", hunter.toolID)
	}
	if hunter.payload["domain"] != "stripe.com" {
		t.Errorf("hunter attempt missing domain=stripe.com: %v", hunter.payload)
	}
}

func TestDeeplineProviderChainEmail(t *testing.T) {
	chain := deeplineProviderChain("email", "patrick@stripe.com", "")
	if len(chain) != 2 {
		t.Fatalf("email chain length = %d, want 2", len(chain))
	}
	if chain[0].payload["email"] != "patrick@stripe.com" {
		t.Errorf("apollo attempt missing email payload: %v", chain[0].payload)
	}
}

func TestExtractApolloPersonFillsPersonalAndWorkEmail(t *testing.T) {
	raw := []byte(`{"job_id":"x","status":"completed","result":{"data":{"person":{
		"name":"Mike Craig","title":"Head of Engineering",
		"linkedin_url":"https://www.linkedin.com/in/mkscrg",
		"email":"mike@stripe.com","email_status":"verified",
		"personal_emails":["mkscrg@gmail.com"],
		"organization":{"name":"Stripe"}
	}}}}`)
	r := &WaterfallResult{Fields: map[string]any{}}
	step := &WaterfallStep{}
	extractDeeplineFields(r, json.RawMessage(raw), "apollo", step)
	if r.Fields["name"] != "Mike Craig" {
		t.Errorf("name = %v", r.Fields["name"])
	}
	if r.Fields["email"] != "mike@stripe.com" {
		t.Errorf("email = %v", r.Fields["email"])
	}
	if r.Fields["personal_email"] != "mkscrg@gmail.com" {
		t.Errorf("personal_email = %v", r.Fields["personal_email"])
	}
	if r.Fields["email_confidence"] != "verified" {
		t.Errorf("email_confidence = %v", r.Fields["email_confidence"])
	}
}

func TestExtractApolloPersonOmitsUnavailableWorkEmail(t *testing.T) {
	// Apollo returns email="" and email_status="unavailable" when it has no
	// verified work address. The extractor must NOT fill "email" with "" and
	// must NOT treat "unavailable" as a hit.
	raw := []byte(`{"result":{"data":{"person":{
		"name":"Mike Craig",
		"email":"","email_status":"unavailable",
		"personal_emails":["mkscrg@gmail.com"]
	}}}}`)
	r := &WaterfallResult{Fields: map[string]any{}}
	step := &WaterfallStep{}
	extractDeeplineFields(r, json.RawMessage(raw), "apollo", step)
	if _, has := r.Fields["email"]; has {
		t.Errorf("email unexpectedly set when Apollo reported unavailable: %v", r.Fields["email"])
	}
	if r.Fields["personal_email"] != "mkscrg@gmail.com" {
		t.Errorf("personal_email should still be set: %v", r.Fields["personal_email"])
	}
}

func TestExtractDropleadsRecordsCatchAllStatus(t *testing.T) {
	raw := []byte(`{"result":{"data":{
		"email":"mike@stripe.com","status":"catch_all",
		"mx_provider":"Google","company_domain":"stripe.com"
	}}}`)
	r := &WaterfallResult{Fields: map[string]any{}}
	step := &WaterfallStep{}
	extractDeeplineFields(r, json.RawMessage(raw), "dropleads", step)
	if r.Fields["email"] != "mike@stripe.com" {
		t.Errorf("email = %v", r.Fields["email"])
	}
	if r.Fields["email_confidence"] != "catch_all" {
		t.Errorf("email_confidence = %v, want catch_all", r.Fields["email_confidence"])
	}
}

func TestClassifyTargetKinds(t *testing.T) {
	cases := map[string]string{
		"alice@stripe.com":                          "email",
		"https://www.linkedin.com/in/patrickcollison": "linkedin_url",
		"http://linkedin.com/in/foo/":               "linkedin_url",
		"Mike Craig":                                "name",
		"just-a-handle":                             "name",
	}
	for in, want := range cases {
		if got := classifyTarget(in); got != want {
			t.Errorf("classifyTarget(%q) = %q, want %q", in, got, want)
		}
	}
}
