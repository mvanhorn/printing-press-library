// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cmuxclient

import "testing"

func TestIconState(t *testing.T) {
	cases := []struct {
		title string
		want  State
	}{
		{"✳ Claude Code", StateAwaiting},
		{"✻ Output HTML", StateAwaiting},
		{"⠐ Working", StateWorking},
		{"Plain terminal", StateIdle},
		{"", StateUnknown},
		{"  ⠂ leading spaces", StateWorking},
	}
	for _, tc := range cases {
		if got := IconState(tc.title); got != tc.want {
			t.Errorf("IconState(%q) = %s, want %s", tc.title, got, tc.want)
		}
	}
}

func TestCanonicalState(t *testing.T) {
	cases := []struct {
		name          string
		statusValue   string
		titles        []string
		strandedCount int
		want          State
	}{
		{"stranded count ignored — falls back to status value", "Running", []string{"Plain"}, 2, StateWorking},
		{"working icon beats Running text", "Needs input", []string{"⠐ Working"}, 0, StateWorking},
		{"awaiting icon beats Running text", "Running", []string{"✳ Claude Code"}, 0, StateAwaiting},
		{"falls back to status value", "Running", []string{"Plain"}, 0, StateWorking},
		{"empty all idle", "", []string{}, 0, StateIdle},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CanonicalState(tc.statusValue, tc.titles, tc.strandedCount); got != tc.want {
				t.Errorf("CanonicalState(%q, %v, %d) = %s, want %s", tc.statusValue, tc.titles, tc.strandedCount, got, tc.want)
			}
		})
	}
}
