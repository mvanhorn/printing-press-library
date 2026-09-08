// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelFreshnessHelpWires smoke-tests that the freshness command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelFreshnessHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"freshness", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("freshness --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "freshness"} {
		if !strings.Contains(help, want) {
			t.Fatalf("freshness --help missing %q in output:\n%s", want, help)
		}
	}
}

// TestNovelFreshnessRejectsPositionalArgs pins the guard that a stray word --
// a typo'd flag, most often -- is diagnosed instead of quietly triggering a
// live fetch with all-default flags. A non-empty args slice also suppresses
// the no-flags help branch, so without this guard "freshness junk" ran.
func TestNovelFreshnessRejectsPositionalArgs(t *testing.T) {
	// The command is built standalone rather than driven through RootCmd so
	// the test exercises only the guard: the root's PersistentPreRunE seeds
	// the on-disk learn store, which a unit test must not touch.
	cmd := newNovelFreshnessCmd(&rootFlags{})
	cmd.SetArgs([]string{"junk"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("freshness junk returned nil error; a positional argument must be rejected, not ignored")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("freshness junk exit code = %d, want 2 (usage error); err = %v", got, err)
	}
	if !strings.Contains(err.Error(), "positional") {
		t.Fatalf("freshness junk error = %q, want it to name the rejected argument", err)
	}
}

// TestNovelFreshnessColumnProbes pins the two measured MUFAP column quirks the
// command depends on: the fund name column is "Fund Name" only on tab=returns
// and plain "Fund" elsewhere, and tab=payout carries no Validity Date column
// at all. Resolving either with a single spelling made every payout run report
// a permanent column difference as an upstream page change, and made every
// non-returns tab emit a blank fund for every row.
func TestNovelFreshnessColumnProbes(t *testing.T) {
	if got := MUFAPRowName(map[string]string{"Fund": "ABL Cash Fund"}); got != "ABL Cash Fund" {
		t.Fatalf(`MUFAPRowName with the "Fund" spelling = %q, want the fund name (freshness renders "" for every row otherwise)`, got)
	}
	if got := MUFAPValidityColumn([]string{"Sector", "Fund", "Payout Date"}); got != "Payout Date" {
		t.Fatalf("MUFAPValidityColumn on a payout header list = %q, want %q", got, "Payout Date")
	}
}
