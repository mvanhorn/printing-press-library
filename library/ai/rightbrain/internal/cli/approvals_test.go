// Copyright 2026 Farouk Umar and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestNovelApprovalsHelpWires smoke-tests that the approvals command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelApprovalsHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"approvals", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("approvals --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "approvals"} {
		if !strings.Contains(help, want) {
			t.Fatalf("approvals --help missing %q in output:\n%s", want, help)
		}
	}
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	ts, ok := parseAPITime(s)
	if !ok {
		t.Fatalf("parseAPITime(%q) failed", s)
	}
	return ts
}

// TestApprovalsSortsOldestFirst pins the ordering contract: the queue exists to
// answer "what has been stuck longest", so the oldest parked request must lead.
func TestApprovalsSortsOldestFirst(t *testing.T) {
	now := mustTime(t, "2026-07-31T12:00:00Z")
	records := []approvalRecord{
		{ID: "recent", Status: "pending", Created: "2026-07-31T11:30:00Z", ToolName: "send_email"},
		{ID: "oldest", Status: "pending", Created: "2026-07-29T12:00:00Z", ToolName: "charge_card"},
		{ID: "middle", Status: "pending", Created: "2026-07-31T06:00:00Z", ToolName: "post_message"},
	}
	got := buildApprovalsReport(records, now, 0, 0, "")
	if len(got.Pending) != 3 {
		t.Fatalf("pending = %d, want 3", len(got.Pending))
	}
	want := []string{"oldest", "middle", "recent"}
	for i, id := range want {
		if got.Pending[i].ID != id {
			t.Errorf("Pending[%d].ID = %q, want %q", i, got.Pending[i].ID, id)
		}
	}
	if got.OldestSecs != 48*3600 {
		t.Errorf("OldestSecs = %v, want %v", got.OldestSecs, 48*3600)
	}
}

// TestApprovalsSeparatesExpired covers the triage split that makes this command
// worth running: an approval whose window lapsed needs a re-run, not a decision,
// so it must never sit in the actionable queue.
func TestApprovalsSeparatesExpired(t *testing.T) {
	now := mustTime(t, "2026-07-31T12:00:00Z")
	records := []approvalRecord{
		{ID: "live", Status: "pending", Created: "2026-07-31T10:00:00Z", ExpiresAt: "2026-08-01T10:00:00Z"},
		{ID: "lapsed-by-time", Status: "pending", Created: "2026-07-30T10:00:00Z", ExpiresAt: "2026-07-31T09:00:00Z"},
		{ID: "lapsed-by-status", Status: "expired", Created: "2026-07-30T08:00:00Z"},
	}
	got := buildApprovalsReport(records, now, 0, 0, "")

	if len(got.Pending) != 1 || got.Pending[0].ID != "live" {
		t.Fatalf("Pending = %+v, want only the un-expired request", got.Pending)
	}
	if !got.Pending[0].Actionable {
		t.Error("the un-expired pending request should be Actionable")
	}
	if len(got.Expired) != 2 {
		t.Fatalf("Expired = %d, want 2", len(got.Expired))
	}
	for _, v := range got.Expired {
		if v.Actionable {
			t.Errorf("expired request %q must not be Actionable", v.ID)
		}
		if !v.Expired {
			t.Errorf("request %q should be flagged Expired", v.ID)
		}
	}
	// expires_in for a lapsed request must be negative, not clamped to zero —
	// "expired 3h ago" is actionable information.
	for _, v := range got.Expired {
		if v.ID == "lapsed-by-time" && v.ExpiresInSecs >= 0 {
			t.Errorf("lapsed request ExpiresInSecs = %v, want negative", v.ExpiresInSecs)
		}
	}
}

// TestApprovalsOlderThanFilter checks the staleness filter both includes and
// excludes correctly, rather than only checking that something came back.
func TestApprovalsOlderThanFilter(t *testing.T) {
	now := mustTime(t, "2026-07-31T12:00:00Z")
	records := []approvalRecord{
		{ID: "fresh", Status: "pending", Created: "2026-07-31T11:59:00Z"}, // 1m old
		{ID: "stale", Status: "pending", Created: "2026-07-31T09:00:00Z"}, // 3h old
	}
	got := buildApprovalsReport(records, now, time.Hour, 0, "1h")
	if len(got.Pending) != 1 || got.Pending[0].ID != "stale" {
		t.Fatalf("Pending = %+v, want only the 3h-old request", got.Pending)
	}
	// Counts are pre-filter: the caller still deserves to know the fresh one exists.
	if got.Counts["pending"] != 2 {
		t.Errorf("Counts[pending] = %d, want 2 (counts are pre-filter)", got.Counts["pending"])
	}
}

