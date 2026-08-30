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

// TestNovelReviewWatchHelpWires smoke-tests that the review-watch command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelReviewWatchHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"review-watch", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("review-watch --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "review-watch"} {
		if !strings.Contains(help, want) {
			t.Fatalf("review-watch --help missing %q in output:\n%s", want, help)
		}
	}
}

// --- Phase 3 behavior tests ---

func TestLastReviewTransition(t *testing.T) {
	seq := []reviewPoint{
		{Status: "pending", At: "2026-01-01T00:00:00Z"},
		{Status: "approved", At: "2026-01-02T00:00:00Z"},
		{Status: "approved", At: "2026-01-03T00:00:00Z"},
	}
	got := lastReviewTransition(seq)
	if got.From != "pending" || got.To != "approved" || got.Transitions != 1 {
		t.Fatalf("unexpected transition: %+v", got)
	}
	if got.At != "2026-01-02T00:00:00Z" {
		t.Fatalf("unexpected at: %q", got.At)
	}
	if lastReviewTransition(nil).Transitions != 0 {
		t.Fatalf("empty history should have zero transitions")
	}
}

func TestNovelReviewWatch_EmptyStore(t *testing.T) {
	novelEmptyStore(t)
	out, err := runNovelCmd(t, "review-watch", "--json")
	if err != nil {
		t.Fatalf("review-watch: %v", err)
	}
	var wo reviewWatchOutput
	if err := json.Unmarshal([]byte(out), &wo); err != nil {
		t.Fatalf("review-watch json: %v\n%s", err, out)
	}
	if len(wo.Ads) != 0 {
		t.Fatalf("expected empty ads, got %+v", wo.Ads)
	}
}

func TestNovelReviewWatch_HappyPath(t *testing.T) {
	novelSeedStore(t)
	out, err := runNovelCmd(t, "review-watch", "--json")
	if err != nil {
		t.Fatalf("review-watch: %v", err)
	}
	var wo reviewWatchOutput
	if err := json.Unmarshal([]byte(out), &wo); err != nil {
		t.Fatalf("review-watch json: %v\n%s", err, out)
	}
	var ad1 *reviewWatchRow
	for i := range wo.Ads {
		if wo.Ads[i].AdID == "ad_1" {
			ad1 = &wo.Ads[i]
		}
	}
	if ad1 == nil {
		t.Fatalf("ad_1 missing from review-watch: %+v", wo.Ads)
	}
	if ad1.ReviewStatus != "approved" {
		t.Fatalf("ad_1 current review status = %q, want approved", ad1.ReviewStatus)
	}
	if ad1.LastFrom != "rejected" || ad1.LastTo != "approved" || ad1.Transitions != 1 {
		t.Fatalf("ad_1 transition unexpected: %+v", ad1)
	}
	if strings.Contains(out, `"ads":null`) {
		t.Fatalf("ads must never marshal as null: %s", out)
	}
}
