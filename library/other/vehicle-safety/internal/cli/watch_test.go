// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelWatchHelpWires smoke-tests that the watch command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelWatchHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"watch", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("watch --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "watch"} {
		if !strings.Contains(help, want) {
			t.Fatalf("watch --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestReconcileCampaignSnapshotPreservesEmptyFetchAndTracksLifecycle(t *testing.T) {
	prior := map[string]campaignSnapshot{
		"A": {Remedy: "old", Active: true},
		"B": {Remedy: "same", Active: false},
	}
	kept, emptyDelta := reconcileCampaignSnapshot(prior, map[string]string{}, true)
	if emptyDelta.Advanced || len(kept) != 2 {
		t.Fatalf("empty fetch advanced=%v kept=%v", emptyDelta.Advanced, kept)
	}

	next, delta := reconcileCampaignSnapshot(prior, map[string]string{"B": "same", "C": "new"}, true)
	if len(delta.Added) != 1 || delta.Added[0] != "C" || len(delta.Restored) != 1 || delta.Restored[0] != "B" || len(delta.Removed) != 1 || delta.Removed[0] != "A" {
		t.Fatalf("unexpected delta: %#v", delta)
	}
	if next["A"].Active || !next["B"].Active || !next["C"].Active {
		t.Fatalf("unexpected next snapshot: %#v", next)
	}
}
