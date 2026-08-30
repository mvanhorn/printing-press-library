// Copyright 2026 RyanGravetteIDLA and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelGroupsExpandHelpWires smoke-tests that the groups expand command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelGroupsExpandHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"groups", "expand", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("groups expand --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "expand"} {
		if !strings.Contains(help, want) {
			t.Fatalf("groups expand --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestExpandGroupFlattensNestedCycleSafe(t *testing.T) {
	// all-staff -> [alice, teachers(group)]; teachers -> [bob, all-staff(cycle)]
	graph := map[string][]directoryMember{
		"all-staff@example.com": {
			{Email: "alice@example.com", Type: "USER"},
			{Email: "teachers@example.com", Type: "GROUP"},
		},
		"teachers@example.com": {
			{Email: "bob@example.com", Type: "USER"},
			{Email: "all-staff@example.com", Type: "GROUP"}, // cycle back to root
		},
	}
	fetch := func(g string) ([]directoryMember, error) { return graph[g], nil }

	view, err := expandGroup("all-staff@example.com", fetch, 0, 0, 500)
	if err != nil {
		t.Fatalf("expandGroup error = %v", err)
	}
	// Both distinct users resolved exactly once.
	if len(view.Members) != 2 {
		t.Fatalf("want 2 effective members, got %d: %+v", len(view.Members), view.Members)
	}
	byEmail := map[string]effectiveMember{}
	for _, m := range view.Members {
		byEmail[m.Email] = m
	}
	if _, ok := byEmail["alice@example.com"]; !ok {
		t.Error("alice should be a direct member of all-staff")
	}
	if bob, ok := byEmail["bob@example.com"]; !ok {
		t.Error("bob should be resolved via the nested teachers group")
	} else if bob.ViaGroup != "teachers@example.com" {
		t.Errorf("bob.viaGroup = %q, want teachers@example.com", bob.ViaGroup)
	}
	// The cycle back to all-staff must be recorded, not re-expanded.
	if len(view.CyclesSkipped) == 0 {
		t.Error("expected the all-staff cycle to be recorded in CyclesSkipped")
	}
	if view.ScannedGroups != 2 {
		t.Errorf("ScannedGroups = %d, want 2 (root + teachers, cycle not re-scanned)", view.ScannedGroups)
	}
}

func TestExpandGroupDeduplicatesUsers(t *testing.T) {
	// carol is in both nested groups; first-parent-wins keeps a single entry.
	graph := map[string][]directoryMember{
		"root@example.com": {
			{Email: "a@example.com", Type: "GROUP"},
			{Email: "b@example.com", Type: "GROUP"},
		},
		"a@example.com": {{Email: "carol@example.com", Type: "USER"}},
		"b@example.com": {{Email: "carol@example.com", Type: "USER"}},
	}
	fetch := func(g string) ([]directoryMember, error) { return graph[g], nil }
	view, err := expandGroup("root@example.com", fetch, 0, 0, 500)
	if err != nil {
		t.Fatalf("expandGroup error = %v", err)
	}
	if len(view.Members) != 1 || view.Members[0].Email != "carol@example.com" {
		t.Fatalf("want single carol entry, got %+v", view.Members)
	}
}
