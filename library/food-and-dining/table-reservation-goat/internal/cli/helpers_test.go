// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// helperFlags returns a rootFlags suitable for exercising printJSONFiltered
// in a unit test: --json on (so the writer-target check inside printOutput
// takes the encoder branch deterministically), no other format flags set.
func helperFlags(selectFields string) *rootFlags {
	return &rootFlags{
		asJSON:       true,
		selectFields: selectFields,
	}
}

// decodeMap pretty-prints + decodes printJSONFiltered output back into a
// map[string]any for field-level assertions. The output is a JSON-encoded
// json.RawMessage (double-encoded) when --json is set — the encoder writes
// the raw bytes as a string-quoted JSON literal because RawMessage
// implements MarshalJSON. Handle both shapes: first try direct map decode;
// if that fails, decode as a string then re-decode.
func decodeMap(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	trimmed := bytes.TrimSpace(raw)
	var m map[string]any
	if err := json.Unmarshal(trimmed, &m); err == nil {
		return m
	}
	t.Fatalf("decodeMap: not a JSON object: %s", string(raw))
	return nil
}

// TestPrintJSONFiltered_NoSelectIsIdentity is a regression guard: when
// selectFields is empty, printJSONFiltered must marshal the full input
// untouched (no safety-field reinjection short-circuit, no field
// dropping).
func TestPrintJSONFiltered_NoSelectIsIdentity(t *testing.T) {
	input := DisambiguationEnvelope{
		NeedsClarification: true,
		ErrorKind:          ErrorKindLocationAmbiguous,
		WhatWasAsked:       "bellevue",
		Candidates: []Candidate{
			{Name: "Bellevue, WA", State: "WA"},
		},
		AgentGuidance: AgentGuidance{
			PreferredRecovery: "Ask the user which Bellevue.",
			RerunPattern:      "<command> --location '<chosen-name>'",
		},
	}
	var buf bytes.Buffer
	if err := printJSONFiltered(&buf, input, helperFlags("")); err != nil {
		t.Fatalf("printJSONFiltered: %v", err)
	}
	got := decodeMap(t, buf.Bytes())

	for _, k := range []string{"needs_clarification", "error_kind", "what_was_asked", "candidates", "agent_guidance"} {
		if _, ok := got[k]; !ok {
			t.Errorf("identity: missing key %q in %v", k, got)
		}
	}
}

// TestPrintJSONFiltered_PreservesNeedsClarification pins that a
// --select pointing only at one envelope field still emits the full
// safety-field set (needs_clarification, error_kind, what_was_asked,
// candidates, agent_guidance). The agent's prompt may ask for one
// piece, but the envelope is load-bearing: without all five fields
// the agent loses the recovery contract.
func TestPrintJSONFiltered_PreservesNeedsClarification(t *testing.T) {
	input := DisambiguationEnvelope{
		NeedsClarification: true,
		ErrorKind:          ErrorKindLocationAmbiguous,
		WhatWasAsked:       "bellevue",
		Candidates: []Candidate{
			{Name: "Bellevue, WA", State: "WA"},
			{Name: "Bellevue, NE", State: "NE"},
		},
		AgentGuidance: AgentGuidance{
			PreferredRecovery: "Ask the user which Bellevue.",
			RerunPattern:      "<command> --location '<chosen-name>'",
		},
	}
	var buf bytes.Buffer
	if err := printJSONFiltered(&buf, input, helperFlags("needs_clarification")); err != nil {
		t.Fatalf("printJSONFiltered: %v", err)
	}
	got := decodeMap(t, buf.Bytes())

	for _, k := range []string{"needs_clarification", "error_kind", "what_was_asked", "candidates", "agent_guidance"} {
		if _, ok := got[k]; !ok {
			t.Errorf("safety preserve: missing key %q under --select needs_clarification; got %v", k, got)
		}
	}
	// And the value must be preserved verbatim, not blanked.
	if v, _ := got["needs_clarification"].(bool); !v {
		t.Errorf("needs_clarification = %v; want true", got["needs_clarification"])
	}
	if v, _ := got["error_kind"].(string); v != string(ErrorKindLocationAmbiguous) {
		t.Errorf("error_kind = %q; want %q", v, ErrorKindLocationAmbiguous)
	}
	if cands, ok := got["candidates"].([]any); !ok || len(cands) != 2 {
		t.Errorf("candidates = %v; want 2-element slice", got["candidates"])
	}
}

