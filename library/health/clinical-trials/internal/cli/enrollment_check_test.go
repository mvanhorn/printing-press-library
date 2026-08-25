// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"io"
	"strings"
	"testing"
)

// TestNovelEnrollmentCheckHelpWires smoke-tests that the enrollment-check
// command resolves at runtime and renders --help without error.
func TestNovelEnrollmentCheckHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"enrollment-check", "--help"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("enrollment-check --help error = %v (novel command not wired correctly?)", err)
	}
}

func TestEnrollmentVerdictFromRisk(t *testing.T) {
	cases := []struct {
		name        string
		factor      riskFactor
		phase       string
		wantVerdict string
	}{
		{
			name:        "zero points is typical",
			factor:      riskFactor{Name: "enrollment", Points: 0, Detail: "enrollment 500"},
			phase:       "PHASE3",
			wantVerdict: "typical",
		},
		{
			name:        "eight points is modest",
			factor:      riskFactor{Name: "enrollment", Points: 8, Detail: "enrollment 50"},
			phase:       "PHASE2",
			wantVerdict: "modest",
		},
		{
			name:        "fifteen points with not-posted detail is not_posted",
			factor:      riskFactor{Name: "enrollment", Points: 15, Detail: "enrollment not posted"},
			phase:       "PHASE1",
			wantVerdict: "not_posted",
		},
		{
			name:        "fifteen points with a small target is modest",
			factor:      riskFactor{Name: "enrollment", Points: 15, Detail: "enrollment 10"},
			phase:       "PHASE1",
			wantVerdict: "modest",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := riskView{Factors: []riskFactor{tc.factor}}
			got := enrollmentVerdictFromRisk(v, tc.phase)
			if got.Verdict != tc.wantVerdict {
				t.Errorf("Verdict = %q, want %q", got.Verdict, tc.wantVerdict)
			}
			if got.RiskPoints != tc.factor.Points {
				t.Errorf("RiskPoints = %d, want %d", got.RiskPoints, tc.factor.Points)
			}
			if got.Label == "" {
				t.Error("Label should not be empty")
			}
			if got.Caveat == "" {
				t.Error("Caveat should not be empty")
			}
		})
	}
}

func TestEnrollmentVerdictLabelUsesPhase(t *testing.T) {
	v := riskView{Factors: []riskFactor{{Name: "enrollment", Points: 0, Detail: "enrollment 500"}}}
	got := enrollmentVerdictFromRisk(v, "PHASE3")
	// phaseLabel(PHASE3) should surface in the label; at minimum the label is
	// non-generic (not the "this trial" fallback) when a phase is supplied.
	if strings.Contains(got.Label, "this trial") {
		t.Errorf("Label = %q, expected a phase-specific label for PHASE3", got.Label)
	}
}
