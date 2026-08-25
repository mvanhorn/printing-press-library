// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"io"
	"testing"
)

// TestNovelSimilarHelpWires smoke-tests that the similar command resolves at
// runtime and renders --help without error.
func TestNovelSimilarHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"similar", "--help"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("similar --help error = %v (novel command not wired correctly?)", err)
	}
}

func TestScoreMatch(t *testing.T) {
	cases := []struct {
		name      string
		seed      Trial
		candidate Trial
		want      int
	}{
		{
			name:      "no overlap",
			seed:      Trial{Phases: []string{"PHASE1"}, Conditions: []string{"Cancer"}, Interventions: []string{"DrugA"}},
			candidate: Trial{Phases: []string{"PHASE3"}, Conditions: []string{"Diabetes"}, Interventions: []string{"DrugZ"}},
			want:      0,
		},
		{
			name:      "shared phase scores +2",
			seed:      Trial{Phases: []string{"PHASE2"}},
			candidate: Trial{Phases: []string{"PHASE2"}},
			want:      2,
		},
		{
			name:      "two shared phases score +4",
			seed:      Trial{Phases: []string{"PHASE1", "PHASE2"}},
			candidate: Trial{Phases: []string{"PHASE1", "PHASE2"}},
			want:      4,
		},
		{
			name:      "additional shared condition scores +1 (first is excluded)",
			seed:      Trial{Conditions: []string{"Cancer", "Melanoma"}},
			candidate: Trial{Conditions: []string{"Cancer", "Melanoma"}},
			want:      1, // index 0 ("Cancer") skipped, "Melanoma" matches
		},
		{
			name:      "shared intervention token scores +1",
			seed:      Trial{Interventions: []string{"Pembrolizumab injection"}},
			candidate: Trial{Interventions: []string{"Pembrolizumab 200mg"}},
			want:      1,
		},
		{
			name:      "short tokens (<=3 chars) are ignored",
			seed:      Trial{Interventions: []string{"abc xy"}},
			candidate: Trial{Interventions: []string{"abc def"}},
			want:      0, // "abc" is 3 chars, not >3, so ignored
		},
		{
			name: "combined phase + condition + intervention",
			seed: Trial{
				Phases:        []string{"PHASE2"},
				Conditions:    []string{"Cancer", "Melanoma"},
				Interventions: []string{"Pembrolizumab"},
			},
			candidate: Trial{
				Phases:        []string{"PHASE2"},
				Conditions:    []string{"Cancer", "Melanoma"},
				Interventions: []string{"Pembrolizumab injection"},
			},
			want: 4, // +2 phase, +1 condition, +1 intervention
		},
		{
			name:      "case-insensitive condition match",
			seed:      Trial{Conditions: []string{"cancer", "MELANOMA"}},
			candidate: Trial{Conditions: []string{"Cancer", "melanoma"}},
			want:      1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := scoreMatch(tc.seed, tc.candidate); got != tc.want {
				t.Errorf("scoreMatch = %d, want %d", got, tc.want)
			}
		})
	}
}
