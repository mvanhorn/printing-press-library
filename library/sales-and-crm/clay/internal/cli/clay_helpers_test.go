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
	got, unknown := resolveNamesToRefs("{{Company}} in {{City}}", byName)
	if len(unknown) != 0 {
		t.Fatalf("unexpected unknown refs: %v", unknown)
	}
	if got != "{{f_company}} in {{f_city}}" {
		t.Fatalf("resolveNamesToRefs = %q", got)
	}
	// Case-insensitive match.
	if got, _ := resolveNamesToRefs("{{company}}", byName); got != "{{f_company}}" {
		t.Fatalf("case-insensitive lookup failed: %q", got)
	}
	// Unknown names are reported, not silently written.
	if _, unknown := resolveNamesToRefs("{{Nope}}", byName); len(unknown) != 1 {
		t.Fatalf("expected 1 unknown ref, got %v", unknown)
	}
	// Already-id refs are left alone.
	if got, _ := resolveNamesToRefs("{{f_company}}", byName); got != "{{f_company}}" {
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
