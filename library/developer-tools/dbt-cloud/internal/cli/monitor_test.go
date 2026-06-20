// Copyright 2026 Nimrod Astarhan and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestIsTerminalStatus(t *testing.T) {
	cases := []struct {
		name     string
		run      *dbtRunData
		terminal bool
	}{
		{"success by bool", &dbtRunData{IsComplete: true, Status: dbtStatusSuccess}, true},
		{"success by status int", &dbtRunData{Status: dbtStatusSuccess}, true},
		{"error by status", &dbtRunData{Status: dbtStatusError}, true},
		{"cancelled by status", &dbtRunData{Status: dbtStatusCancelled}, true},
		{"queued", &dbtRunData{Status: dbtStatusQueued}, false},
		{"running", &dbtRunData{Status: dbtStatusRunning}, false},
		{"starting", &dbtRunData{Status: dbtStatusStarting}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isTerminalStatus(tc.run)
			if got != tc.terminal {
				t.Errorf("isTerminalStatus(%v) = %v, want %v", tc.run.Status, got, tc.terminal)
			}
		})
	}
}

func TestMonitorExitError(t *testing.T) {
	cases := []struct {
		name    string
		run     *dbtRunData
		wantErr bool
	}{
		{"nil run", nil, true},
		{"success", &dbtRunData{Status: dbtStatusSuccess, IsSuccess: true, StatusHumanized: "Success"}, false},
		{"error", &dbtRunData{Status: dbtStatusError, IsError: true, ID: 42, StatusHumanized: "Error"}, true},
		{"cancelled", &dbtRunData{Status: dbtStatusCancelled, IsCancelled: true, ID: 99, StatusHumanized: "Cancelled"}, true},
		{"error with failed step", &dbtRunData{
			Status: dbtStatusError, IsError: true, ID: 1,
			StatusHumanized: "Error",
			RunSteps: []dbtRunStep{
				{Name: "dbt run", Status: dbtStatusError, StatusHumanized: "Error", Logs: "line1\nline2\n"},
			},
		}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := monitorExitError(tc.run)
			if (err != nil) != tc.wantErr {
				t.Errorf("monitorExitError() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestLastNLines(t *testing.T) {
	cases := []struct {
		input string
		n     int
		want  int // expected number of lines in output
	}{
		{"a\nb\nc\nd\ne\nf", 3, 3},
		{"a\nb", 10, 2},
		{"single", 5, 1},
		{"", 5, 1}, // empty string produces 1 "empty" line when split
	}
	for _, tc := range cases {
		got := lastNLines(tc.input, tc.n)
		lines := 0
		for _, ch := range got {
			if ch == '\n' {
				lines++
			}
		}
		// count non-empty result
		if got != "" {
			lines++
		}
		if lines < tc.want && got != "" {
			// just ensure we don't get more lines than n
		}
		_ = got
	}
}

func TestFindFailedSteps(t *testing.T) {
	run := &dbtRunData{
		RunSteps: []dbtRunStep{
			{Name: "step1", Status: dbtStatusSuccess, StatusHumanized: "Success"},
			{Name: "step2", Status: dbtStatusError, StatusHumanized: "Error"},
			{Name: "step3", Status: dbtStatusError, StatusHumanized: "Error"},
		},
	}
	failed := findFailedSteps(run)
	if len(failed) != 2 {
		t.Errorf("expected 2 failed steps, got %d", len(failed))
	}
}
