// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func sampleTaggedDesigns() []taggedDesign {
	return []taggedDesign{
		{ID: "1", Title: "Fidget Toy", Downloads: 100, Tags: []string{"Toy", "fidget", "cute"}},
		{ID: "2", Title: "Toy Box", Downloads: 300, Tags: []string{"toy", "storage"}},
		{ID: "3", Title: "Spinner", Downloads: 200, Tags: []string{"FIDGET", "toy", "desk"}},
		{ID: "4", Title: "Vase", Downloads: 50, Tags: []string{"decor"}},
	}
}

func TestAggregateTags(t *testing.T) {
	got := aggregateTags(sampleTaggedDesigns(), 1, 10)
	// toy appears in 3 (case-insensitive merge), fidget in 2.
	idx := map[string]int{}
	for _, c := range got {
		idx[c.Tag] = c.Count
	}
	if idx["toy"] != 3 {
		t.Errorf("toy count = %d want 3 (case-insensitive merge)", idx["toy"])
	}
	if idx["fidget"] != 2 {
		t.Errorf("fidget count = %d want 2", idx["fidget"])
	}
	if got[0].Tag != "toy" {
		t.Errorf("top tag = %q want toy", got[0].Tag)
	}
}

func TestAggregateTagsMinCount(t *testing.T) {
	got := aggregateTags(sampleTaggedDesigns(), 2, 10)
	for _, c := range got {
		if c.Count < 2 {
			t.Errorf("tag %q count %d below min-count 2", c.Tag, c.Count)
		}
	}
	// only toy(3) and fidget(2) qualify
	if len(got) != 2 {
		t.Errorf("min-count 2 returned %d tags want 2: %+v", len(got), got)
	}
}

func TestMatchAllTags(t *testing.T) {
	got := matchAllTags(sampleTaggedDesigns(), []string{"toy", "fidget"}, 10)
	// designs 1 and 3 have both toy+fidget; ranked by downloads => 3 (200) before 1 (100).
	if len(got) != 2 {
		t.Fatalf("got %d matches want 2: %+v", len(got), got)
	}
	if got[0].ID != "3" || got[1].ID != "1" {
		t.Errorf("order = %s,%s want 3,1 (downloads desc)", got[0].ID, got[1].ID)
	}
}

func TestMatchAllTagsNoMatch(t *testing.T) {
	got := matchAllTags(sampleTaggedDesigns(), []string{"toy", "decor"}, 10)
	if len(got) != 0 {
		t.Errorf("no design has both toy+decor, got %d", len(got))
	}
}

func TestMatchAllTagsWhitespaceArgs(t *testing.T) {
	// All-whitespace args must not vacuously match every design.
	got := matchAllTags(sampleTaggedDesigns(), []string{"  ", "\t"}, 10)
	if len(got) != 0 {
		t.Errorf("all-whitespace tags should match nothing, got %d", len(got))
	}
}