// TestPrintJSONFiltered_PreservesLocationWarning pins R-shape: a
// goatResponse with results + location_warning, where --select asks
// for results.name only. The filter must still apply to results
// (only the name field per element), but location_warning must
// survive in full.
func TestPrintJSONFiltered_PreservesLocationWarning(t *testing.T) {
	input := goatResponse{
		Query: "sushi",
		Results: []goatResult{
			{Name: "Sushi Kashiba", Network: "opentable"},
			{Name: "Shiro's Sushi", Network: "opentable"},
		},
		Sources: []string{"opentable", "tock"},
		LocationWarning: &LocationWarningField{
			Picked:     "Bellevue, WA",
			Alternates: []string{"Bellevue, NE", "Bellevue, KY"},
			Reason:     "medium_tier_ambiguous_pick",
		},
		QueriedAt: "2026-01-01T00:00:00Z",
	}
	var buf bytes.Buffer
	if err := printJSONFiltered(&buf, input, helperFlags("results.name")); err != nil {
		t.Fatalf("printJSONFiltered: %v", err)
	}
	got := decodeMap(t, buf.Bytes())

	// location_warning is fully preserved.
	lw, ok := got["location_warning"].(map[string]any)
	if !ok {
		t.Fatalf("location_warning missing or wrong shape; got %v", got["location_warning"])
	}
	if v, _ := lw["picked"].(string); v != "Bellevue, WA" {
		t.Errorf("location_warning.picked = %q; want %q", v, "Bellevue, WA")
	}
	if alts, _ := lw["alternates"].([]any); len(alts) != 2 {
		t.Errorf("location_warning.alternates len = %d; want 2", len(alts))
	}
	if v, _ := lw["reason"].(string); v != "medium_tier_ambiguous_pick" {
		t.Errorf("location_warning.reason = %q; want %q", v, "medium_tier_ambiguous_pick")
	}

	// --select results.name still applies: results carries name on each
	// element, but NOT network or other fields.
	results, ok := got["results"].([]any)
	if !ok || len(results) != 2 {
		t.Fatalf("results = %v; want 2-element slice", got["results"])
	}
	for i, r := range results {
		obj, ok := r.(map[string]any)
		if !ok {
			t.Fatalf("result[%d] not an object: %v", i, r)
		}
		if _, ok := obj["name"]; !ok {
			t.Errorf("result[%d].name missing; got %v", i, obj)
		}
		if _, ok := obj["network"]; ok {
			t.Errorf("result[%d].network should be filtered out; got %v", i, obj)
		}
	}

	// And query (top-level, not safety, not selected) should be filtered out.
	if _, ok := got["query"]; ok {
		t.Errorf("query should be filtered out under --select results.name; got %v", got["query"])
	}
}

// TestPrintJSONFiltered_PreservesLocationResolved mirrors the
// location_warning case for the location_resolved decorator: the
// resolved-to picker carries Source/Tier/Score that callers branch
// on, so it must survive --select results.name.
func TestPrintJSONFiltered_PreservesLocationResolved(t *testing.T) {
	input := goatResponse{
		Query: "sushi",
		Results: []goatResult{
			{Name: "Sushi Kashiba", Network: "opentable"},
		},
		Sources: []string{"opentable"},
		LocationResolved: &LocationResolvedField{
			Input:      "seattle",
			ResolvedTo: "Seattle, WA",
			Score:      0.95,
			Tier:       ResolutionTierHigh,
			Reason:     "single_canonical_match",
			Source:     SourceExplicitFlag,
		},
		QueriedAt: "2026-01-01T00:00:00Z",
	}
	var buf bytes.Buffer
	if err := printJSONFiltered(&buf, input, helperFlags("results.name")); err != nil {
		t.Fatalf("printJSONFiltered: %v", err)
	}
	got := decodeMap(t, buf.Bytes())

	lr, ok := got["location_resolved"].(map[string]any)
	if !ok {
		t.Fatalf("location_resolved missing or wrong shape; got %v", got["location_resolved"])
	}
	if v, _ := lr["resolved_to"].(string); v != "Seattle, WA" {
		t.Errorf("location_resolved.resolved_to = %q; want %q", v, "Seattle, WA")
	}
	if v, _ := lr["tier"].(string); v != string(ResolutionTierHigh) {
		t.Errorf("location_resolved.tier = %q; want %q", v, ResolutionTierHigh)
	}
}

