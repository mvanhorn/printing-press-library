// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelAuditHelpWires smoke-tests that the audit command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelAuditHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"audit", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("audit --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "audit"} {
		if !strings.Contains(help, want) {
			t.Fatalf("audit --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestHasNoRealCategory(t *testing.T) {
	uncategorized := map[int64]struct{}{7: {}}
	tests := []struct {
		name       string
		categories []int64
		want       bool
	}{
		{name: "no categories", categories: []int64{}, want: true},
		{name: "only uncategorized", categories: []int64{7}, want: true},
		{name: "uncategorized and real", categories: []int64{7, 9}, want: false},
		{name: "one real category", categories: []int64{9}, want: false},
		{name: "unknown lone category is not assumed uncategorized", categories: []int64{99}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasNoRealCategory(tt.categories, uncategorized); got != tt.want {
				t.Fatalf("hasNoRealCategory() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAuditLocalReadsAreDrainFirstAndNullSafe(t *testing.T) {
	ctx, db := openLocalCommandTestStore(t)
	insertLocalCommandResource(t, ctx, db, "pages", "20", `{
		"id": 20,
		"title": {"rendered": "Sparse page"}
	}`)

	contents, err := loadAuditContent(ctx, db, []string{"pages"})
	if err != nil {
		t.Fatalf("loadAuditContent() error = %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("loadAuditContent() len = %d, want 1", len(contents))
	}
	if contents[0].FeaturedMedia.Valid || contents[0].CategoriesRaw != "[]" || contents[0].TagsRaw != "[]" {
		t.Fatalf("loadAuditContent() optional fields = %+v", contents[0])
	}
	if _, err := loadAuditMediaIDs(ctx, db); err != nil {
		t.Fatalf("loadAuditMediaIDs() after drained content error = %v", err)
	}
	if _, err := loadUncategorizedIDs(ctx, db); err != nil {
		t.Fatalf("loadUncategorizedIDs() after drained media error = %v", err)
	}
}
