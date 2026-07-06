// Copyright 2026 Ryan Gravette and contributors. Licensed under Apache-2.0. See LICENSE.
// Behavioral tests for Dawn concept parsing + aggregation (powers course stats/tree).

package cli

import (
	"encoding/json"
	"testing"
)

// synthetic concept fixture — no real data.
const sampleConcept = `{
  "id": "c_test",
  "title": "Test Course",
  "status": "enabled",
  "section": [
    {"id":"s1","title":"Sec One","status":"enabled","instruction":[
      {"id":"i1","title":"Intro","type":"video","duration":100,"points":5,"interaction":[{"id":"x1"},{"id":"x2"}]},
      {"id":"i2","title":"Quiz","type":"survey","duration":50,"points":0,"interaction":[{"id":"x3"}]}
    ]},
    {"id":"s2","title":"Sec Two","status":"enabled","instruction":[
      {"id":"i3","title":"Lesson","type":"activity","duration":200,"points":10,"interaction":[]}
    ]}
  ]
}`

func TestConceptParseAndAggregate(t *testing.T) {
	var c dawnConcept
	if err := json.Unmarshal([]byte(sampleConcept), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if c.ID != "c_test" || c.Title != "Test Course" {
		t.Fatalf("bad top-level parse: %+v", c)
	}
	if len(c.Section) != 2 {
		t.Fatalf("sections = %d, want 2", len(c.Section))
	}
	var instr, inter, points int
	var dur float64
	for _, s := range c.Section {
		instr += len(s.Instruction)
		for _, in := range s.Instruction {
			inter += in.interactionCount()
			points += in.Points
			dur += in.Duration
		}
	}
	if instr != 3 {
		t.Errorf("instructions = %d, want 3", instr)
	}
	if inter != 3 {
		t.Errorf("interactions = %d, want 3", inter)
	}
	if points != 15 {
		t.Errorf("points = %d, want 15", points)
	}
	if dur != 350 {
		t.Errorf("duration = %v, want 350", dur)
	}
}

func TestInteractionCount(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want int
	}{
		{"none", `{"id":"i","interaction":[]}`, 0},
		{"two", `{"id":"i","interaction":[{"id":"a"},{"id":"b"}]}`, 2},
		{"missing", `{"id":"i"}`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var in dawnInstruction
			if err := json.Unmarshal([]byte(tc.raw), &in); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got := in.interactionCount(); got != tc.want {
				t.Errorf("interactionCount() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestEnvelopeParse(t *testing.T) {
	raw := `{"totalMatches":2,"matches":[{"id":"a"},{"id":"b"}]}`
	var env dawnEnvelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.TotalMatches != 2 {
		t.Errorf("totalMatches = %d, want 2", env.TotalMatches)
	}
	if len(env.Matches) != 2 {
		t.Errorf("matches = %d, want 2", len(env.Matches))
	}
}
