// Copyright 2026 wwilson1017 and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
	"time"
)

func TestParseStatus(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"next_action", 1, true},
		{"Next Action", 1, true},
		{"next-action", 1, true},
		{"waiting", 5, true},
		{"5", 5, true},
		{"0", 0, true},
		{"11", 0, false},
		{"bogus", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		got, ok := parseStatus(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("parseStatus(%q) = (%d,%v), want (%d,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestParsePriority(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"negative", -1, true},
		{"low", 0, true},
		{"high", 2, true},
		{"top", 3, true},
		{"-1", -1, true},
		{"3", 3, true},
		{"4", 0, false},
		{"bogus", 0, false},
	}
	for _, c := range cases {
		got, ok := parsePriority(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("parsePriority(%q) = (%d,%v), want (%d,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestParseGoalLevel(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"lifetime", 0, true},
		{"long", 1, true},
		{"long-term", 1, true},
		{"short", 2, true},
		{"short-term", 2, true},
		{"short_term", 2, true},
		{"2", 2, true},
		{"3", 0, false},
		{"bogus", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		got, ok := parseGoalLevel(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("parseGoalLevel(%q) = (%d,%v), want (%d,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestParseDueDate(t *testing.T) {
	want := time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC).Unix()
	got, err := parseDueDate("2026-06-20")
	if err != nil {
		t.Fatalf("parseDueDate returned error: %v", err)
	}
	if got != want {
		t.Errorf("parseDueDate(2026-06-20) = %d, want %d (noon UTC)", got, want)
	}
	if _, err := parseDueDate("not-a-date"); err == nil {
		t.Error("parseDueDate(not-a-date) should error")
	}
}

func TestComputeStalledProjects(t *testing.T) {
	folders := map[int]string{1: "HasNextAction", 2: "Stalled", 3: "AlsoStalled"}
	// folder 1: one Next Action -> not stalled
	// folder 2: only an Active task -> stalled
	// folder 3: two non-next-action tasks -> stalled (count 2)
	// folder 0 (no folder): ignored
	open := []taskRow{
		{ID: "a", Folder: 1, Status: statusNextAction},
		{ID: "b", Folder: 1, Status: 2},
		{ID: "c", Folder: 2, Status: 2},
		{ID: "d", Folder: 3, Status: 2},
		{ID: "e", Folder: 3, Status: 5},
		{ID: "f", Folder: 0, Status: 2},
	}
	got := computeStalledProjects(open, folders)
	if len(got) != 2 {
		t.Fatalf("expected 2 stalled projects, got %d: %+v", len(got), got)
	}
	// Sorted by open count desc: folder 3 (2 open) before folder 2 (1 open).
	if got[0].FolderID != 3 || got[0].OpenTasks != 2 {
		t.Errorf("first stalled = %+v, want folder 3 with 2 open", got[0])
	}
	if got[1].FolderID != 2 || got[1].OpenTasks != 1 {
		t.Errorf("second stalled = %+v, want folder 2 with 1 open", got[1])
	}
}
