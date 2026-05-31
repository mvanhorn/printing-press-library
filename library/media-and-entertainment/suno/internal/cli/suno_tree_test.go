// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"strings"
	"testing"
)

func TestTree_ReconstructsParentChild(t *testing.T) {
	seedClips(t, []map[string]any{
		{"id": "root", "title": "Root"},
		{"id": "child", "title": "Child", "parent_clip_id": "root"},
	})
	cmd := newTreeCmd(&rootFlags{})
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"root"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("tree: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "root") || !strings.Contains(got, "child") {
		t.Fatalf("expected root and child in tree, got:\n%s", got)
	}
}

func TestTree_NotFound(t *testing.T) {
	seedClips(t, []map[string]any{{"id": "a", "title": "A"}})
	cmd := newTreeCmd(&rootFlags{})
	cmd.SetOut(&strings.Builder{})
	cmd.SetArgs([]string{"missing"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected not-found error for missing clip")
	}
}
