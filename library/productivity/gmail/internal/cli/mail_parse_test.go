// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"testing"
)

func TestParseFromHeader(t *testing.T) {
	cases := []struct {
		name      string
		in        string
		wantEmail string
		wantName  string
	}{
		{"bare addr", "news@letters.example", "news@letters.example", ""},
		{"name and addr", "Letters Weekly <news@letters.example>", "news@letters.example", "Letters Weekly"},
		{"quoted name with comma", `"Doe, Jane" <jane.doe@example.com>`, "jane.doe@example.com", "Doe, Jane"},
		{"utf8 name", "Ana Müller <ana@example.de>", "ana@example.de", "Ana Müller"},
		{"rfc2047 encoded name", "=?UTF-8?Q?Ana_M=C3=BCller?= <ana@example.de>", "ana@example.de", "Ana Müller"},
		{"angle only", "<alerts@bank.example>", "alerts@bank.example", ""},
		{"empty", "", "", ""},
		{"whitespace only", "   ", "", ""},
		{"malformed no addr", "totally busted header", "", "totally busted header"},
		{"malformed but recoverable angle", "Deals!! Best, Ever <deals@shop.example>", "deals@shop.example", "Deals!! Best, Ever"},
		{"uppercase email lowercased", "Foo <BAR@Example.COM>", "bar@example.com", "Foo"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			email, name := parseFromHeader(c.in)
			if email != c.wantEmail || name != c.wantName {
				t.Fatalf("parseFromHeader(%q) = (%q, %q), want (%q, %q)",
					c.in, email, name, c.wantEmail, c.wantName)
			}
		})
	}
}

func TestDeriveCategory(t *testing.T) {
	cases := []struct {
		name   string
		labels []string
		want   string
	}{
		{"promotions", []string{"UNREAD", "CATEGORY_PROMOTIONS", "INBOX"}, "promotions"},
		{"social", []string{"CATEGORY_SOCIAL"}, "social"},
		{"updates", []string{"INBOX", "CATEGORY_UPDATES"}, "updates"},
		{"forums", []string{"CATEGORY_FORUMS"}, "forums"},
		{"personal is primary", []string{"CATEGORY_PERSONAL", "INBOX"}, "primary"},
		{"inbox without category is primary", []string{"INBOX", "UNREAD"}, "primary"},
		{"archived uncategorized is empty", []string{"UNREAD"}, ""},
		{"no labels", nil, ""},
		{"trash only", []string{"TRASH"}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := deriveCategory(c.labels); got != c.want {
				t.Fatalf("deriveCategory(%v) = %q, want %q", c.labels, got, c.want)
			}
		})
	}
}

func TestMailMetaFromMessage(t *testing.T) {
	raw := `{
		"id": "18f3a",
		"threadId": "18f00",
		"labelIds": ["UNREAD", "CATEGORY_PROMOTIONS"],
		"snippet": "This week only …",
		"sizeEstimate": 54321,
		"internalDate": "1723972800000",
		"payload": {"headers": [
			{"name": "From", "value": "Letters Weekly <News@Letters.example>"},
			{"name": "Subject", "value": "This week's letters"},
			{"name": "List-Unsubscribe", "value": "<https://letters.example/u/1>, <mailto:u@letters.example>"},
			{"name": "List-Unsubscribe-Post", "value": "List-Unsubscribe=One-Click"},
			{"name": "Authentication-Results", "value": "mx.google.com; dkim=pass header.i=@letters.example; spf=pass"}
		]}
	}`
	var msg gmailMessageMeta
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatal(err)
	}
	m := mailMetaFromMessage("ads", &msg)
	if m.Account != "ads" || m.ID != "18f3a" || m.ThreadID != "18f00" {
		t.Fatalf("identity fields wrong: %+v", m)
	}
	if m.FromEmail != "news@letters.example" || m.FromName != "Letters Weekly" {
		t.Fatalf("from parse wrong: %q %q", m.FromEmail, m.FromName)
	}
	if m.Subject != "This week's letters" || m.Snippet != "This week only …" {
		t.Fatalf("subject/snippet wrong: %+v", m)
	}
	if m.InternalDate != 1723972800000 || m.SizeEstimate != 54321 {
		t.Fatalf("date/size wrong: %+v", m)
	}
	if m.Category != "promotions" || !m.Unread {
		t.Fatalf("derived fields wrong: %+v", m)
	}
	if m.ListUnsubscribe == "" || m.ListUnsubscribePost != "List-Unsubscribe=One-Click" {
		t.Fatalf("unsubscribe headers wrong: %+v", m)
	}
	if m.AuthResults != "mx.google.com; dkim=pass header.i=@letters.example; spf=pass" {
		t.Fatalf("auth_results not captured: %q", m.AuthResults)
	}
	if m.ListUnsubDomain != "" {
		t.Fatalf("list_unsub_domain must stay empty at sync time, got %q", m.ListUnsubDomain)
	}

	// Missing headers / nil labels degrade to zero values, not panics.
	var empty gmailMessageMeta
	empty.ID = "x"
	m2 := mailMetaFromMessage("ads", &empty)
	if m2.FromEmail != "" || m2.Category != "" || m2.Unread || m2.InternalDate != 0 {
		t.Fatalf("empty message mapping wrong: %+v", m2)
	}
	if m2.LabelIDs == nil {
		t.Fatal("LabelIDs must round-trip as [], not nil")
	}
}
