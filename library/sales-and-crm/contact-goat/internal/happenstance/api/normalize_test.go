// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package api

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/client"
)

// TestToClientPerson_HappyPath covers the canonical case from the plan:
// a SearchResult with all four fields populated normalizes into a Person
// where exactly those four fields carry the same values. Every other
// Person field stays at its Go zero value.
func TestToClientPerson_HappyPath(t *testing.T) {
	in := SearchResult{
		Name:                "Adam Silver",
		CurrentTitle:        "Commissioner",
		CurrentCompany:      "NBA",
		WeightedTraitsScore: 0.91,
	}

	got := ToClientPerson(in)

	if got.Name != "Adam Silver" {
		t.Errorf("Name = %q, want %q", got.Name, "Adam Silver")
	}
	if got.CurrentTitle != "Commissioner" {
		t.Errorf("CurrentTitle = %q, want %q", got.CurrentTitle, "Commissioner")
	}
	if got.CurrentCompany != "NBA" {
		t.Errorf("CurrentCompany = %q, want %q", got.CurrentCompany, "NBA")
	}
	if got.Score != 0.91 {
		t.Errorf("Score = %v, want %v", got.Score, 0.91)
	}

	// Every other Person field should be the zero value. The bearer
	// surface does not return them; renderers must tolerate the
	// emptiness. Spot-check the ones most likely to crash a renderer
	// if they were unexpectedly populated.
	if got.LinkedInURL != "" {
		t.Errorf("LinkedInURL = %q, want empty", got.LinkedInURL)
	}
	if got.TwitterURL != "" {
		t.Errorf("TwitterURL = %q, want empty", got.TwitterURL)
	}
	if got.InstagramURL != "" {
		t.Errorf("InstagramURL = %q, want empty", got.InstagramURL)
	}
	if got.Quotes != "" {
		t.Errorf("Quotes = %q, want empty", got.Quotes)
	}
	if got.QuotesCited != nil {
		t.Errorf("QuotesCited = %v, want nil", got.QuotesCited)
	}
	if got.PersonUUID != "" {
		t.Errorf("PersonUUID = %q, want empty", got.PersonUUID)
	}
	if got.Summary != "" {
		t.Errorf("Summary = %q, want empty", got.Summary)
	}
}

// TestToClientPerson_MissingCurrentCompany covers the plan's edge case:
// when current_company is omitted by the upstream API, the normalized
// Person carries an empty string (not a sentinel, not nil-via-pointer).
// Downstream string ops (concatenation, len(), strings.Contains) stay
// safe.
func TestToClientPerson_MissingCurrentCompany(t *testing.T) {
	in := SearchResult{
		Name:                "Anonymous Founder",
		CurrentTitle:        "Stealth",
		CurrentCompany:      "", // upstream omitted current_company
		WeightedTraitsScore: 0.42,
	}

	got := ToClientPerson(in)

	if got.CurrentCompany != "" {
		t.Errorf("CurrentCompany = %q, want empty string", got.CurrentCompany)
	}
	// Sanity: an empty string is safe for downstream ops.
	if len(got.CurrentCompany) != 0 {
		t.Errorf("len(CurrentCompany) = %d, want 0", len(got.CurrentCompany))
	}
	// The other three fields still hydrate.
	if got.Name != "Anonymous Founder" {
		t.Errorf("Name = %q, want %q", got.Name, "Anonymous Founder")
	}
	if got.CurrentTitle != "Stealth" {
		t.Errorf("CurrentTitle = %q, want %q", got.CurrentTitle, "Stealth")
	}
	if got.Score != 0.42 {
		t.Errorf("Score = %v, want %v", got.Score, 0.42)
	}
}

