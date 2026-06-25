// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"testing"
	"time"
)

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return ts
}

func raws2(objs ...string) []json.RawMessage {
	out := make([]json.RawMessage, len(objs))
	for i, o := range objs {
		out[i] = json.RawMessage(o)
	}
	return out
}

// TestNonIncrementalWarnRequested locks the explicit-vs-automatic distinction:
// the resource_not_incremental warning fires only when the user explicitly
// passes --since against a non-temporal endpoint, never on the automatic
// stored-cursor re-sync path (which would be noise on every routine sync).
func TestNonIncrementalWarnRequested(t *testing.T) {
	cases := []struct {
		name       string
		sinceParam string
		sinceTS    string
		want       bool
	}{
		{"explicit --since, no temporal param -> warn", "", "2026-06-06T00:00:00Z", true},
		{"automatic cursor, no temporal param -> silent", "", "", false},
		{"explicit --since, endpoint supports it -> no warn", "updated_after", "2026-06-06T00:00:00Z", false},
		{"automatic cursor, endpoint supports it -> no warn", "updated_after", "", false},
	}
	for _, tc := range cases {
		if got := nonIncrementalWarnRequested(tc.sinceParam, tc.sinceTS); got != tc.want {
			t.Errorf("%s: nonIncrementalWarnRequested(%q, %q) = %v, want %v",
				tc.name, tc.sinceParam, tc.sinceTS, got, tc.want)
		}
	}
}

func TestSyncResourceTimestampField(t *testing.T) {
	if got := syncResourceTimestampField("clips"); got != "created_at" {
		t.Errorf("clips timestamp field = %q, want created_at", got)
	}
	if got := syncResourceTimestampField("workspace"); got != "" {
		t.Errorf("workspace timestamp field = %q, want empty", got)
	}
}

func TestPageOldestBefore(t *testing.T) {
	boundary := mustTime(t, "2026-06-01T00:00:00Z")
	// Descending page that crosses the boundary (oldest is older than boundary).
	crossing := raws2(
		`{"id":"a","created_at":"2026-06-03T10:00:00.5Z"}`,
		`{"id":"b","created_at":"2026-05-30T09:00:00Z"}`,
	)
	if !pageOldestBefore(crossing, "created_at", boundary) {
		t.Errorf("crossing page: pageOldestBefore = false, want true")
	}
	// Fully in-window page (all newer than boundary).
	inWindow := raws2(
		`{"id":"a","created_at":"2026-06-05T10:00:00Z"}`,
		`{"id":"b","created_at":"2026-06-02T09:00:00Z"}`,
	)
	if pageOldestBefore(inWindow, "created_at", boundary) {
		t.Errorf("in-window page: pageOldestBefore = true, want false")
	}
	// Missing/unparseable timestamps are skipped; with no usable ts -> false.
	if pageOldestBefore(raws2(`{"id":"a"}`), "created_at", boundary) {
		t.Errorf("no-timestamp page: pageOldestBefore = true, want false")
	}
	// Regression: the OLDEST (evaluated) record carries fractional seconds, which
	// Suno emits. A strict time.RFC3339 parse rejects it, skips the record, and
	// the early-stop never fires; parseClipTime handles fractional seconds.
	fractionalOldest := raws2(
		`{"id":"a","created_at":"2026-06-05T10:00:00Z"}`,
		`{"id":"b","created_at":"2026-05-30T09:00:00.123456Z"}`,
	)
	if !pageOldestBefore(fractionalOldest, "created_at", boundary) {
		t.Errorf("fractional oldest: pageOldestBefore = false, want true (fractional-second created_at must parse)")
	}
}
