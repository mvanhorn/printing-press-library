// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.

package verdict

import (
	"encoding/json"
	"testing"
)

func TestBuildCoverageOneErrorDowngrades(t *testing.T) {
	t.Parallel()
	sources := []Source{
		{Account: "personal", Calendar: "a@example.com", FetchedAt: "2026-08-17T10:00:00Z", UpstreamUpdatedMax: "2026-08-16T09:00:00Z", EtagPresent: true},
		{Account: "work", Calendar: "b@example.com", FetchedAt: "2026-08-17T10:00:01Z", UpstreamUpdatedMax: "2026-08-15T09:00:00Z", EtagPresent: true},
		{Account: "work", Calendar: "c@example.com", FetchedAt: "2026-08-17T10:00:02Z", Error: "GET /calendars/c: HTTP 500"},
	}
	cov := BuildCoverage(sources)
	if cov.Complete {
		t.Errorf("one errored source must force complete=false")
	}
	if cov.Checked != len(sources)-1 {
		t.Errorf("checked = %d, want M-1 = %d", cov.Checked, len(sources)-1)
	}
	if cov.Of != len(sources) {
		t.Errorf("of = %d, want %d", cov.Of, len(sources))
	}
}

func TestBuildCoverageAllCleanIsComplete(t *testing.T) {
	t.Parallel()
	sources := []Source{
		{Account: "personal", Calendar: "a@example.com", FetchedAt: "2026-08-17T10:00:00Z", EtagPresent: true},
		{Account: "work", Calendar: "b@example.com", FetchedAt: "2026-08-17T10:00:01Z", EtagPresent: true},
	}
	cov := BuildCoverage(sources)
	if !cov.Complete || cov.Checked != 2 || cov.Of != 2 {
		t.Errorf("all-clean coverage must be complete 2/2, got %+v", cov)
	}
}

func TestBuildCoverageZeroSourcesNeverConfident(t *testing.T) {
	t.Parallel()
	cov := BuildCoverage(nil)
	if cov.Complete {
		t.Errorf("zero sources can never be complete")
	}
	if cov.Sources == nil {
		t.Errorf("sources must marshal as [], not null")
	}
}

// TestCoverageJSONFieldNames pins the exact wire contract:
// "checked","of","complete","sources" and the per-source field names
// "account","calendar","fetched_at","upstream_updated_max","etag_present","error".
func TestCoverageJSONFieldNames(t *testing.T) {
	t.Parallel()
	cov := BuildCoverage([]Source{
		{Account: "personal", Calendar: "a@example.com", FetchedAt: "2026-08-17T10:00:00Z", UpstreamUpdatedMax: "2026-08-16T09:00:00Z", EtagPresent: true, Error: "boom"},
	})
	raw, err := json.Marshal(cov)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"checked", "of", "complete", "sources"} {
		if _, ok := top[key]; !ok {
			t.Errorf("coverage JSON missing required field %q in %s", key, raw)
		}
	}
	var srcs []map[string]json.RawMessage
	if err := json.Unmarshal(top["sources"], &srcs); err != nil || len(srcs) != 1 {
		t.Fatalf("sources array: err=%v n=%d", err, len(srcs))
	}
	for _, key := range []string{"account", "calendar", "fetched_at", "upstream_updated_max", "etag_present", "error"} {
		if _, ok := srcs[0][key]; !ok {
			t.Errorf("coverage source JSON missing required field %q in %s", key, raw)
		}
	}
}
