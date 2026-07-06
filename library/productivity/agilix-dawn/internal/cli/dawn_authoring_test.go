// Copyright 2026 Ryan Gravette and contributors. Licensed under Apache-2.0. See LICENSE.
// Behavioral tests for the course-authoring map mutations (edit command group).

package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGenIDShape(t *testing.T) {
	id := genID("s_")
	if !strings.HasPrefix(id, "s_") {
		t.Fatalf("prefix missing: %s", id)
	}
	// s_ + 32 hex chars
	if len(id) != 2+32 {
		t.Fatalf("len = %d, want 34: %s", len(id), id)
	}
	if genID("s_") == id {
		t.Errorf("genID should not repeat")
	}
}

func TestSectionsOfAndAsMap(t *testing.T) {
	raw := `{"id":"c_x","section":[{"id":"s_1","title":"One","instruction":[]}]}`
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}
	secs := sectionsOf(m)
	if len(secs) != 1 {
		t.Fatalf("sections = %d, want 1", len(secs))
	}
	sm, ok := asMap(secs[0])
	if !ok || sm["id"] != "s_1" {
		t.Fatalf("asMap failed: %+v ok=%v", sm, ok)
	}
	// missing section key -> nil, not panic
	if sectionsOf(map[string]any{"id": "c_y"}) != nil {
		t.Errorf("expected nil for missing section")
	}
}

func TestSectionRemoveRoundTrip(t *testing.T) {
	raw := `{"id":"c_x","section":[
	  {"id":"s_1","title":"One"},
	  {"id":"s_2","title":"Two"},
	  {"id":"s_3","title":"Three"}]}`
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}
	// remove s_2 (mirrors the command's filter loop)
	kept := make([]any, 0)
	found := false
	for _, s := range sectionsOf(m) {
		if sm, ok := asMap(s); ok && sm["id"] == "s_2" {
			found = true
			continue
		}
		kept = append(kept, s)
	}
	if !found {
		t.Fatal("s_2 not found")
	}
	m["section"] = kept
	got := sectionsOf(m)
	if len(got) != 2 {
		t.Fatalf("after remove = %d, want 2", len(got))
	}
	for _, s := range got {
		if sm, _ := asMap(s); sm["id"] == "s_2" {
			t.Errorf("s_2 still present")
		}
	}
}

func TestFindInstruction(t *testing.T) {
	raw := `{"id":"c_x","section":[
	  {"id":"s_1","instruction":[{"id":"i_1","interaction":[]},{"id":"i_2","interaction":[]}]},
	  {"id":"s_2","instruction":[{"id":"i_3","interaction":[]}]}]}`
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}
	// found
	instr, err := findInstruction(m, "s_1", "i_2")
	if err != nil || instr["id"] != "i_2" {
		t.Fatalf("expected i_2, got %v err=%v", instr, err)
	}
	// mutating the returned map affects the original document
	instr["interaction"] = []any{map[string]any{"id": "q_1"}}
	if got := len(arrOf(m["section"].([]any)[0].(map[string]any)["instruction"].([]any)[1].(map[string]any)["interaction"])); got != 1 {
		t.Errorf("in-place mutation lost: %d", got)
	}
	// missing instruction
	if _, err := findInstruction(m, "s_1", "i_99"); err == nil {
		t.Error("expected error for missing instruction")
	}
	// missing section
	if _, err := findInstruction(m, "s_99", "i_1"); err == nil {
		t.Error("expected error for missing section")
	}
}

func TestPathEscape(t *testing.T) {
	cases := map[string]string{
		"c_abc123":  "c_abc123",
		"c_a/b":     "c_a%2Fb",
		"c_a b":     "c_a%20b",
		"c-x.y_z":   "c-x.y_z",
	}
	for in, want := range cases {
		if got := pathEscape(in); got != want {
			t.Errorf("pathEscape(%q) = %q, want %q", in, got, want)
		}
	}
}
