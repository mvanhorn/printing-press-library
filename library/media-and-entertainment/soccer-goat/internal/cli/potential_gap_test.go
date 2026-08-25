// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/report"
)

// TestNovelPotentialGapHelpWires smoke-tests that the potential-gap command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelPotentialGapHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"potential-gap", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("potential-gap --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "potential-gap"} {
		if !strings.Contains(help, want) {
			t.Fatalf("potential-gap --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestRankPotentialGapsReportsUnavailableData(t *testing.T) {
	team := &report.TeamReport{ClubName: "Benfica", Players: []report.PlayerReport{{Name: "Player", EAOverall: 75}}}
	result := rankPotentialGaps(team, 10)
	if result.Gaps == nil || len(result.Gaps) != 0 {
		t.Fatalf("gaps = %#v, want non-nil empty slice", result.Gaps)
	}
	if result.Note != noPotentialDataNote {
		t.Fatalf("note = %q, want %q", result.Note, noPotentialDataNote)
	}
}
