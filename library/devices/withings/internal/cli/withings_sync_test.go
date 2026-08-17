// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored tests for the Withings sync id-extraction + list-extraction
// logic and an end-to-end sync into a temp store via a fake transport.

package cli

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/withings/internal/store"
)

// jsonInt renders an int64 as a JSON number literal for inline fixtures.
func jsonInt(n int64) string { return strconv.FormatInt(n, 10) }

func TestWithingsExtractItems(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		listKey string
		want    int
	}{
		{"measuregrps", `{"updatetime":1,"measuregrps":[{"grpid":1},{"grpid":2}]}`, "measuregrps", 2},
		{"activities", `{"activities":[{"date":"2026-06-01"}],"more":false}`, "activities", 1},
		{"series-workouts", `{"series":[{"id":10},{"id":11},{"id":12}],"more":false}`, "series", 3},
		{"devices", `{"devices":[{"deviceid":"abc"}]}`, "devices", 1},
		{"bare array", `[{"grpid":1},{"grpid":2},{"grpid":3}]`, "measuregrps", 3},
		{"empty body", `{"updatetime":123}`, "measuregrps", 0},
		{"fallback to series when listKey absent", `{"series":[{"id":1}]}`, "measuregrps", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items, err := withingsExtractItems(json.RawMessage(tc.body), tc.listKey)
			if err != nil {
				t.Fatalf("withingsExtractItems: %v", err)
			}
			if len(items) != tc.want {
				t.Errorf("got %d items, want %d", len(items), tc.want)
			}
		})
	}
}

func TestWithingsItemID(t *testing.T) {
	cases := []struct {
		name    string
		obj     map[string]any
		idField string
		want    string
	}{
		{"grpid numeric", map[string]any{"grpid": json.Number("1699123456")}, "grpid", "1699123456"},
		{"date string", map[string]any{"date": "2026-06-01"}, "date", "2026-06-01"},
		{"workout id", map[string]any{"id": json.Number("4242")}, "id", "4242"},
		{"timestamp", map[string]any{"timestamp": json.Number("1699999999")}, "timestamp", "1699999999"},
		{"deviceid string", map[string]any{"deviceid": "hash-abc"}, "deviceid", "hash-abc"},
		{"fallback to grpid when idField absent", map[string]any{"grpid": json.Number("7")}, "missing", "7"},
		{"no id", map[string]any{"foo": "bar"}, "id", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := withingsItemID(tc.obj, tc.idField); got != tc.want {
				t.Errorf("withingsItemID(%v, %q) = %q, want %q", tc.obj, tc.idField, got, tc.want)
			}
		})
	}
}

func TestWithingsEnsureID_RekeysToStableID(t *testing.T) {
	// A measure group with only grpid (no "id") should gain an "id" equal to
	// grpid so UpsertBatch keys it deterministically.
	in := json.RawMessage(`{"grpid":1699123456,"category":1,"measures":[{"value":80000,"type":1,"unit":-3}]}`)
	out, err := withingsEnsureID(in, "grpid")
	if err != nil {
		t.Fatalf("withingsEnsureID: %v", err)
	}
	obj, err := store.DecodeJSONObject(out)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if obj["id"] == nil {
		t.Fatal("expected synthesized id")
	}
	if got := store.ResourceIDString(obj["id"]); got != "1699123456" {
		t.Errorf("id = %q, want 1699123456", got)
	}
}

// fakeForm is a withingsSyncForm that returns a canned body per path/action.
type fakeForm struct {
	responses map[string]string // keyed by action
	calls     []map[string]any
}

func (f *fakeForm) WithingsForm(_ context.Context, path string, form map[string]any) (json.RawMessage, error) {
	f.calls = append(f.calls, form)
	action, _ := form["action"].(string)
	if body, ok := f.responses[action]; ok {
		return json.RawMessage(body), nil
	}
	return json.RawMessage(`{}`), nil
}

