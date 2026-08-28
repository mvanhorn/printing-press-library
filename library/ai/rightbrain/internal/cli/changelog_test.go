// Copyright 2026 Farouk Umar and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestNovelChangelogHelpWires smoke-tests that the changelog command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelChangelogHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"changelog", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("changelog --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "changelog", "--since", "--verify", "--actor", "--resource-type", "--limit"} {
		if !strings.Contains(help, want) {
			t.Fatalf("changelog --help missing %q in output:\n%s", want, help)
		}
	}
}

// changelogNow is the fixed "now" every case below is measured against, so ages
// are deterministic.
var changelogNow = time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)

func changelogAt(hoursAgo int) string {
	return changelogNow.Add(-time.Duration(hoursAgo) * time.Hour).Format(time.RFC3339)
}

func findChangelogEvent(t *testing.T, report changelogReport, id string) changelogEvent {
	t.Helper()
	for _, ev := range report.Events {
		if ev.ID == id {
			return ev
		}
	}
	t.Fatalf("event %q not present in report; got %d events", id, len(report.Events))
	return changelogEvent{}
}

func TestChangelogResolvesResourceNames(t *testing.T) {
	const taskUUID = "0195d1ff-1f05-437a-95ac-6de8969cb47b"
	const agentUUID = "0195d207-32bb-d03d-cfdc-f4516e9222c8"

	cases := []struct {
		name         string
		event        auditRow
		names        map[string]string
		wantName     string
		wantResolved bool
	}{
		{
			name: "task uuid resolves to its mirrored name",
			event: auditRow{
				ID: "e1", ActorUserID: "alice", EventType: "task.updated",
				ResourceType: "task", ResourceID: taskUUID, OccurredAt: changelogAt(2),
			},
			names:        map[string]string{taskUUID: "Nightly Summarizer"},
			wantName:     "Nightly Summarizer",
			wantResolved: true,
		},
		{
			name: "agent uuid resolves to its mirrored name",
			event: auditRow{
				ID: "e2", ActorUserID: "bob", EventType: "agent.created",
				ResourceType: "task_agent", ResourceID: agentUUID, OccurredAt: changelogAt(5),
			},
			names:        map[string]string{taskUUID: "Nightly Summarizer", agentUUID: "Triage Bot"},
			wantName:     "Triage Bot",
			wantResolved: true,
		},
		{
			name: "whitespace-only mirrored name is not treated as a name",
			event: auditRow{
				ID: "e3", ActorUserID: "carol", EventType: "skill.updated",
				ResourceType: "skill", ResourceID: taskUUID, OccurredAt: changelogAt(1),
			},
			names:        map[string]string{taskUUID: "   "},
			wantName:     taskUUID,
			wantResolved: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report := buildChangelogReport([]auditRow{tc.event}, tc.names, changelogNow)
			if report.Total != 1 {
				t.Fatalf("total = %d, want 1", report.Total)
			}
			got := findChangelogEvent(t, report, tc.event.ID)
			if got.ResourceName != tc.wantName {
				t.Errorf("resource_name = %q, want %q", got.ResourceName, tc.wantName)
			}
			if got.Resolved != tc.wantResolved {
				t.Errorf("resolved = %v, want %v", got.Resolved, tc.wantResolved)
			}
			if got.ResourceID != tc.event.ResourceID {
				t.Errorf("resource_id = %q, want the original UUID %q", got.ResourceID, tc.event.ResourceID)
			}
			if tc.wantResolved && report.ResolvedCount != 1 {
				t.Errorf("resolved_count = %d, want 1", report.ResolvedCount)
			}
			if !tc.wantResolved && report.UnresolvedCount != 1 {
				t.Errorf("unresolved_count = %d, want 1", report.UnresolvedCount)
			}
			if got.Age == "" || got.Age == "unknown" {
				t.Errorf("age = %q, want a humanized duration", got.Age)
			}
		})
	}
}

