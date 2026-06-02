// Copyright 2026 riccardovandra and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for the hand-built project-management transcendence logic.

package cli

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseMSField(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want int64
	}{
		{"string epoch", "1709251200000", 1709251200000},
		{"number epoch", float64(1709251200000), 1709251200000},
		{"empty string", "", 0},
		{"nil", nil, 0},
		{"garbage", "not-a-number", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseMSField(c.in); got != c.want {
				t.Fatalf("parseMSField(%v) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}

func TestParsePMTask(t *testing.T) {
	raw := json.RawMessage(`{
		"id": "86abc",
		"name": "Fix login bug",
		"status": {"status": "in progress", "type": "custom"},
		"assignees": [{"id": 42, "username": "priya"}, {"id": 7, "username": "marco"}],
		"due_date": "1709251200000",
		"date_updated": "1709000000000",
		"time_estimate": 3600000,
		"list": {"id": "901"},
		"space": {"id": "555"},
		"dependencies": [{"task_id": "86abc", "depends_on": "86xyz", "type": 1}]
	}`)
	tk, ok := parsePMTask(raw)
	if !ok {
		t.Fatal("parsePMTask returned ok=false")
	}
	if tk.ID != "86abc" || tk.Name != "Fix login bug" {
		t.Fatalf("bad id/name: %q %q", tk.ID, tk.Name)
	}
	if tk.Status != "in progress" || tk.StatusType != "custom" {
		t.Fatalf("bad status: %q %q", tk.Status, tk.StatusType)
	}
	if !tk.open() {
		t.Fatal("expected open task")
	}
	if len(tk.Assignees) != 2 || tk.Assignees[0].ID != 42 {
		t.Fatalf("bad assignees: %+v", tk.Assignees)
	}
	if tk.DueDate != 1709251200000 || tk.TimeEstimate != 3600000 {
		t.Fatalf("bad due/estimate: %d %d", tk.DueDate, tk.TimeEstimate)
	}
	if tk.ListID != "901" || tk.SpaceID != "555" {
		t.Fatalf("bad list/space: %q %q", tk.ListID, tk.SpaceID)
	}
	if len(tk.Deps) != 1 || tk.Deps[0].DependsOn != "86xyz" {
		t.Fatalf("bad deps: %+v", tk.Deps)
	}
}

func TestPMTaskOpenClosed(t *testing.T) {
	closed := json.RawMessage(`{"id":"1","status":{"status":"done","type":"closed"},"date_closed":"1709000000000"}`)
	tk, _ := parsePMTask(closed)
	if tk.open() {
		t.Fatal("closed task reported open")
	}
}

func TestMatchAssignee(t *testing.T) {
	tk := pmTask{Assignees: []pmAssignee{{ID: 42, Username: "priya", Email: "p@x.com"}}}
	cases := []struct {
		sel  string
		meID int64
		want bool
	}{
		{"", 0, true},        // empty selector matches all
		{"priya", 0, true},   // username
		{"PRIYA", 0, true},   // case-insensitive
		{"42", 0, true},      // numeric id
		{"p@x.com", 0, true}, // email
		{"me", 42, true},     // me resolved to id 42
		{"me", 99, false},    // me resolved to a different id
		{"marco", 0, false},  // not assigned
	}
	for _, c := range cases {
		if got := tk.matchAssignee(c.sel, c.meID); got != c.want {
			t.Fatalf("matchAssignee(%q, %d) = %v, want %v", c.sel, c.meID, got, c.want)
		}
	}
}

func TestParseDurationWindow(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
		ok   bool
	}{
		{"7d", 7 * 24 * time.Hour, true},
		{"2w", 14 * 24 * time.Hour, true},
		{"24h", 24 * time.Hour, true},
		{"90m", 90 * time.Minute, true},
		{"", 0, false},
		{"abc", 0, false},
	}
	for _, c := range cases {
		got, err := parseDurationWindow(c.in)
		if c.ok && (err != nil || got != c.want) {
			t.Fatalf("parseDurationWindow(%q) = %v, %v; want %v", c.in, got, err, c.want)
		}
		if !c.ok && err == nil {
			t.Fatalf("parseDurationWindow(%q) expected error", c.in)
		}
	}
}

func TestResolveDue(t *testing.T) {
	// Anchor: Wednesday 2026-06-03 10:00 UTC.
	now := time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		in       string
		wantYMD  string // expected date (local) YYYY-MM-DD
		wantHour int
		ok       bool
	}{
		{"today", "2026-06-03", 23, true},
		{"tomorrow", "2026-06-04", 23, true},
		{"yesterday", "2026-06-02", 23, true},
		{"friday", "2026-06-05", 23, true},      // upcoming Friday
		{"next friday", "2026-06-12", 23, true}, // a week out
		{"friday 5pm", "2026-06-05", 17, true},
		{"3d", "2026-06-06", 0, true}, // offset keeps the now time-of-day-ish
		{"garbage words here", "", 0, false},
	}
	for _, c := range cases {
		ms, ok := resolveDue(c.in, now)
		if ok != c.ok {
			t.Fatalf("resolveDue(%q) ok=%v want %v", c.in, ok, c.ok)
		}
		if !ok {
			continue
		}
		got := time.UnixMilli(ms).UTC()
		if got.Format("2006-01-02") != c.wantYMD {
			t.Fatalf("resolveDue(%q) date=%s want %s", c.in, got.Format("2006-01-02"), c.wantYMD)
		}
		if c.in == "friday 5pm" && got.Hour() != c.wantHour {
			t.Fatalf("resolveDue(%q) hour=%d want %d", c.in, got.Hour(), c.wantHour)
		}
	}
}

func TestParseClock(t *testing.T) {
	cases := []struct {
		in     string
		hh, mm int
		ok     bool
	}{
		{"5pm", 17, 0, true},
		{"5:30pm", 17, 30, true},
		{"9am", 9, 0, true},
		{"12am", 0, 0, true},
		{"17:00", 17, 0, true},
		{"7", 0, 0, false}, // bare number is ambiguous, not a clock
		{"xx", 0, 0, false},
	}
	for _, c := range cases {
		hh, mm, ok := parseClock(c.in)
		if ok != c.ok {
			t.Fatalf("parseClock(%q) ok=%v want %v", c.in, ok, c.ok)
		}
		if ok && (hh != c.hh || mm != c.mm) {
			t.Fatalf("parseClock(%q) = %d:%02d want %d:%02d", c.in, hh, mm, c.hh, c.mm)
		}
	}
}

func TestFingerprintChange(t *testing.T) {
	a := pmTask{ID: "1", Status: "open", Assignees: []pmAssignee{{ID: 1}}, DueDate: 100}
	b := pmTask{ID: "1", Status: "review", Assignees: []pmAssignee{{ID: 1}}, DueDate: 100}
	fa, fb := fingerprintOf(a), fingerprintOf(b)
	if fa.status == fb.status {
		t.Fatal("expected status fingerprints to differ")
	}
	if fa.assignees != fb.assignees {
		t.Fatal("expected assignee fingerprints to match")
	}
}

func TestAccumulateStatusMinutes(t *testing.T) {
	var obj map[string]any
	_ = json.Unmarshal([]byte(`{
		"current_status": {"status":"review","total_time":{"by_minute":30}},
		"status_history": [
			{"status":"open","total_time":{"by_minute":120}},
			{"status":"review","total_time":{"by_minute":45}}
		]
	}`), &obj)
	per := map[string]int64{}
	accumulateStatusMinutes(obj, per)
	if per["review"] != 75 { // 30 current + 45 history
		t.Fatalf("review minutes = %d, want 75", per["review"])
	}
	if per["open"] != 120 {
		t.Fatalf("open minutes = %d, want 120", per["open"])
	}
}
