// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package envelope

import (
	"encoding/json"
	"strings"
	"testing"
)

type testEnvelope struct {
	Results       []json.RawMessage `json:"results"`
	Count         int               `json:"count"`
	ReturnedCount int               `json:"returned_count"`
	Truncated     bool              `json:"truncated"`
	NextOffset    *int              `json:"next_offset,omitempty"`
	MaxBytes      int               `json:"max_bytes"`
	Note          string            `json:"note,omitempty"`
}

func TestListSmallStillEmitsEnvelope(t *testing.T) {
	got := List(rawItems(`{"id":1}`), Options{Count: 1, MaxBytes: 32000, SupportsCursor: true})
	env := parseEnvelope(t, got)
	if env.Count != 1 || env.ReturnedCount != 1 || len(env.Results) != 1 || env.Truncated {
		t.Fatalf("small envelope = %+v", env)
	}
	if env.NextOffset != nil || env.Note != "" {
		t.Fatalf("small envelope should omit cursor/note: %+v", env)
	}
}

func TestListLargeTruncatesWithinBudget(t *testing.T) {
	items := make([]json.RawMessage, 0, 10)
	for i := 0; i < 10; i++ {
		items = append(items, json.RawMessage(`{"text":"`+strings.Repeat("x", 500)+`"}`))
	}
	got := List(items, Options{Count: len(items), MaxBytes: 1200, SupportsCursor: true})
	if len(got) > 1200 {
		t.Fatalf("len = %d, want <= 1200", len(got))
	}
	env := parseEnvelope(t, got)
	if !env.Truncated || env.NextOffset == nil || *env.NextOffset != env.ReturnedCount {
		t.Fatalf("large envelope cursor/truncation wrong: %+v", env)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(got, &raw); err != nil {
		t.Fatalf("unmarshal raw envelope: %v", err)
	}
	if _, ok := raw["original_bytes"]; ok {
		t.Fatalf("list envelope should not include original_bytes: %s", got)
	}
}

func TestListLargeWithoutCursorOmitsNextOffset(t *testing.T) {
	items := make([]json.RawMessage, 0, 10)
	for i := 0; i < 10; i++ {
		items = append(items, json.RawMessage(`{"text":"`+strings.Repeat("x", 500)+`"}`))
	}
	got := List(items, Options{Count: len(items), MaxBytes: 1200, SupportsCursor: false})
	env := parseEnvelope(t, got)
	if !env.Truncated {
		t.Fatalf("large envelope should truncate: %+v", env)
	}
	if env.NextOffset != nil {
		t.Fatalf("non-cursor envelope should omit next_offset, got %d", *env.NextOffset)
	}
}

func TestObjectArrayPreservesSiblingsAndPagesArray(t *testing.T) {
	obj := map[string]json.RawMessage{
		"totals":      json.RawMessage(`{"all":4}`),
		"since":       json.RawMessage(`"2026-06-01"`),
		"by_location": json.RawMessage(`[]`),
	}
	items := make([]json.RawMessage, 0, 4)
	for i := 0; i < 4; i++ {
		items = append(items, json.RawMessage(`{"text":"`+strings.Repeat("x", 500)+`"}`))
	}
	got := ObjectArray(obj, "by_location", items, Options{Count: len(items), MaxBytes: 1200, SupportsCursor: true})
	if len(got) > 1200 {
		t.Fatalf("len = %d, want <= 1200", len(got))
	}
	var env struct {
		Totals        map[string]int    `json:"totals"`
		Since         string            `json:"since"`
		ByLocation    []json.RawMessage `json:"by_location"`
		Results       []json.RawMessage `json:"results"`
		ReturnedCount int               `json:"returned_count"`
		Truncated     bool              `json:"truncated"`
		NextOffset    *int              `json:"next_offset"`
	}
	if err := json.Unmarshal(got, &env); err != nil {
		t.Fatalf("unmarshal object envelope: %v\n%s", err, got)
	}
	if env.Totals["all"] != 4 || env.Since != "2026-06-01" {
		t.Fatalf("siblings not preserved: %+v", env)
	}
	if len(env.Results) != 0 {
		t.Fatalf("object array envelope flattened to results: %+v", env)
	}
	if !env.Truncated || env.NextOffset == nil || env.ReturnedCount != len(env.ByLocation) {
		t.Fatalf("object array pagination metadata wrong: %+v", env)
	}
}

func TestListSingleOversizedItem(t *testing.T) {
	got := List(rawItems(`{"text":"`+strings.Repeat("z", 2000)+`"}`), Options{Count: 1, MaxBytes: 400, SupportsCursor: true})
	if len(got) > 400 {
		t.Fatalf("len = %d, want <= 400", len(got))
	}
	env := parseEnvelope(t, got)
	if !env.Truncated || env.ReturnedCount != 0 || len(env.Results) != 0 || env.NextOffset == nil || *env.NextOffset != 0 {
		t.Fatalf("single oversized envelope = %+v", env)
	}
}

func TestListEmptyResultsAreArray(t *testing.T) {
	got := List(nil, Options{Count: 0, MaxBytes: 32000})
	env := parseEnvelope(t, got)
	if env.Results == nil || len(env.Results) != 0 || env.Count != 0 || env.ReturnedCount != 0 {
		t.Fatalf("empty envelope = %+v raw=%s", env, got)
	}
	if strings.Contains(string(got), `"results":null`) {
		t.Fatalf("empty results encoded as null: %s", got)
	}
}

func TestMaxBytesDefaultOverrideAndInvalid(t *testing.T) {
	t.Setenv("PP_MCP_MAX_BYTES", "")
	if got := MaxBytes(); got != 32000 {
		t.Fatalf("MaxBytes default = %d, want 32000", got)
	}
	t.Setenv("PP_MCP_MAX_BYTES", "5000")
	if got := MaxBytes(); got != 5000 {
		t.Fatalf("MaxBytes override = %d, want 5000", got)
	}
	t.Setenv("PP_MCP_MAX_BYTES", "garbage")
	if got := MaxBytes(); got != 32000 {
		t.Fatalf("MaxBytes garbage = %d, want 32000", got)
	}
	t.Setenv("PP_MCP_MAX_BYTES", "0")
	if got := MaxBytes(); got != 32000 {
		t.Fatalf("MaxBytes zero = %d, want 32000", got)
	}
}

func parseEnvelope(t *testing.T, data []byte) testEnvelope {
	t.Helper()
	var env testEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("unmarshal envelope: %v\n%s", err, data)
	}
	return env
}