// TestChangelogNeverInventsNames is the absence-of-correctness check: a UUID the
// mirror does not know must come back as the bare UUID, never as a guessed name
// and never blanked out.
func TestChangelogNeverInventsNames(t *testing.T) {
	const knownUUID = "0195d1ff-1f05-437a-95ac-6de8969cb47b"
	const unknownUUID = "01960000-dead-beef-cafe-000000000001"

	events := []auditRow{
		{ID: "known", ActorUserID: "alice", EventType: "task.updated",
			ResourceType: "task", ResourceID: knownUUID, OccurredAt: changelogAt(3)},
		{ID: "unknown", ActorUserID: "alice", EventType: "task.deleted",
			ResourceType: "task", ResourceID: unknownUUID, OccurredAt: changelogAt(4)},
	}
	names := map[string]string{knownUUID: "Nightly Summarizer"}

	report := buildChangelogReport(events, names, changelogNow)

	known := findChangelogEvent(t, report, "known")
	if !known.Resolved || known.ResourceName != "Nightly Summarizer" {
		t.Fatalf("known event: resolved=%v name=%q, want true/%q", known.Resolved, known.ResourceName, "Nightly Summarizer")
	}

	unknown := findChangelogEvent(t, report, "unknown")
	if unknown.Resolved {
		t.Errorf("unknown uuid reported resolved=true; it must stay unresolved")
	}
	if unknown.ResourceName != unknownUUID {
		t.Errorf("resource_name = %q, want the bare UUID %q (no invented or blanked name)", unknown.ResourceName, unknownUUID)
	}
	if unknown.ResourceName == "" {
		t.Errorf("resource_name was blanked; the bare UUID must be preserved")
	}
	if unknown.ResourceName == known.ResourceName {
		t.Errorf("unknown uuid borrowed the neighbouring resource's name %q", known.ResourceName)
	}
	if report.ResolvedCount != 1 || report.UnresolvedCount != 1 {
		t.Errorf("resolved_count=%d unresolved_count=%d, want 1/1", report.ResolvedCount, report.UnresolvedCount)
	}
	if report.Total != 2 {
		t.Errorf("total = %d, want 2", report.Total)
	}

	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshaling report: %v", err)
	}
	if !strings.Contains(string(raw), unknownUUID) {
		t.Errorf("marshaled report dropped the unresolved UUID %q: %s", unknownUUID, raw)
	}
}

