// Copyright 2026 Justin Fu and contributors. Licensed under Apache-2.0. See LICENSE.

package refdata

import (
	"strings"
	"testing"
)

// TestSCAWheelStructure verifies the curated wheel has the expected
// Level-1 categories and at least one Level-2 child per category.
// SCA wheel reference: sca.coffee.
func TestSCAWheelStructure(t *testing.T) {
	if len(SCAWheel) == 0 {
		t.Fatal("SCAWheel is empty")
	}
	// The official SCA Coffee Tasters' Flavor Wheel has 9 Level-1
	// categories. Allow ±2 for the curated-subset variant.
	if len(SCAWheel) < 7 || len(SCAWheel) > 11 {
		t.Errorf("expected 7-11 Level-1 categories, got %d", len(SCAWheel))
	}

	// Spot-check that three iconic Level-1 categories are present.
	required := []string{"Fruity", "Floral", "Sweet"}
	for _, want := range required {
		if !findNode(SCAWheel, want) {
			t.Errorf("missing Level-1 category %q", want)
		}
	}

	// Every Level-1 must have at least one child (Level-2).
	for _, l1 := range SCAWheel {
		if len(l1.Children) == 0 {
			t.Errorf("Level-1 category %q has no Level-2 children", l1.Name)
		}
	}
}

// TestFlattenSectionsHappyPath verifies FlattenSections walks the
// wheel and emits sections.
func TestFlattenSectionsHappyPath(t *testing.T) {
	sections := FlattenSections()
	if len(sections) == 0 {
		t.Fatal("FlattenSections returned 0 sections")
	}

	// Expect at least one section path containing "Blackberry" since
	// the wheel includes Fruity > Berry > Blackberry.
	found := false
	for _, s := range sections {
		if strings.EqualFold(s.Leaf, "blackberry") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected at least one section path to mention 'blackberry'")
	}

	// Every section must have at least Top populated — used for
	// agent-facing output in flavor-wheel.
	for i, s := range sections {
		if s.Top == "" {
			t.Errorf("section[%d] has empty Top", i)
		}
	}
}

func findNode(nodes []FlavorNode, name string) bool {
	for _, n := range nodes {
		if n.Name == name {
			return true
		}
	}
	return false
}
