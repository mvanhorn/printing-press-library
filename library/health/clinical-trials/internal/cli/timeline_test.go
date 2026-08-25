// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"io"
	"testing"
)

// TestNovelTimelineHelpWires smoke-tests that the timeline command resolves at
// runtime and renders --help without error.
func TestNovelTimelineHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"timeline", "--help"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("timeline --help error = %v (novel command not wired correctly?)", err)
	}
}

// makeRawTimeline builds a rawTimelineStudy with the four date fields set.
func makeRawTimeline(nct, title, status, start, primary, completion, lastUpdate, whyStopped string) rawTimelineStudy {
	var s rawTimelineStudy
	s.ProtocolSection.IdentificationModule.NCTID = nct
	s.ProtocolSection.IdentificationModule.BriefTitle = title
	s.ProtocolSection.StatusModule.OverallStatus = status
	s.ProtocolSection.StatusModule.WhyStopped = whyStopped
	s.ProtocolSection.StatusModule.StartDateStruct.Date = start
	s.ProtocolSection.StatusModule.PrimaryCompletionDateStruct.Date = primary
	s.ProtocolSection.StatusModule.CompletionDateStruct.Date = completion
	s.ProtocolSection.StatusModule.LastUpdatePostDateStruct.Date = lastUpdate
	return s
}

func TestBuildTimeline(t *testing.T) {
	t.Run("sorts events chronologically across date formats", func(t *testing.T) {
		// Deliberately out of order; mixes YYYY-MM-DD, YYYY-MM, and YYYY.
		s := makeRawTimeline("NCT1", "A trial", "COMPLETED",
			"2020-01-15", // Start
			"2021-06",    // Primary completion
			"2022",       // Completion
			"2023-03-01", // Last update posted
			"")
		v := buildTimeline(s)
		wantLabels := []string{"Start", "Primary completion", "Completion", "Last update posted"}
		if len(v.Events) != len(wantLabels) {
			t.Fatalf("events len = %d, want %d", len(v.Events), len(wantLabels))
		}
		for i, want := range wantLabels {
			if v.Events[i].Label != want {
				t.Errorf("Events[%d].Label = %q, want %q", i, v.Events[i].Label, want)
			}
		}
	})

	t.Run("lexicographic sort is chronologically correct", func(t *testing.T) {
		// Provide dates that only sort right if compared as strings left-to-right.
		s := makeRawTimeline("NCT2", "B", "RECRUITING",
			"2022-12-31", // Start (latest)
			"2019-01",    // Primary completion (earliest)
			"2020",       // Completion (middle)
			"",           // Last update posted (omitted)
			"")
		v := buildTimeline(s)
		wantOrder := []string{"2019-01", "2020", "2022-12-31"}
		if len(v.Events) != len(wantOrder) {
			t.Fatalf("events len = %d, want %d", len(v.Events), len(wantOrder))
		}
		for i, want := range wantOrder {
			if v.Events[i].Date != want {
				t.Errorf("Events[%d].Date = %q, want %q", i, v.Events[i].Date, want)
			}
		}
	})

	t.Run("empty dates are omitted", func(t *testing.T) {
		s := makeRawTimeline("NCT3", "C", "RECRUITING",
			"2021-01-01", "", "", "2021-05-01", "")
		v := buildTimeline(s)
		if len(v.Events) != 2 {
			t.Fatalf("events len = %d, want 2 (empty dates omitted)", len(v.Events))
		}
	})

	t.Run("no dates yields explanatory note", func(t *testing.T) {
		s := makeRawTimeline("NCT4", "D", "UNKNOWN", "", "", "", "", "")
		v := buildTimeline(s)
		if len(v.Events) != 0 {
			t.Fatalf("events len = %d, want 0", len(v.Events))
		}
		if v.Note == "" {
			t.Error("expected a note when no dates are posted")
		}
	})

	t.Run("whyStopped surfaces in note", func(t *testing.T) {
		s := makeRawTimeline("NCT5", "E", "TERMINATED",
			"2021-01-01", "", "", "", "lack of funding")
		v := buildTimeline(s)
		if v.Note == "" {
			t.Error("expected a note carrying whyStopped")
		}
	})
}