func TestWithingsSyncResource_EndToEnd(t *testing.T) {
	s, _ := newTestStore(t)

	fake := &fakeForm{responses: map[string]string{
		"getdevice": `{"devices":[{"deviceid":"dev-1","type":"Scale","model":"Body+","battery":"high"},{"deviceid":"dev-2","type":"Watch","model":"ScanWatch","battery":"medium"}]}`,
	}}

	count, dryRun, err := withingsSyncResource(context.Background(), fake, s, "devices", time.Now().AddDate(-1, 0, 0), false)
	if err != nil {
		t.Fatalf("withingsSyncResource: %v", err)
	}
	if dryRun {
		t.Fatal("did not expect dry-run")
	}
	if count != 2 {
		t.Fatalf("stored %d devices, want 2", count)
	}

	// Verify the rows landed in the store under resource_type "devices".
	rows, err := s.List("devices", 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("store has %d device rows, want 2", len(rows))
	}

	// Verify the request carried the right action.
	if len(fake.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(fake.calls))
	}
	if fake.calls[0]["action"] != "getdevice" {
		t.Errorf("action = %v, want getdevice", fake.calls[0]["action"])
	}
}

func TestWithingsSyncResource_MeasureGroups(t *testing.T) {
	s, _ := newTestStore(t)
	epoch := time.Now().Add(-48 * time.Hour).Unix()
	fake := &fakeForm{responses: map[string]string{
		"getmeas": `{"updatetime":1,"measuregrps":[` +
			`{"grpid":111,"category":1,"date":` + jsonInt(epoch) + `,"measures":[{"value":80000,"type":1,"unit":-3}]},` +
			`{"grpid":222,"category":1,"date":` + jsonInt(epoch) + `,"measures":[{"value":79000,"type":1,"unit":-3}]}` +
			`]}`,
	}}
	count, _, err := withingsSyncResource(context.Background(), fake, s, "measure", time.Now().AddDate(-1, 0, 0), false)
	if err != nil {
		t.Fatalf("withingsSyncResource: %v", err)
	}
	if count != 2 {
		t.Fatalf("stored %d measure groups, want 2", count)
	}
	// The measure groups should be loadable by the analytics layer.
	groups, err := loadMeasureGroups(s, time.Now().AddDate(-1, 0, 0))
	if err != nil {
		t.Fatalf("loadMeasureGroups: %v", err)
	}
	if len(groups) != 2 {
		t.Errorf("loadMeasureGroups got %d, want 2", len(groups))
	}
}

func TestWithingsSyncResource_DryRunSentinel(t *testing.T) {
	s, _ := newTestStore(t)
	fake := &fakeForm{responses: map[string]string{
		"getdevice": `{"dry_run":true}`,
	}}
	count, dryRun, err := withingsSyncResource(context.Background(), fake, s, "devices", time.Now().AddDate(-1, 0, 0), false)
	if err != nil {
		t.Fatalf("withingsSyncResource: %v", err)
	}
	if !dryRun {
		t.Error("expected dry-run sentinel to be detected")
	}
	if count != 0 {
		t.Errorf("count = %d, want 0 for dry-run", count)
	}
}

func TestBuildWithingsSyncForm_DateWindows(t *testing.T) {
	cutoff := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	ymd := buildWithingsSyncForm(withingsSyncSpecs["activity"], cutoff)
	if ymd["startdateymd"] != "2026-01-01" {
		t.Errorf("activity startdateymd = %v, want 2026-01-01", ymd["startdateymd"])
	}
	if _, ok := ymd["enddateymd"]; !ok {
		t.Error("activity form missing enddateymd")
	}

	epoch := buildWithingsSyncForm(withingsSyncSpecs["measure"], cutoff)
	if epoch["startdate"] != int(cutoff.Unix()) {
		t.Errorf("measure startdate = %v, want %d", epoch["startdate"], cutoff.Unix())
	}

	dev := buildWithingsSyncForm(withingsSyncSpecs["devices"], cutoff)
	if _, ok := dev["startdate"]; ok {
		t.Error("devices form should carry no date window")
	}
	if dev["action"] != "getdevice" {
		t.Errorf("devices action = %v, want getdevice", dev["action"])
	}
}
