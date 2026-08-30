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

// TestNovelDriftHelpWires smoke-tests that the drift command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelDriftHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"drift", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("drift --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "drift"} {
		if !strings.Contains(help, want) {
			t.Fatalf("drift --help missing %q in output:\n%s", want, help)
		}
	}
}

// --- Phase 3 behavior tests ---

func TestDiffEntityFields(t *testing.T) {
	prev := map[string]any{"status": "paused", "budget": map[string]any{"daily_spend_limit_micros": 80000000}}
	cur := map[string]any{"status": "active", "budget": map[string]any{"daily_spend_limit_micros": 100000000}}
	got := diffEntityFields(prev, cur)
	byField := map[string]driftField{}
	for _, ch := range got {
		byField[ch.Field] = ch
	}
	if byField["status"].To != "active" {
		t.Fatalf("status change missing: %+v", got)
	}
	if byField["daily_budget"].From != "8e+07" && byField["daily_budget"].From != "80000000" {
		t.Fatalf("budget from unexpected: %+v", got)
	}
	// identical snapshots -> no changes
	if d := diffEntityFields(prev, prev); len(d) != 0 {
		t.Fatalf("identical snapshots should diff empty, got %+v", d)
	}
}

func TestNovelDrift_EmptyStoreJSON(t *testing.T) {
	novelEmptyStore(t)
	out, err := runNovelCmd(t, "drift", "--json")
	if err != nil {
		t.Fatalf("drift: %v", err)
	}
	if strings.TrimSpace(out) != "[]" {
		t.Fatalf("drift empty expected [], got %q", out)
	}
}

func TestNovelDrift_HappyPath(t *testing.T) {
	novelSeedStore(t)
	out, err := runNovelCmd(t, "drift", "--json", "--since", "7d")
	if err != nil {
		t.Fatalf("drift: %v", err)
	}
	var changes []driftChange
	if err := json.Unmarshal([]byte(out), &changes); err != nil {
		t.Fatalf("drift json: %v\n%s", err, out)
	}
	var sawCampaign bool
	for _, ch := range changes {
		if ch.EntityID == "cmpn_1" {
			sawCampaign = true
			for _, f := range ch.Changes {
				if f.Field == "status" {
					return
				}
			}
		}
	}
	if !sawCampaign {
		t.Fatalf("expected a status drift for cmpn_1, got %s", out)
	}
	t.Fatalf("cmpn_1 present but no status change recorded: %s", out)
}
