// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestNovelTreeHelpWires smoke-tests that the tree command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelTreeHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"tree", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("tree --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "tree"} {
		if !strings.Contains(help, want) {
			t.Fatalf("tree --help missing %q in output:\n%s", want, help)
		}
	}
}

// --- Phase 3 behavior tests ---

func TestNovelTree_EmptyStoreJSON(t *testing.T) {
	novelEmptyStore(t)
	out, err := runNovelCmd(t, "tree", "--json")
	if err != nil {
		t.Fatalf("tree empty: %v", err)
	}
	if strings.TrimSpace(out) != "[]" {
		t.Fatalf("tree empty expected [], got %q", out)
	}
}

func TestNovelTree_HappyPath(t *testing.T) {
	novelSeedStore(t)
	out, err := runNovelCmd(t, "tree", "--json")
	if err != nil {
		t.Fatalf("tree: %v", err)
	}
	var tree []treeCampaign
	if err := json.Unmarshal([]byte(out), &tree); err != nil {
		t.Fatalf("tree json: %v\n%s", err, out)
	}
	if len(tree) != 2 {
		t.Fatalf("expected 2 campaigns, got %d: %s", len(tree), out)
	}
	var found bool
	for _, cam := range tree {
		if cam.ID == "cmpn_1" {
			found = true
			if len(cam.AdGroups) != 3 {
				t.Fatalf("cmpn_1 expected 3 ad groups, got %d", len(cam.AdGroups))
			}
			var had1 bool
			for _, ag := range cam.AdGroups {
				if ag.ID == "adgrp_1" {
					had1 = true
					if len(ag.Ads) != 1 || ag.Ads[0].ID != "ad_1" || ag.Ads[0].ReviewStatus != "approved" {
						t.Fatalf("adgrp_1 ads unexpected: %+v", ag.Ads)
					}
				}
			}
			if !had1 {
				t.Fatalf("adgrp_1 missing")
			}
		}
	}
	if !found {
		t.Fatalf("cmpn_1 missing from tree")
	}
}

func TestNovelTree_StatusFilter(t *testing.T) {
	novelSeedStore(t)
	out, err := runNovelCmd(t, "tree", "--json", "--status", "paused")
	if err != nil {
		t.Fatalf("tree: %v", err)
	}
	var tree []treeCampaign
	if err := json.Unmarshal([]byte(out), &tree); err != nil {
		t.Fatalf("tree json: %v", err)
	}
	if len(tree) != 1 || tree[0].ID != "cmpn_2" {
		t.Fatalf("status filter expected only cmpn_2, got %s", out)
	}
}