// TestApprovalsEmptyIsHonest is the absence-of-correctness case: nothing parked
// must produce an explicit empty result and a note, never a fabricated row, and
// the JSON must carry [] rather than null so agents can iterate it safely.
func TestApprovalsEmptyIsHonest(t *testing.T) {
	now := mustTime(t, "2026-07-31T12:00:00Z")

	got := buildApprovalsReport(nil, now, 0, 0, "")
	if len(got.Pending) != 0 || len(got.Expired) != 0 {
		t.Fatalf("expected no rows, got pending=%d expired=%d", len(got.Pending), len(got.Expired))
	}
	if got.Note == "" {
		t.Error("empty result must carry an explanatory note")
	}
	if got.OldestSecs != 0 {
		t.Errorf("OldestSecs = %v, want 0 when nothing is parked", got.OldestSecs)
	}

	blob, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{`"pending":[]`, `"expired":[]`} {
		if !strings.Contains(string(blob), want) {
			t.Errorf("JSON missing %s (null slices break agent iteration): %s", want, blob)
		}
	}

	// Filtering everything out must explain how to widen the sweep, and must
	// name the flag rather than silently returning nothing.
	filtered := buildApprovalsReport(
		[]approvalRecord{{ID: "fresh", Status: "pending", Created: "2026-07-31T11:59:00Z"}},
		now, 24*time.Hour, 0, "1d")
	if len(filtered.Pending) != 0 {
		t.Fatalf("expected the fresh request to be filtered out, got %+v", filtered.Pending)
	}
	if !strings.Contains(filtered.Note, "--older-than") {
		t.Errorf("filtered-empty note should name --older-than, got %q", filtered.Note)
	}
}

// TestApprovalsUnparseableTimestamp guards the degradation path: a timestamp the
// API renders in an unexpected shape must read as "unknown" rather than 1970,
// which would otherwise sort to the top of the queue as the oldest item.
func TestApprovalsUnparseableTimestamp(t *testing.T) {
	now := mustTime(t, "2026-07-31T12:00:00Z")
	records := []approvalRecord{
		{ID: "bad-ts", Status: "pending", Created: "not-a-timestamp"},
		{ID: "good-ts", Status: "pending", Created: "2026-07-31T11:00:00Z"},
	}
	got := buildApprovalsReport(records, now, 0, 0, "")

	var bad approvalView
	for _, v := range got.Pending {
		if v.ID == "bad-ts" {
			bad = v
		}
	}
	if bad.ParkedHuman != "unknown" {
		t.Errorf("ParkedHuman = %q, want %q", bad.ParkedHuman, "unknown")
	}
	if bad.ParkedSeconds != 0 {
		t.Errorf("ParkedSeconds = %v, want 0 for an unparseable timestamp", bad.ParkedSeconds)
	}
	if got.Pending[0].ID == "bad-ts" {
		t.Error("an unparseable timestamp must not sort to the top as the oldest item")
	}
}

// TestApprovalsUnknownAgeSurvivesOlderThan covers the interaction between an
// unparseable created stamp and --older-than. Such a record has ParkedSeconds
// 0 only because its age is unknown, so any positive --older-than swept it out
// of both output sections while Counts still counted it: the JSON's counts did
// not match its own rows, and a genuinely parked approval was invisible.
// An unknown age cannot be proven recent, so the filter must not apply to it.
func TestApprovalsUnknownAgeSurvivesOlderThan(t *testing.T) {
	now := mustTime(t, "2026-07-31T12:00:00Z")
	records := []approvalRecord{
		{ID: "unknown-age", Status: "pending", Created: "not-a-timestamp", ToolName: "charge_card"},
		{ID: "old", Status: "pending", Created: "2026-07-30T12:00:00Z", ToolName: "send_email"},
		{ID: "fresh", Status: "pending", Created: "2026-07-31T11:59:00Z", ToolName: "post_message"},
	}

	got := buildApprovalsReport(records, now, time.Hour, 0, "1h")

	ids := make([]string, 0, len(got.Pending))
	for _, v := range got.Pending {
		ids = append(ids, v.ID)
	}
	found := false
	for _, id := range ids {
		if id == "unknown-age" {
			found = true
		}
		if id == "fresh" {
			t.Errorf("a request parked 1m survived --older-than 1h; pending = %v", ids)
		}
	}
	if !found {
		t.Fatalf("the request with an unreadable created stamp was filtered out by --older-than; pending = %v", ids)
	}
	if len(got.Pending) != 2 {
		t.Fatalf("pending = %v, want the old request and the unknown-age one", ids)
	}

	// The rows and the counts must describe the same set of records: every
	// pending record that Counts counted but --older-than could not judge has
	// to be visible somewhere.
	if got.Counts["pending"] != 3 {
		t.Errorf("Counts[pending] = %d, want 3 (counts are pre-filter)", got.Counts["pending"])
	}
	unknown := got.Pending[len(got.Pending)-1]
	if unknown.ID != "unknown-age" {
		unknown = got.Pending[0]
	}
	if unknown.ParkedHuman != "unknown" {
		t.Errorf("parked = %q, want %q so the row is not read as brand new", unknown.ParkedHuman, "unknown")
	}
	if !unknown.ParkedUnknown {
		t.Errorf("parked_unknown = false, want true: 0 seconds here means unmeasured, not fresh")
	}
	if !unknown.Actionable {
		t.Errorf("actionable = false, want true: a pending, unexpired request still needs a decision")
	}

	// An expired record with an unreadable stamp must reach the expired
	// section rather than vanish from both.
	expired := buildApprovalsReport(
		[]approvalRecord{{ID: "gone", Status: "expired", Created: "nope"}},
		now, time.Hour, 0, "1h")
	if len(expired.Expired) != 1 {
		t.Fatalf("expired = %+v, want the unknown-age expired record surfaced", expired.Expired)
	}
}
