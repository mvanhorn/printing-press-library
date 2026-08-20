// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package valuation

import "testing"

func TestBySlug_FindsAtmos(t *testing.T) {
	def, ok := BySlug(ProgramAtmos)
	if !ok {
		t.Fatalf("BySlug(ProgramAtmos) returned ok=false; want true")
	}
	if def.Slug != ProgramAtmos {
		t.Errorf("Slug = %q; want %q", def.Slug, ProgramAtmos)
	}
	if def.FallbackCPP <= 0 {
		t.Errorf("FallbackCPP = %v; want > 0", def.FallbackCPP)
	}
	if def.TPGRowMatch == "" {
		t.Errorf("TPGRowMatch is empty")
	}
}

func TestBySlug_UnknownProgram(t *testing.T) {
	_, ok := BySlug(Program("united-mileageplus"))
	if ok {
		t.Errorf("BySlug for unregistered slug returned ok=true; want false")
	}
}

func TestSlugs_IncludesAtmos(t *testing.T) {
	found := false
	for _, s := range Slugs() {
		if s == ProgramAtmos {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Slugs() = %v; missing ProgramAtmos", Slugs())
	}
}
