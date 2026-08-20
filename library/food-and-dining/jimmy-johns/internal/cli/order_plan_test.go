package cli

import (
	"strings"
	"testing"
)

func TestComposePlan_DefaultCatalog_4People(t *testing.T) {
	plan := composePlan(4, nil, defaultCatalog)
	if plan.People != 4 {
		t.Fatalf("People = %d, want 4", plan.People)
	}
	// 4 * 1.2 = 4.8 -> ceil = 5 sandwiches across 3 varieties: 2,2,1
	var totalSandwich int
	for _, l := range plan.Lines {
		if !strings.Contains(strings.ToLower(l.Reason), "sandwich") {
			continue
		}
		totalSandwich += l.Quantity
	}
	if totalSandwich != 5 {
		t.Errorf("total sandwich qty = %d, want 5 (ceil(4*1.2))", totalSandwich)
	}
	if plan.EstSubtotal <= 0 {
		t.Errorf("EstSubtotal = %f, want > 0", plan.EstSubtotal)
	}
	if len(plan.NotesForAgent) == 0 {
		t.Errorf("expected placeholder-ID note for fallback catalog")
	}
}

func TestComposePlan_VegetarianFilter_DropsMeat(t *testing.T) {
	plan := composePlan(2, []string{"vegetarian"}, defaultCatalog)
	for _, l := range plan.Lines {
		if !strings.Contains(strings.ToLower(l.Reason), "sandwich") {
			continue
		}
		ln := strings.ToLower(l.Name)
		if strings.Contains(ln, "turkey") || strings.Contains(ln, "pepe") || strings.Contains(ln, "italian") || strings.Contains(ln, "vito") {
			t.Errorf("vegetarian plan contains meat sandwich %q", l.Name)
		}
	}
}

func TestComposePlan_NoMatchingSandwiches_EmitsAgentNote(t *testing.T) {
	// Catalog with only meat items + vegetarian filter should produce a note
	catalog := []planMenuItem{
		{ID: "x1", Name: "Big Beef Stack", Category: "sandwich", Price: 9.99},
	}
	plan := composePlan(3, []string{"vegetarian"}, catalog)
	if len(plan.NotesForAgent) == 0 {
		t.Fatalf("expected an agent note explaining no matches; got plan with %d lines", len(plan.Lines))
	}
	found := false
	for _, n := range plan.NotesForAgent {
		if strings.Contains(n, "no matching sandwiches") {
			found = true
		}
	}
	if !found {
		t.Errorf("notes don't explain the empty match: %v", plan.NotesForAgent)
	}
}
