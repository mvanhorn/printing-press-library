// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestNovelPaceHelpWires smoke-tests that the pace command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelPaceHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"pace", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("pace --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "pace"} {
		if !strings.Contains(help, want) {
			t.Fatalf("pace --help missing %q in output:\n%s", want, help)
		}
	}
}

// --- Phase 3 behavior tests ---

func TestPaceProjectedSpend(t *testing.T) {
	cases := []struct {
		spend int64
		elap  int
		days  int
		want  int64
	}{
		{500, 1, 7, 3500},
		{500, 5, 7, 700},
		{500, 0, 7, 500},
		{500, 10, 7, 500}, // elapsed capped to days
	}
	for _, tc := range cases {
		if got := paceProjectedSpend(tc.spend, tc.elap, tc.days); got != tc.want {
			t.Fatalf("paceProjectedSpend(%d,%d,%d)=%d want %d", tc.spend, tc.elap, tc.days, got, tc.want)
		}
	}
}

func TestPaceClassify(t *testing.T) {
	cases := []struct {
		budget, proj int64
		want         string
	}{
		{100, 50, "under"},
		{100, 95, "on"},
		{100, 120, "over"},
		{0, 50, "unknown"},
	}
	for _, tc := range cases {
		if got := paceClassify(tc.budget, tc.proj); got != tc.want {
			t.Fatalf("paceClassify(%d,%d)=%q want %q", tc.budget, tc.proj, got, tc.want)
		}
	}
}

func TestNovelPace_EmptyStore(t *testing.T) {
	novelEmptyStore(t)
	// stdout stays a bare array (uniform across every novel command); the
	// explanation for the empty result belongs on stderr.
	out, errOut, err := runNovelCmdOutErr(t, "pace", "--json")
	if err != nil {
		t.Fatalf("pace: %v", err)
	}
	var rows []paceRow
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("pace json: %v\n%s", err, out)
	}
	if len(rows) != 0 {
		t.Fatalf("expected empty results, got %+v", rows)
	}
	if !strings.Contains(errOut, "sync") {
		t.Fatalf("stderr should explain the empty result and name sync, got %q", errOut)
	}
}

func TestNovelPace_NoInsightsNote(t *testing.T) {
	novelSeedStore(t)
	out, errOut, err := runNovelCmdOutErr(t, "pace", "--json")
	if err != nil {
		t.Fatalf("pace: %v", err)
	}
	var rows []paceRow
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("pace json: %v\n%s", err, out)
	}
	if len(rows) != 0 {
		t.Fatalf("expected empty results, got %+v", rows)
	}
	if !strings.Contains(errOut, "sync") {
		t.Fatalf("stderr note should name sync, got %q", errOut)
	}
}

func TestNovelPace_HappyPath(t *testing.T) {
	s := novelSeedStore(t)
	if _, err := s.DB().Exec(`INSERT INTO campaigns_insights (id, campaigns_id, data, synced_at, parent_id) VALUES (?,?,?,?,?)`,
		"ins_1", "cmpn_1", `{"spend":50}`, time.Now().UTC().Format(time.RFC3339), "cmpn_1"); err != nil {
		t.Fatalf("insert insight: %v", err)
	}
	out, err := runNovelCmd(t, "pace", "--json", "--days", "7")
	if err != nil {
		t.Fatalf("pace: %v", err)
	}
	var rows []paceRow
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("pace json: %v\n%s", err, out)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 paced campaign, got %+v", rows)
	}
	r0 := rows[0]
	if r0.CampaignID != "cmpn_1" {
		t.Fatalf("unexpected campaign %q", r0.CampaignID)
	}
	// budget 100M/day *7 = 700M; spend 50M *7/1 = 350M -> under
	if r0.Pace != "under" {
		t.Fatalf("expected under pace, got %q (%+v)", r0.Pace, r0)
	}
}