// TestToClientPersonFromResearch_HappyPath covers the standard
// research-surface projection: Summary becomes Quotes,
// Employment[0].{Title,Company} become CurrentTitle/CurrentCompany,
// and the caller-supplied displayName becomes Name.
func TestToClientPersonFromResearch_HappyPath(t *testing.T) {
	in := ResearchProfile{
		Employment: []EmploymentEntry{
			{Title: "Commissioner", Company: "NBA", StartDate: "2014-02-01"},
			{Title: "Deputy Commissioner", Company: "NBA", StartDate: "2006-07-01", EndDate: "2014-01-31"},
		},
		Summary: "Long-tenured NBA executive who became commissioner in 2014.",
	}

	got := ToClientPersonFromResearch(in, "Adam Silver")

	if got.Name != "Adam Silver" {
		t.Errorf("Name = %q, want %q", got.Name, "Adam Silver")
	}
	if got.CurrentTitle != "Commissioner" {
		t.Errorf("CurrentTitle = %q, want %q (employment[0].Title)", got.CurrentTitle, "Commissioner")
	}
	if got.CurrentCompany != "NBA" {
		t.Errorf("CurrentCompany = %q, want %q (employment[0].Company)", got.CurrentCompany, "NBA")
	}
	if got.Quotes != "Long-tenured NBA executive who became commissioner in 2014." {
		t.Errorf("Quotes = %q, want the Summary text", got.Quotes)
	}
	// Score does not exist on the research surface — stays zero.
	if got.Score != 0 {
		t.Errorf("Score = %v, want 0 (research has no score)", got.Score)
	}
}

// TestToClientPersonFromResearch_EmptyEmployment covers the plan's edge
// case: Employment of length 0 must NOT panic (no out-of-bounds index)
// and must leave CurrentTitle/CurrentCompany empty.
func TestToClientPersonFromResearch_EmptyEmployment(t *testing.T) {
	in := ResearchProfile{
		Employment: nil, // length 0, the panicky case
		Summary:    "Newly graduated, no employment history yet.",
	}

	got := ToClientPersonFromResearch(in, "Jane Newgrad")

	if got.Name != "Jane Newgrad" {
		t.Errorf("Name = %q, want %q", got.Name, "Jane Newgrad")
	}
	if got.CurrentTitle != "" {
		t.Errorf("CurrentTitle = %q, want empty (no employment)", got.CurrentTitle)
	}
	if got.CurrentCompany != "" {
		t.Errorf("CurrentCompany = %q, want empty (no employment)", got.CurrentCompany)
	}
	if got.Quotes != "Newly graduated, no employment history yet." {
		t.Errorf("Quotes = %q, want the Summary text", got.Quotes)
	}
}

// TestToClientPersonFromResearch_EmptyEmployment_LiteralEmptySlice is a
// belt-and-suspenders pass for the same edge case using an explicit
// empty (non-nil) slice. Both a nil slice and an empty-but-non-nil slice
// must take the same branch — that is, neither must trigger an index op.
func TestToClientPersonFromResearch_EmptyEmployment_LiteralEmptySlice(t *testing.T) {
	in := ResearchProfile{
		Employment: []EmploymentEntry{}, // explicitly empty, not nil
		Summary:    "Same edge case, slice form.",
	}

	got := ToClientPersonFromResearch(in, "Jane Newgrad")

	if got.CurrentTitle != "" {
		t.Errorf("CurrentTitle = %q, want empty", got.CurrentTitle)
	}
	if got.CurrentCompany != "" {
		t.Errorf("CurrentCompany = %q, want empty", got.CurrentCompany)
	}
}

// TestToClientPerson_JSONSnapshotMatchesCookieShape is the integration
// scenario from the plan: a Person produced manually as if it came from
// the cookie path and the same Person produced via ToClientPerson must
// JSON-marshal to byte-identical output. This proves the renderer cannot
// distinguish the two sources by the canonical shape alone — a Source
// field, if anyone ever wants one, has to be a renderer concern, not a
// normalizer concern.
//
// We compare json.Marshal output (not reflect.DeepEqual) because JSON
// field ordering matters for downstream snapshot tests and pretty-print
// pipelines (jq, http response inspectors, etc.).
func TestToClientPerson_JSONSnapshotMatchesCookieShape(t *testing.T) {
	cookieShape := client.Person{
		Name:           "Adam Silver",
		CurrentTitle:   "Commissioner",
		CurrentCompany: "NBA",
		Score:          0.91,
	}

	bearerShape := ToClientPerson(SearchResult{
		Name:                "Adam Silver",
		CurrentTitle:        "Commissioner",
		CurrentCompany:      "NBA",
		WeightedTraitsScore: 0.91,
	})

	cookieJSON, err := json.Marshal(cookieShape)
	if err != nil {
		t.Fatalf("marshal cookie-shape Person: %v", err)
	}
	bearerJSON, err := json.Marshal(bearerShape)
	if err != nil {
		t.Fatalf("marshal bearer-shape Person: %v", err)
	}

	if !bytes.Equal(cookieJSON, bearerJSON) {
		t.Errorf("JSON byte snapshot differs between sources:\n cookie: %s\n bearer: %s",
			cookieJSON, bearerJSON)
	}
}
