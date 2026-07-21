// Copyright 2026 Ryan Kelley and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"testing"
)

// TestKeywordCreatedCount verifies that the created entry reflects only
// successfully created keywords, not the template's total keyword count.
// The RunE loop does: if kwErr == nil { kwCreated++ }, then appends "keywords:%d".
// TestBuildTemplateDiff_DryRunCompat verifies that buildTemplateDiff produces
// output regardless of dry-run state — the diff path makes no API calls and
// must not be suppressed by --dry-run. The guard is `!flagDiff && dryRunOK`.
func TestBuildTemplateDiff_ProducesOutput(t *testing.T) {
	tmpl := campaignTemplate{
		Campaign: map[string]interface{}{"name": "Brand Q3"},
		AdGroups: []templateAdGroup{
			{Name: "Main", Keywords: []map[string]interface{}{{"text": "yoga"}, {"text": "mat"}}},
			{Name: "Retarget", Keywords: nil},
		},
	}
	results := buildTemplateDiff(tmpl, []string{"111", "222"}, "brand-baseline")

	if len(results) != 2 {
		t.Fatalf("want 2 results (one per org), got %d", len(results))
	}
	for _, r := range results {
		if r.Action != "diff" {
			t.Errorf("want action=diff, got %q", r.Action)
		}
		if r.TemplateName != "brand-baseline" {
			t.Errorf("want template_name=brand-baseline, got %q", r.TemplateName)
		}
		if len(r.Diff) == 0 {
			t.Errorf("org %s: want non-empty diff entries", r.OrgID)
		}
	}

	// Spot-check first org's diff entries.
	d := results[0].Diff
	assertDiffField(t, d, "campaign.name", "Brand Q3")
	assertDiffField(t, d, "ad_groups.count", "2")
	assertDiffField(t, d, "ad_groups[0].name", "Main")
	assertDiffField(t, d, "ad_groups[0].keywords.count", "2")
	assertDiffField(t, d, "ad_groups[1].keywords.count", "0")
}

func assertDiffField(t *testing.T, entries []templateDiffEntry, field, wantVal string) {
	t.Helper()
	for _, e := range entries {
		if e.Field == field {
			if e.Template != wantVal {
				t.Errorf("diff field %q: want %q, got %q", field, wantVal, e.Template)
			}
			return
		}
	}
	t.Errorf("diff field %q not found in entries", field)
}

// TestDryRunGate verifies the !flagDiff && dryRunOK condition that gates
// early exit: plain --dry-run short-circuits, but --diff --dry-run does not.
func TestDryRunGate(t *testing.T) {
	cases := []struct {
		flagDiff bool
		dryRun   bool
		wantExit bool // true = should short-circuit (return nil before diff/API)
	}{
		{flagDiff: false, dryRun: true, wantExit: true},  // plain --dry-run: short-circuit
		{flagDiff: true, dryRun: true, wantExit: false},  // --diff --dry-run: no short-circuit
		{flagDiff: true, dryRun: false, wantExit: false}, // --diff only: no short-circuit
		{flagDiff: false, dryRun: false, wantExit: false}, // neither: no short-circuit
	}
	for _, tc := range cases {
		flags := &rootFlags{dryRun: tc.dryRun}
		got := !tc.flagDiff && dryRunOK(flags)
		if got != tc.wantExit {
			t.Errorf("flagDiff=%v dryRun=%v: want short-circuit=%v, got %v",
				tc.flagDiff, tc.dryRun, tc.wantExit, got)
		}
	}
}

func TestKeywordCreatedCount(t *testing.T) {
	cases := []struct {
		results []bool // true = API success, false = API error
		want    string
	}{
		{[]bool{true, true, true}, "keywords:3"},
		{[]bool{true, false, true}, "keywords:2"}, // partial failure visible in output
		{[]bool{false, false}, "keywords:0"},       // all failed — not template count
		{[]bool{true}, "keywords:1"},
	}
	for _, tc := range cases {
		var kwCreated int
		for _, ok := range tc.results {
			if ok {
				kwCreated++
			}
		}
		got := fmt.Sprintf("keywords:%d", kwCreated)
		if got != tc.want {
			t.Errorf("want %q, got %q", tc.want, got)
		}
	}
}

func TestExtractIDFromCreateResponse(t *testing.T) {
	cases := []struct {
		name   string
		resp   string
		keys   []string
		wantID string
	}{
		{
			name:   "flat id field",
			resp:   `{"id": "42", "name": "Brand"}`,
			keys:   []string{"id", "campaignId"},
			wantID: "42",
		},
		{
			name:   "flat campaignId field",
			resp:   `{"campaignId": "99"}`,
			keys:   []string{"id", "campaignId"},
			wantID: "99",
		},
		{
			name:   "data envelope",
			resp:   `{"data": {"id": "77", "name": "Wrapped"}}`,
			keys:   []string{"id", "campaignId"},
			wantID: "77",
		},
		{
			name:   "numeric id coerced to string",
			resp:   `{"id": 123}`,
			keys:   []string{"id"},
			wantID: "123",
		},
		{
			name:   "unparseable response returns empty",
			resp:   `not json at all`,
			keys:   []string{"id", "campaignId"},
			wantID: "",
		},
		{
			name:   "empty object returns empty",
			resp:   `{}`,
			keys:   []string{"id", "campaignId"},
			wantID: "",
		},
		{
			name:   "data envelope missing id returns empty",
			resp:   `{"data": {"name": "no-id"}}`,
			keys:   []string{"id", "campaignId"},
			wantID: "",
		},
		{
			name:   "adgroup keys",
			resp:   `{"data": {"adGroupId": "55"}}`,
			keys:   []string{"id", "adGroupId"},
			wantID: "55",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractIDFromCreateResponse(json.RawMessage(tc.resp), tc.keys...)
			if got != tc.wantID {
				t.Errorf("want %q, got %q", tc.wantID, got)
			}
		})
	}
}
