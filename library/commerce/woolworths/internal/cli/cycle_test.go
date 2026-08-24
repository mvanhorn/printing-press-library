// Copyright 2026 Richard Gill and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/woolworths/internal/store"
)

// TestNovelCycleHelpWires smoke-tests that the cycle command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelCycleHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"cycle", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cycle --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "cycle"} {
		if !strings.Contains(help, want) {
			t.Fatalf("cycle --help missing %q in output:\n%s", want, help)
		}
	}
}

// ppwSeedCycleDB lays down `days` daily observations for one stockcode, marking
// the days named by halfPrice as half-price sightings.
func ppwSeedCycleDB(t *testing.T, stockcode string, days int, halfPrice map[int]bool) string {
	t.Helper()
	obs := make([]store.PriceObservation, 0, days)
	for d := 0; d < days; d++ {
		if halfPrice[d] {
			obs = append(obs, ppwObs(stockcode, d, 3.00, 6.00, true, true))
			continue
		}
		obs = append(obs, ppwObs(stockcode, d, 6.00, 6.00, false, false))
	}
	return ppwSeedDB(t, obs, nil)
}

// TestCycleAbsenceOfEpisodes is the honesty case: a product that has never been
// half-price must yield an empty result and a note, never a forecast.
func TestCycleAbsenceOfEpisodes(t *testing.T) {
	db := ppwSeedCycleDB(t, "444444", 30, nil)

	stdout, stderr, err := ppwRunCLI(t, "cycle", "444444", "--db", db, "--json")
	t.Logf("stdout:\n%s", stdout)
	t.Logf("stderr:\n%s", stderr)
	if err != nil {
		t.Fatalf("cycle returned error: %v", err)
	}
	rows := ppwDecodeRows[cycleRow](t, stdout)
	if len(rows) != 0 {
		t.Fatalf("expected [] for a product with no half-price episode, got %d row(s): %+v", len(rows), rows)
	}
	if !strings.Contains(stderr, "no recorded half-price episode") {
		t.Fatalf("expected an honest note about the missing episodes, got stderr:\n%s", stderr)
	}
	for _, forbidden := range []string{"next_window", "forecast\": true"} {
		if strings.Contains(stdout, forbidden) {
			t.Fatalf("empty result leaked a forecast field %q:\n%s", forbidden, stdout)
		}
	}
}

// TestCyclePositive seeds 3 half-price episodes whose starts are exactly 28
// days apart and checks the reported rhythm.
func TestCyclePositive(t *testing.T) {
	halfPrice := map[int]bool{}
	for _, start := range []int{0, 28, 56} {
		for d := start; d < start+3; d++ {
			halfPrice[d] = true
		}
	}
	db := ppwSeedCycleDB(t, "555555", 60, halfPrice)

	stdout, stderr, err := ppwRunCLI(t, "cycle", "555555", "--db", db, "--json")
	t.Logf("stdout:\n%s", stdout)
	t.Logf("stderr:\n%s", stderr)
	if err != nil {
		t.Fatalf("cycle returned error: %v", err)
	}
	rows := ppwDecodeRows[cycleRow](t, stdout)
	if len(rows) != 1 {
		t.Fatalf("expected 1 cycle row, got %d", len(rows))
	}
	r := rows[0]
	if r.Episodes != 3 {
		t.Fatalf("episodes = %d, want 3", r.Episodes)
	}
	if math.Abs(r.MedianGapDays-28) > 0.05 {
		t.Fatalf("median gap = %v days, want ~28", r.MedianGapDays)
	}
	if math.Abs(r.MedianRunDays-2) > 0.05 {
		t.Fatalf("median run = %v days, want ~2", r.MedianRunDays)
	}
	if r.Confidence == "low" || r.Confidence == "none" {
		t.Fatalf("confidence = %q, want better than low for 3 regular episodes", r.Confidence)
	}
	if !r.Forecast || r.NextWindowStartAt == 0 || r.NextWindowEndAt <= r.NextWindowStartAt {
		t.Fatalf("expected a forward-looking window, got %+v", r)
	}
	if len(r.EpisodeList) != 3 {
		t.Fatalf("episode list has %d entries, want 3", len(r.EpisodeList))
	}
}

// TestCycleRejectsLiveDataSource pins the "no live equivalent" contract.
func TestCycleRejectsLiveDataSource(t *testing.T) {
	db := ppwSeedCycleDB(t, "555555", 5, map[int]bool{0: true})
	stdout, stderr, err := ppwRunCLI(t, "cycle", "555555", "--db", db, "--data-source", "live", "--json")
	t.Logf("stdout:\n%s", stdout)
	t.Logf("stderr:\n%s", stderr)
	if err == nil {
		t.Fatalf("--data-source live was accepted; it must be refused")
	}
	if !strings.Contains(err.Error(), "no live equivalent") {
		t.Fatalf("error = %q, want it to say there is no live equivalent", err.Error())
	}
	if code := ExitCode(err); code != 2 {
		t.Fatalf("exit code = %d, want 2 for a usage error", code)
	}
}

// TestCycleJSONFidelityOnMissingMirror covers the missing-mirror guard.
func TestCycleJSONFidelityOnMissingMirror(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.db")
	stdout, stderr, err := ppwRunCLI(t, "cycle", "6073909", "--db", missing, "--json")
	t.Logf("stdout:\n%s", stdout)
	t.Logf("stderr:\n%s", stderr)
	if err != nil {
		t.Fatalf("missing mirror returned error: %v", err)
	}
	rows := ppwDecodeRows[cycleRow](t, stdout)
	if len(rows) != 0 {
		t.Fatalf("expected [], got %d rows", len(rows))
	}
	if !strings.Contains(stderr, "no local mirror at") {
		t.Fatalf("missing-mirror guidance absent from stderr:\n%s", stderr)
	}
}

func TestCycleDryRun(t *testing.T) {
	stdout, stderr, err := ppwRunCLI(t, "cycle", "6073909", "--dry-run", "--json")
	t.Logf("stdout:\n%s", stdout)
	t.Logf("stderr:\n%s", stderr)
	if err != nil {
		t.Fatalf("dry run returned error: %v", err)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &envelope); err != nil {
		t.Fatalf("dry-run envelope is not JSON: %v\nraw:\n%s", err, stdout)
	}
	if envelope["dry_run"] != true || envelope["action"] != "cycle" {
		t.Fatalf("unexpected dry-run envelope: %v", envelope)
	}
}

func TestCycleBareInvocationPrintsHelp(t *testing.T) {
	stdout, _, err := ppwRunCLI(t, "cycle")
	if err != nil {
		t.Fatalf("bare invocation returned error: %v", err)
	}
	if !strings.Contains(stdout, "Usage:") {
		t.Fatalf("bare invocation did not print help:\n%s", stdout)
	}
}
