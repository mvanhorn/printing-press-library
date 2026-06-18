// Copyright 2026 Dhilip Subramanian and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestParseFederalRegister(t *testing.T) {
	body := []byte(`{
	  "count": 1,
	  "total_pages": 1,
	  "results": [{
	    "title": "General Services Acquisition Regulation",
	    "type": "Proposed Rule",
	    "abstract": "A proposed rule.",
	    "document_number": "2026-12205",
	    "html_url": "https://www.federalregister.gov/documents/2026/06/17/2026-12205/example",
	    "pdf_url": "https://www.govinfo.gov/example.pdf",
	    "publication_date": "2026-06-17",
	    "agencies": [{"name": "General Services Administration", "slug": "general-services-administration"}],
	    "excerpts": "Large Language Model <span class=\"match\">Artificial</span> Intelligence &amp; Policy"
	  }]
	}`)

	result, err := parseFederalRegister(body)
	if err != nil {
		t.Fatalf("parseFederalRegister returned error: %v", err)
	}
	if result.Count != 1 || len(result.Results) != 1 {
		t.Fatalf("unexpected result shape: %#v", result)
	}
	got := result.Results[0]
	if got.DocumentNumber != "2026-12205" {
		t.Fatalf("document number = %s", got.DocumentNumber)
	}
	if got.Agencies[0] != "General Services Administration" {
		t.Fatalf("agency = %v", got.Agencies)
	}
	if got.Excerpt != "Large Language Model Artificial Intelligence & Policy" {
		t.Fatalf("excerpt = %q", got.Excerpt)
	}
}

func TestNormalizeLimit(t *testing.T) {
	if got := normalizeLimit(1, 5, 50); got != 5 {
		t.Fatalf("low limit normalized to %d, want 5", got)
	}
	if got := normalizeLimit(100, 5, 50); got != 50 {
		t.Fatalf("high limit normalized to %d, want 50", got)
	}
}
