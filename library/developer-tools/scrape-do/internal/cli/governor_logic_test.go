// Copyright 2026 Charles Garrison and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored tests for the pure governor + SERP logic: the credit cost
// table, SERP param-hash canonicalization, organic extraction from a real
// Scrape.do Google SERP shape, and the drift diff. These are the correctness
// guarantees behind `cost`, `google search`, `drift`, and `movers`.

package cli

import (
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-do/internal/config"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-do/internal/store"
)

func TestBuildRequestURLTokenGuard(t *testing.T) {
	cfg := &config.Config{BaseURL: "https://api.scrape.do", ScrapedoApiKey: "REALTOKEN"}
	// An accidental or malicious `--param token=bad` must not override the key.
	req := scrapeRequest{path: "/plugin/google/search", params: map[string]string{"q": "x", "token": "bad"}}
	got := buildRequestURL(cfg, req)
	if !strings.Contains(got, "token=REALTOKEN") {
		t.Errorf("URL must carry the configured token; got %q", got)
	}
	if strings.Contains(got, "token=bad") {
		t.Errorf("user-supplied token must be dropped, not sent; got %q", got)
	}
	if !strings.Contains(got, "q=x") {
		t.Errorf("non-reserved params must pass through; got %q", got)
	}
}

func TestEstimateScrapeCost(t *testing.T) {
	tests := []struct {
		name          string
		url           string
		render, super bool
		wantCredits   int
		wantMode      string
	}{
		{"datacenter default", "https://example.com", false, false, 1, modeDatacenter},
		{"datacenter+render", "https://example.com", true, false, 5, modeDatacenterRender},
		{"super", "https://example.com", false, true, 10, modeSuper},
		{"super+render", "https://example.com", true, true, 25, modeSuperRender},
		{"linkedin override beats render", "https://www.linkedin.com/in/x", true, false, 30, "domain:linkedin.com"},
		{"linkedin override beats super+render", "https://linkedin.com/co/y", true, true, 30, "domain:linkedin.com"},
		{"shopee override", "https://shopee.com/x", false, false, 100, "domain:shopee.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCredits, gotMode := estimateScrapeCost(tt.url, tt.render, tt.super)
			if gotCredits != tt.wantCredits {
				t.Errorf("credits = %d, want %d", gotCredits, tt.wantCredits)
			}
			if gotMode != tt.wantMode {
				t.Errorf("mode = %q, want %q", gotMode, tt.wantMode)
			}
		})
	}
}

func TestBaseModeCost(t *testing.T) {
	want := map[string]int{modeDatacenter: 1, modeDatacenterRender: 5, modeSuper: 10, modeSuperRender: 25, modeGoogle: 10}
	for mode, c := range want {
		if got := baseModeCost(mode); got != c {
			t.Errorf("baseModeCost(%q) = %d, want %d", mode, got, c)
		}
	}
}

func TestSerpParamHash(t *testing.T) {
	base := serpParamHash("best crm", "us", "en", "google.com", "desktop")
	// Case/whitespace-insensitive and default-equivalent inputs hash the same.
	if got := serpParamHash("  Best CRM ", "US", "EN", "", ""); got != base {
		t.Errorf("normalization mismatch: %s != %s", got, base)
	}
	// A different locale must produce a different identity.
	if serpParamHash("best crm", "gb", "en", "google.com", "desktop") == base {
		t.Error("different gl should change the hash")
	}
	// A different query must produce a different identity.
	if serpParamHash("worst crm", "us", "en", "google.com", "desktop") == base {
		t.Error("different query should change the hash")
	}
}

func TestExtractOrganic(t *testing.T) {
	raw := []byte(`{"organic_results":[
		{"position":1,"title":"Salesforce","link":"https://www.salesforce.com","source":"salesforce.com"},
		{"position":2,"title":"HubSpot","link":"https://hubspot.com/crm","snippet":"free crm"},
		{"title":"NoPos","link":"https://zoho.com"}
	]}`)
	got := extractOrganic(raw)
	if len(got) != 3 {
		t.Fatalf("got %d organic rows, want 3", len(got))
	}
	if got[0].Domain != "salesforce.com" {
		t.Errorf("row0 domain = %q, want salesforce.com (from source)", got[0].Domain)
	}
	if got[1].Domain != "hubspot.com" {
		t.Errorf("row1 domain = %q, want hubspot.com (from link host)", got[1].Domain)
	}
	if got[1].Snippet != "free crm" {
		t.Errorf("row1 snippet = %q", got[1].Snippet)
	}
	// Missing position falls back to the 1-based index.
	if got[2].Position != 3 {
		t.Errorf("row2 position = %d, want 3 (index fallback)", got[2].Position)
	}
}

func TestExtractOrganicEmpty(t *testing.T) {
	// Absence-of-correctness: malformed/empty payloads return no rows, not panics.
	if got := extractOrganic([]byte(`not json`)); got != nil {
		t.Errorf("malformed JSON should yield nil, got %v", got)
	}
	if got := extractOrganic([]byte(`{"organic_results":[]}`)); len(got) != 0 {
		t.Errorf("empty organic_results should yield 0 rows, got %d", len(got))
	}
}

func TestDiffOrganic(t *testing.T) {
	prev := []store.OrganicRow{
		{Position: 1, Domain: "a.com"},
		{Position: 2, Domain: "b.com"},
		{Position: 3, Domain: "c.com"},
	}
	cur := []store.OrganicRow{
		{Position: 1, Domain: "b.com"}, // moved up 1
		{Position: 2, Domain: "a.com"}, // moved down 1
		{Position: 3, Domain: "d.com"}, // new
		// c.com dropped
	}
	movers := diffOrganic(cur, prev)
	byDomain := map[string]driftMover{}
	for _, m := range movers {
		byDomain[m.Domain] = m
	}
	if m := byDomain["b.com"]; m.Status != "moved" || m.Delta == nil || *m.Delta != 1 {
		t.Errorf("b.com: %+v, want moved +1", m)
	}
	if m := byDomain["a.com"]; m.Status != "moved" || m.Delta == nil || *m.Delta != -1 {
		t.Errorf("a.com: %+v, want moved -1", m)
	}
	if m := byDomain["d.com"]; m.Status != "new" {
		t.Errorf("d.com: %+v, want new", m)
	}
	if m := byDomain["c.com"]; m.Status != "dropped" {
		t.Errorf("c.com: %+v, want dropped", m)
	}
}

func TestParseSinceDur(t *testing.T) {
	if _, err := parseSinceDur("7d"); err != nil {
		t.Errorf("7d should parse: %v", err)
	}
	if _, err := parseSinceDur("24h"); err != nil {
		t.Errorf("24h should parse: %v", err)
	}
	if _, err := parseSinceDur("2w"); err != nil {
		t.Errorf("2w should parse: %v", err)
	}
	if _, err := parseSinceDur("today"); err != nil {
		t.Errorf("today should parse: %v", err)
	}
	if _, err := parseSinceDur("garbage"); err == nil {
		t.Error("garbage should not parse")
	}
}
