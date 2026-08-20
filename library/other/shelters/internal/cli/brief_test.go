// Copyright 2026 Abe Diaz (@abe238) and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

func TestBuildBriefCounts(t *testing.T) {
	shelters := parseFixture(t, syntheticFixture)
	d := buildBrief(shelters)

	checks := map[string][2]int{
		"open":       {d.OpenCount, 12},
		"pets":       {d.PetFriendlyCount, 7},
		"ada":        {d.ADACount, 6},
		"wheelchair": {d.WheelchairCount, 7},
		"computable": {d.CapacityComputable, 10},
		"atCapacity": {d.AtCapacityCount, 2},
	}
	for name, c := range checks {
		if c[0] != c[1] {
			t.Errorf("brief %s = %d, want %d", name, c[0], c[1])
		}
	}
	if d.ByState["TX"] == 0 || d.ByState["LA"] == 0 {
		t.Errorf("by_state missing TX/LA: %+v", d.ByState)
	}
	if d.Summary == "" {
		t.Error("summary is empty")
	}
}

// TestBriefMarkdownHasGratitudeFooter: the user-requested footer must be present.
func TestBriefMarkdownHasGratitudeFooter(t *testing.T) {
	shelters := parseFixture(t, syntheticFixture)
	md := renderBriefMarkdown(buildBrief(shelters))
	for _, want := range []string{
		"# Shelter Situational Briefing",
		"first responders",
		"emergency management",
		"relief nonprofit",
		"call 911",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("brief markdown missing %q", want)
		}
	}
}

// TestBriefQuietState: zero shelters must not panic or fabricate.
func TestBriefQuietState(t *testing.T) {
	d := buildBrief(nil)
	if d.OpenCount != 0 || d.PetFriendlyCount != 0 || d.AtCapacityCount != 0 {
		t.Errorf("empty brief should be all zero: %+v", d)
	}
	md := renderBriefMarkdown(d)
	if !strings.Contains(md, "No open shelters") {
		t.Error("quiet-state markdown should say no open shelters")
	}
}
