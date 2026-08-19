// Copyright 2026 Ade Amos and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import "testing"

func testFields() []clayField {
	return []clayField{
		{ID: "f_company", Name: "Company", Type: "text"},
		{ID: "f_city", Name: "City", Type: "text"},
	}
}

func TestFormulaRefs(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{"none", "literal text", 0},
		{"one", "{{f_company}}", 1},
		{"two distinct", "{{f_company}} + {{f_city}}", 2},
		{"dedupes", "{{f_company}} + {{f_company}}", 1},
		{"tolerates spaces", "{{ f_company }}", 1},
		{"ignores named", "{{Company}}", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := len(formulaRefs(tc.in)); got != tc.want {
				t.Fatalf("formulaRefs(%q) = %d refs, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestResolveRefsToNames(t *testing.T) {
	byID := indexByID(testFields())
	got := resolveRefsToNames("{{f_company}} in {{f_city}}", byID)
	want := "{{Company}} in {{City}}"
	if got != want {
		t.Fatalf("resolveRefsToNames = %q, want %q", got, want)
	}
	// Unknown ids must be preserved verbatim rather than silently dropped.
	if got := resolveRefsToNames("{{f_missing}}", byID); got != "{{f_missing}}" {
		t.Fatalf("unknown ref rewritten to %q", got)
	}
}

func TestResolveNamesToRefs(t *testing.T) {
	byName := indexByName(testFields())
	byID := indexByID(testFields())
	got, unknown, _ := resolveNamesToRefs("{{Company}} in {{City}}", byName, byID)
	if len(unknown) != 0 {
		t.Fatalf("unexpected unknown refs: %v", unknown)
	}
	if got != "{{f_company}} in {{f_city}}" {
		t.Fatalf("resolveNamesToRefs = %q", got)
	}
	// Case-insensitive match.
	if got, _, _ := resolveNamesToRefs("{{company}}", byName, byID); got != "{{f_company}}" {
		t.Fatalf("case-insensitive lookup failed: %q", got)
	}
	// Unknown names are reported, not silently written.
	if _, unknown, _ := resolveNamesToRefs("{{Nope}}", byName, byID); len(unknown) != 1 {
		t.Fatalf("expected 1 unknown ref, got %v", unknown)
	}
	// Already-id refs are left alone.
	if got, _, _ := resolveNamesToRefs("{{f_company}}", byName, byID); got != "{{f_company}}" {
		t.Fatalf("id ref rewritten: %q", got)
	}
}

func TestOrderByDependency(t *testing.T) {
	cols := []blueprintColumn{
		{Name: "Derived", Type: "formula", DependsOn: []string{"Base"}},
		{Name: "Base", Type: "text"},
	}
	ordered, leftover := orderByDependency(cols)
	if len(leftover) != 0 {
		t.Fatalf("unexpected leftover: %v", leftover)
	}
	if len(ordered) != 2 || ordered[0].Name != "Base" {
		t.Fatalf("dependency order wrong: %+v", ordered)
	}
}

func TestOrderByDependencyDetectsCycle(t *testing.T) {
	cols := []blueprintColumn{
		{Name: "A", Type: "formula", DependsOn: []string{"B"}},
		{Name: "B", Type: "formula", DependsOn: []string{"A"}},
	}
	ordered, leftover := orderByDependency(cols)
	if len(ordered) != 0 {
		t.Fatalf("cycle should place nothing, got %+v", ordered)
	}
	if len(leftover) != 2 {
		t.Fatalf("cycle should report both columns, got %v", leftover)
	}
}

func TestIsSystemField(t *testing.T) {
	if !isSystemField("f_created_at") {
		t.Fatal("f_created_at should be a system field")
	}
	if isSystemField("f_company") {
		t.Fatal("f_company should not be a system field")
	}
}

func TestRemapNamedRefs(t *testing.T) {
	nameToID := map[string]string{"company": "f_new1"}
	got, unknown := remapNamedRefs("{{Company}}", nameToID)
	if got != "{{f_new1}}" || len(unknown) != 0 {
		t.Fatalf("remapNamedRefs = %q unknown=%v", got, unknown)
	}
	if _, unknown := remapNamedRefs("{{Ghost}}", nameToID); len(unknown) != 1 {
		t.Fatalf("expected unknown ref for {{Ghost}}")
	}
}

// A column may legitimately be NAMED with an "f_" prefix. The old precedence
// treated any such token as an already-resolved field id and skipped the
// remap, which silently broke formulas on blueprint apply.
func TestResolveNamesToRefsPrefersNameOverFieldIDShape(t *testing.T) {
	fields := []clayField{
		{ID: "f_realid1", Name: "f_lookalike"},
		{ID: "f_realid2", Name: "Company"},
	}
	byName := indexByName(fields)

	byID := indexByID(fields)

	// f_lookalike is only a column NAME here (not a real field id), so it must
	// still resolve by name.
	got, unknown, _ := resolveNamesToRefs("{{f_lookalike}}", byName, byID)
	if len(unknown) != 0 {
		t.Fatalf("unexpected unknown refs: %v", unknown)
	}
	if got != "{{f_realid1}}" {
		t.Fatalf("column named f_lookalike was not remapped: got %q, want {{f_realid1}}", got)
	}

	// A real id that is not a column name passes through untouched.
	if got, _, _ := resolveNamesToRefs("{{f_realid2}}", byName, byID); got != "{{f_realid2}}" {
		t.Fatalf("actual field id should pass through, got %q", got)
	}

	// A non-id, non-column token is still reported as unresolved.
	if _, unknown, _ := resolveNamesToRefs("{{Ghost}}", byName, byID); len(unknown) != 1 {
		t.Fatalf("expected Ghost to be unresolved, got %v", unknown)
	}
}

// An explicit, real field id must bind to that field even when a DIFFERENT
// column is literally named like it — and the collision must be reported.
func TestResolveNamesToRefsExplicitFieldIDWins(t *testing.T) {
	fields := []clayField{
		{ID: "f_company", Name: "Company"},
		{ID: "f_other", Name: "f_company"}, // a column NAMED like the id above
	}
	byName := indexByName(fields)
	byID := indexByID(fields)

	got, unknown, ambiguous := resolveNamesToRefs("{{f_company}}", byName, byID)
	if len(unknown) != 0 {
		t.Fatalf("unexpected unknown refs: %v", unknown)
	}
	if got != "{{f_company}}" {
		t.Fatalf("explicit field id must bind to itself, got %q (would have read the wrong column)", got)
	}
	if len(ambiguous) != 1 || ambiguous[0] != "f_company" {
		t.Fatalf("collision should be reported as ambiguous, got %v", ambiguous)
	}

	// Ordinary names are unaffected.
	if got, _, _ := resolveNamesToRefs("{{Company}}", byName, byID); got != "{{f_company}}" {
		t.Fatalf("ordinary name remap broke, got %q", got)
	}
}

func TestRemapNamedRefsPrefersNameOverFieldIDShape(t *testing.T) {
	nameToID := map[string]string{
		"f_lookalike": "f_newid1",
		"company":     "f_newid2",
	}

	got, unknown := remapNamedRefs("{{f_lookalike}}", nameToID)
	if len(unknown) != 0 {
		t.Fatalf("unexpected unknown refs: %v", unknown)
	}
	if got != "{{f_newid1}}" {
		t.Fatalf("blueprint column named f_lookalike was not remapped: got %q, want {{f_newid1}}", got)
	}

	// An id-shaped token that is not a blueprint column name passes through.
	if got, _ := remapNamedRefs("{{f_untouched}}", nameToID); got != "{{f_untouched}}" {
		t.Fatalf("id-shaped token should pass through, got %q", got)
	}

	// Ordinary names still remap.
	if got, _ := remapNamedRefs("{{Company}}", nameToID); got != "{{f_newid2}}" {
		t.Fatalf("ordinary name remap broke, got %q", got)
	}
}

func TestFieldIDPatternShape(t *testing.T) {
	for _, ok := range []string{"f_abc123", "f_0tjzmr4xFvXTHXoNU7q", "f_created_at"} {
		if !fieldIDPattern.MatchString(ok) {
			t.Fatalf("%q should match the field-id shape", ok)
		}
	}
	for _, bad := range []string{"f_", "Company", "f lookalike", "gv_abc", "f_has-dash"} {
		if fieldIDPattern.MatchString(bad) {
			t.Fatalf("%q should NOT match the field-id shape", bad)
		}
	}
}

// Read and write must agree. If a column's NAME is also another column's real
// field id, rendering that name would rebind the formula on the next save, so
// the read path must keep the raw id instead.
func TestFormulaRoundTripDoesNotRebind(t *testing.T) {
	fields := []clayField{
		{ID: "f_realid", Name: "f_other"}, // name collides with the id below
		{ID: "f_other", Name: "Company"},
	}
	byID := indexByID(fields)
	byName := indexByName(fields)

	rendered := resolveRefsToNames("{{f_realid}}", byID)
	if rendered != "{{f_realid}}" {
		t.Fatalf("read rendered a colliding name %q; a save would rebind it", rendered)
	}

	// Round trip must land on the same field it started from.
	written, unknown, _ := resolveNamesToRefs(rendered, byName, byID)
	if len(unknown) != 0 {
		t.Fatalf("unexpected unknown refs: %v", unknown)
	}
	if written != "{{f_realid}}" {
		t.Fatalf("round trip rebound %q -> %q", rendered, written)
	}

	// A non-colliding name still renders for readability.
	if got := resolveRefsToNames("{{f_other}}", byID); got != "{{Company}}" {
		t.Fatalf("non-colliding name should render, got %q", got)
	}
}
