// Copyright 2026 HenryBranchAdams and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/name-that-ui/internal/namethatui"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/name-that-ui/internal/store"
)

// TestNovelCrosswalkHelpWires smoke-tests that the crosswalk command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelCrosswalkHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"crosswalk", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("crosswalk --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "crosswalk"} {
		if !strings.Contains(help, want) {
			t.Fatalf("crosswalk --help missing %q in output:\n%s", want, help)
		}
	}
}

func seedCrosswalkDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "crosswalk.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	components := []namethatui.Component{
		{ID: "mac/menu-button", Platform: "mac", Slug: "menu-button", Name: "Menu Button", AKA: []string{"overflow menu"}, Fuzzy: []string{"contextual actions"}, API: []namethatui.API{{Framework: "AppKit", Symbol: "NSMenuItem"}, {Framework: "SwiftUI", Symbol: "Menu"}, {Framework: "ARIA", Symbol: "menuitem"}, {Framework: "HTML", Symbol: "button"}, {Framework: "React", Symbol: "MenuButton"}}, Parts: []namethatui.Part{{ID: "item", Name: "Menu Item", API: "NSMenuItem", Description: "One item in the menu."}}, SourceURL: "https://example.test/menu-button"},
		{ID: "web/action-menu", Platform: "web", Slug: "action-menu", Name: "Action Menu", AKA: []string{"overflow menu"}, SourceURL: "https://example.test/action-menu"},
	}
	for _, component := range components {
		raw, _ := json.Marshal(component)
		if err := db.Upsert("components", component.ID, raw); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.SaveSyncState("components", "", len(components)); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func runCrosswalk(t *testing.T, db string, args ...string) (map[string]any, error) {
	t.Helper()
	var flags rootFlags
	root := newRootCmd(&flags)
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs(append([]string{"--json", "--no-learn", "crosswalk", "--db", db}, args...))
	err := root.Execute()
	result := map[string]any{}
	if out.Len() > 0 {
		if decodeErr := json.Unmarshal(out.Bytes(), &result); decodeErr != nil {
			t.Fatalf("invalid JSON %q: %v", out.String(), decodeErr)
		}
	}
	return result, err
}

func TestCrosswalkMatchesExactFuzzyAPIAndParts(t *testing.T) {
	db := seedCrosswalkDB(t)
	for _, tc := range []struct {
		concept string
		field   string
		value   string
	}{
		{"menu button", "component_name", "Menu Button"},
		{"contextual action", "component_fuzzy", "contextual actions"},
		{"NSMenuItem", "api_symbol", "NSMenuItem"},
		{"menu item", "part_name", "Menu Item"},
	} {
		t.Run(tc.concept, func(t *testing.T) {
			got, err := runCrosswalk(t, db, tc.concept)
			if err != nil {
				t.Fatal(err)
			}
			candidates := got["candidates"].([]any)
			if len(candidates) == 0 || !hasEvidence(candidates, tc.field, tc.value) {
				t.Fatalf("%q candidates = %#v", tc.concept, candidates)
			}
		})
	}
}

func TestCrosswalkMatrixSourceLinksAmbiguityAndEmptyArrays(t *testing.T) {
	db := seedCrosswalkDB(t)
	got, err := runCrosswalk(t, db, "NSMenuItem")
	if err != nil {
		t.Fatal(err)
	}
	matrix := got["matrix"].([]any)
	if len(matrix) == 0 {
		t.Fatal("expected matrix row")
	}
	row := matrix[0].(map[string]any)
	if !containsString(row["appkit"].([]any), "NSMenuItem") || !containsString(row["swiftui"].([]any), "Menu") || !containsString(row["aria"].([]any), "menuitem") || !containsString(row["html"].([]any), "button") {
		t.Fatalf("matrix = %#v", row)
	}
	if other := row["other"].([]any); len(other) != 1 || other[0].(map[string]any)["framework"] != "React" {
		t.Fatalf("other framework terms = %#v", other)
	}
	if !containsString(got["source_urls"].([]any), "https://example.test/menu-button") {
		t.Fatalf("source urls = %#v", got["source_urls"])
	}
	got, err = runCrosswalk(t, db, "overflow menu")
	if err != nil {
		t.Fatal(err)
	}
	if got["ambiguous"] != true || len(got["candidates"].([]any)) != 2 {
		t.Fatalf("ambiguity = %#v", got)
	}
	got, err = runCrosswalk(t, db, "unmatched")
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"candidates", "matrix", "source_urls"} {
		if values, ok := got[field].([]any); !ok || values == nil || len(values) != 0 {
			t.Fatalf("%s must be []: %#v", field, got[field])
		}
	}
}

func TestCrosswalkDryRunAndMissingMirror(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.db")
	_, err := runCrosswalk(t, missing, "menu")
	if err == nil || !strings.Contains(err.Error(), "sync --resources catalog") {
		t.Fatalf("missing mirror error = %v", err)
	}
	got, err := runCrosswalk(t, missing, "menu", "--dry-run")
	if err != nil || got["dry_run"] != true || got["sqlite_opened"] != false {
		t.Fatalf("dry run = %#v, %v", got, err)
	}
	for _, field := range []string{"candidates", "matrix", "source_urls"} {
		if values, ok := got[field].([]any); !ok || values == nil {
			t.Fatalf("dry-run %s must be []: %#v", field, got[field])
		}
	}
}

func hasEvidence(candidates []any, field, value string) bool {
	for _, raw := range candidates {
		for _, evidence := range raw.(map[string]any)["evidence"].([]any) {
			item := evidence.(map[string]any)
			if item["field"] == field && item["value"] == value {
				return true
			}
		}
	}
	return false
}