func rawItems(items ...string) []json.RawMessage {
	out := make([]json.RawMessage, 0, len(items))
	for _, item := range items {
		out = append(out, json.RawMessage(item))
	}
	return out
}

// Compare sizing and search with the previous exhaustive renderer, including
// budgets where removing the truncation note makes the complete page cheaper.
func TestE1MeasuredEnvelopeMatchesLinear(t *testing.T) {
	items := rawItems(`{"id":1}`, ` { "id": 2, "text":"<tag>" } `, `{"id":3}`, `{"id":4}`)
	for budget := 100; budget < 1200; budget++ {
		opts := Options{Count: len(items), MaxBytes: budget, SupportsCursor: true, Note: defaultTruncationNote}
		for _, field := range []string{"results", "clusters"} {
			obj := map[string]json.RawMessage{"sibling": json.RawMessage(`"kept"`)}
			render := func(page []json.RawMessage) map[string]any {
				if field == "results" {
					return build(page, opts)
				}
				return buildObject(obj, field, page, opts)
			}
			var want []byte
			for n := len(items); n >= 0; n-- {
				raw, err := json.Marshal(render(items[:n]))
				if err != nil {
					t.Fatal(err)
				}
				if len(raw) <= budget || n == 0 {
					want = raw
					break
				}
			}
			var got []byte
			if field == "results" {
				got = List(items, opts)
			} else {
				got = ObjectArray(obj, field, items, opts)
			}
			if string(got) != string(want) {
				t.Fatalf("size search differs from exhaustive renderer: field=%s budget=%d got=%s want=%s", field, budget, got, want)
			}
		}
	}
}