// TestPrintJSONFiltered_NoSafetyFieldsNoChange pins that a payload
// without any safety fields routes through --select exactly as
// before — the safety-field preservation path adds no observable
// behavior change when the keys aren't present.
func TestPrintJSONFiltered_NoSafetyFieldsNoChange(t *testing.T) {
	input := map[string]any{
		"query": "sushi",
		"results": []map[string]any{
			{"name": "Sushi Kashiba", "network": "opentable", "city": "Seattle"},
			{"name": "Shiro's Sushi", "network": "opentable", "city": "Seattle"},
		},
		"sources_queried": []string{"opentable"},
	}
	var buf bytes.Buffer
	if err := printJSONFiltered(&buf, input, helperFlags("results.name")); err != nil {
		t.Fatalf("printJSONFiltered: %v", err)
	}
	got := decodeMap(t, buf.Bytes())

	// query and sources_queried should both be filtered out.
	if _, ok := got["query"]; ok {
		t.Errorf("query should be filtered out; got %v", got)
	}
	if _, ok := got["sources_queried"]; ok {
		t.Errorf("sources_queried should be filtered out; got %v", got)
	}

	results, ok := got["results"].([]any)
	if !ok || len(results) != 2 {
		t.Fatalf("results = %v; want 2-element slice", got["results"])
	}
	for i, r := range results {
		obj, _ := r.(map[string]any)
		if _, ok := obj["name"]; !ok {
			t.Errorf("result[%d].name missing", i)
		}
		if _, ok := obj["network"]; ok {
			t.Errorf("result[%d].network should be filtered; got %v", i, obj)
		}
	}

	// No safety fields should magically appear.
	for _, k := range []string{"needs_clarification", "error_kind", "location_warning", "location_resolved", "agent_guidance", "what_was_asked"} {
		if _, ok := got[k]; ok {
			t.Errorf("safety key %q must not appear when absent in input; got %v", k, got)
		}
	}
}

// TestPrintJSONFiltered_SafetyFieldsAreNotArrayElements pins that
// safety-field reinjection only fires on top-level objects. When the
// outer payload is an array (e.g., a bare list response), each element
// is filtered as today and no synthetic safety fields are merged in.
func TestPrintJSONFiltered_SafetyFieldsAreNotArrayElements(t *testing.T) {
	input := []map[string]any{
		{"name": "A", "extra": 1},
		{"name": "B", "extra": 2},
	}
	var buf bytes.Buffer
	if err := printJSONFiltered(&buf, input, helperFlags("name")); err != nil {
		t.Fatalf("printJSONFiltered: %v", err)
	}
	out := strings.TrimSpace(buf.String())
	var arr []map[string]any
	// The output may be double-encoded — printOutput wraps via the
	// encoder which writes RawMessage as a raw JSON token, so the
	// underlying payload comes out as an array directly.
	if err := json.Unmarshal([]byte(out), &arr); err != nil {
		t.Fatalf("expected JSON array; got %q: %v", out, err)
	}
	if len(arr) != 2 {
		t.Fatalf("array len = %d; want 2", len(arr))
	}
	for i, el := range arr {
		if _, ok := el["name"]; !ok {
			t.Errorf("arr[%d].name missing", i)
		}
		if _, ok := el["extra"]; ok {
			t.Errorf("arr[%d].extra should be filtered; got %v", i, el)
		}
	}
}