func TestChangelogGroupsByResourceTypeAndActor(t *testing.T) {
	const taskUUID = "0195d1ff-1f05-437a-95ac-6de8969cb47b"
	const agentUUID = "0195d207-32bb-d03d-cfdc-f4516e9222c8"
	const skillUUID = "0195d300-0000-0000-0000-00000000abcd"

	events := []auditRow{
		{ID: "a", ActorUserID: "alice", EventType: "task.updated", ResourceType: "task", ResourceID: taskUUID, OccurredAt: changelogAt(1)},
		{ID: "b", ActorUserID: "alice", EventType: "task.updated", ResourceType: "task", ResourceID: taskUUID, OccurredAt: changelogAt(6)},
		{ID: "c", ActorUserID: "bob", EventType: "agent.created", ResourceType: "task_agent", ResourceID: agentUUID, OccurredAt: changelogAt(9)},
		{ID: "d", ActorUserID: "", EventType: "skill.updated", ResourceType: "skill", ResourceID: skillUUID, OccurredAt: changelogAt(11)},
		{ID: "e", ActorUserID: "bob", EventType: "skill.updated", ResourceType: "", ResourceID: skillUUID, OccurredAt: changelogAt(20)},
	}
	names := map[string]string{taskUUID: "Nightly Summarizer", agentUUID: "Triage Bot"}

	report := buildChangelogReport(events, names, changelogNow)

	if report.Total != len(events) {
		t.Fatalf("total = %d, want %d", report.Total, len(events))
	}
	wantByType := map[string]int{"task": 2, "task_agent": 1, "skill": 1, "unknown": 1}
	if len(report.ByResourceType) != len(wantByType) {
		t.Errorf("by_resource_type = %v, want %v", report.ByResourceType, wantByType)
	}
	for k, want := range wantByType {
		if got := report.ByResourceType[k]; got != want {
			t.Errorf("by_resource_type[%q] = %d, want %d (full map %v)", k, got, want, report.ByResourceType)
		}
	}
	wantByActor := map[string]int{"alice": 2, "bob": 2, "unknown": 1}
	if len(report.ByActor) != len(wantByActor) {
		t.Errorf("by_actor = %v, want %v", report.ByActor, wantByActor)
	}
	for k, want := range wantByActor {
		if got := report.ByActor[k]; got != want {
			t.Errorf("by_actor[%q] = %d, want %d (full map %v)", k, got, want, report.ByActor)
		}
	}
	// The two skill-uuid events have no mirrored name; the task and agent ones do.
	if report.ResolvedCount != 3 || report.UnresolvedCount != 2 {
		t.Errorf("resolved_count=%d unresolved_count=%d, want 3/2", report.ResolvedCount, report.UnresolvedCount)
	}
	// Events are grouped: every row of a resource type is contiguous.
	seen := map[string]bool{}
	prev := ""
	for i, ev := range report.Events {
		if i > 0 && ev.ResourceType != prev && seen[ev.ResourceType] {
			t.Fatalf("resource type %q reappears after a different type at index %d; grouping broken: %+v",
				ev.ResourceType, i, report.Events)
		}
		seen[ev.ResourceType] = true
		prev = ev.ResourceType
	}
	if report.Note != "" {
		t.Errorf("note = %q, want empty for a non-empty window", report.Note)
	}
}

// TestChangelogEmptyWindowFabricatesNothing is the second absence-of-correctness
// check: an empty window must report zero, marshal events as [] rather than
// null, and say why it is empty.
func TestChangelogEmptyWindowFabricatesNothing(t *testing.T) {
	cases := []struct {
		name   string
		events []auditRow
		names  map[string]string
	}{
		{"nil slice, nil names", nil, nil},
		{"empty slice, populated names", []auditRow{}, map[string]string{"0195d1ff": "Nightly Summarizer"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report := buildChangelogReport(tc.events, tc.names, changelogNow)

			if report.Total != 0 {
				t.Errorf("total = %d, want 0", report.Total)
			}
			if report.Events == nil {
				t.Fatalf("events is nil; it must be an initialized empty slice")
			}
			if len(report.Events) != 0 {
				t.Fatalf("events has %d entries, want 0 — activity was fabricated", len(report.Events))
			}
			if report.ResolvedCount != 0 || report.UnresolvedCount != 0 {
				t.Errorf("resolved_count=%d unresolved_count=%d, want 0/0", report.ResolvedCount, report.UnresolvedCount)
			}
			if report.Note == "" {
				t.Errorf("note is empty; an empty window must explain itself")
			}
			if report.IntegrityChecked {
				t.Errorf("integrity_checked = true without --verify")
			}

			raw, err := json.Marshal(report)
			if err != nil {
				t.Fatalf("marshaling report: %v", err)
			}
			out := string(raw)
			if !strings.Contains(out, `"events":[]`) {
				t.Errorf("marshaled report lacks `\"events\":[]`: %s", out)
			}
			if strings.Contains(out, `"events":null`) {
				t.Errorf("marshaled report has `\"events\":null`: %s", out)
			}
			if !strings.Contains(out, `"by_resource_type":{}`) {
				t.Errorf("marshaled report lacks `\"by_resource_type\":{}`: %s", out)
			}
			if !strings.Contains(out, `"by_actor":{}`) {
				t.Errorf("marshaled report lacks `\"by_actor\":{}`: %s", out)
			}
			if strings.Contains(out, `"integrity"`) {
				t.Errorf("integrity key present without --verify: %s", out)
			}
		})
	}
}
