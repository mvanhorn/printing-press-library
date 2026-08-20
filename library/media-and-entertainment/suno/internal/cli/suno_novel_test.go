// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
	"time"
)

func TestValidateReadOnlySQL(t *testing.T) {
	cases := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{"plain select", "SELECT * FROM clips", false},
		{"select count", "SELECT count(*) AS n FROM clips", false},
		{"lowercase select", "select id from clips limit 5", false},
		{"with cte select", "WITH t AS (SELECT id FROM clips) SELECT * FROM t", false},
		{"trailing semicolon ok", "SELECT 1;", false},
		{"leading whitespace", "   SELECT 1", false},
		{"drop rejected", "DROP TABLE clips", true},
		{"delete rejected", "DELETE FROM clips", true},
		{"insert rejected", "INSERT INTO clips VALUES (1)", true},
		{"update rejected", "UPDATE clips SET title='x'", true},
		{"alter rejected", "ALTER TABLE clips ADD COLUMN x TEXT", true},
		{"create rejected", "CREATE TABLE x (a int)", true},
		{"attach rejected", "ATTACH DATABASE 'x' AS y", true},
		{"pragma rejected", "PRAGMA user_version = 9", true},
		{"multiple statements rejected", "SELECT 1; DROP TABLE clips", true},
		{"with-then-delete rejected", "WITH t AS (SELECT id FROM clips) DELETE FROM clips", true},
		{"with-then-delete newline-bypass rejected", "WITH t AS (SELECT 1) DELETE\nFROM clips WHERE 1=1", true},
		{"with-then-insert tab-bypass rejected", "WITH t AS (SELECT 1) INSERT\tINTO clips VALUES (1)", true},
		{"comment-hidden write rejected", "SELECT 1 -- ok\n; DELETE FROM clips", true},
		{"empty rejected", "   ", true},
		{"non-select rejected", "EXPLAIN SELECT 1", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateReadOnlySQL(tc.query)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateReadOnlySQL(%q) err=%v, wantErr=%v", tc.query, err, tc.wantErr)
			}
		})
	}
}

func TestAnalyticsGroupExpr(t *testing.T) {
	cases := []struct {
		name    string
		groupBy string
		want    string
		wantErr bool
	}{
		{"known column", "model_name", `"model_name"`, false},
		{"status column", "status", `"status"`, false},
		{"json path", "metadata.duration_type", `json_extract(data, '$.metadata.duration_type')`, false},
		{"empty rejected", "", "", true},
		{"injection rejected", "model_name; DROP TABLE clips", "", true},
		{"spaces rejected", "model name", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := analyticsGroupExpr(tc.groupBy)
			if (err != nil) != tc.wantErr {
				t.Fatalf("analyticsGroupExpr(%q) err=%v wantErr=%v", tc.groupBy, err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.want {
				t.Fatalf("analyticsGroupExpr(%q)=%q want %q", tc.groupBy, got, tc.want)
			}
		})
	}
}

func TestParseCredits(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantCred   int64
		wantPlan   string
		wantPeriod string
	}{
		{"total_credits_left wins", `{"total_credits_left":150,"credits":999,"plan":"pro"}`, 150, "pro", ""},
		{"falls back to credits", `{"credits":42}`, 42, "", ""},
		{"absent fields default zero", `{"foo":"bar"}`, 0, "", ""},
		{"subscription_type plan", `{"credits":5,"subscription_type":"premier"}`, 5, "premier", ""},
		{"empty body", `{}`, 0, "", ""},
		// Suno's live shape: plan is a nested object (plan.name / plan.plan_key),
		// subscription_type is a bool, and period holds the billing interval.
		{"nested plan object", `{"credits":0,"period":"year","subscription_type":true,"plan":{"plan_key":"premier","name":"Premier Plan","level":30}}`, 0, "Premier Plan", "year"},
		{"nested plan key only", `{"plan":{"plan_key":"basic"}}`, 0, "basic", ""},
		// period is the billing interval, never the plan name.
		{"period is not a plan", `{"period":"year"}`, 0, "", "year"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseCredits(json.RawMessage(tc.body))
			if got.Credits != tc.wantCred {
				t.Fatalf("credits=%d want %d", got.Credits, tc.wantCred)
			}
			if got.Plan != tc.wantPlan {
				t.Fatalf("plan=%q want %q", got.Plan, tc.wantPlan)
			}
			if got.Period != tc.wantPeriod {
				t.Fatalf("period=%q want %q", got.Period, tc.wantPeriod)
			}
		})
	}
}

func TestParseClipTimeAndWindow(t *testing.T) {
	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	window := 7 * 24 * time.Hour
	cutoff := now.Add(-window)

	cases := []struct {
		name     string
		ts       string
		inWindow bool
	}{
		{"rfc3339 recent", "2026-05-27T10:00:00Z", true},
		{"rfc3339 old", "2026-04-01T10:00:00Z", false},
		{"date only recent", "2026-05-25", true},
		{"date only old", "2026-01-01", false},
		{"space layout", "2026-05-27 09:00:00", true},
		{"unparseable", "not-a-date", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parsed, ok := parseClipTime(tc.ts)
			in := ok && parsed.After(cutoff)
			if in != tc.inWindow {
				t.Fatalf("parseClipTime(%q) inWindow=%v want %v", tc.ts, in, tc.inWindow)
			}
		})
	}
}

func TestLineageParentRefs(t *testing.T) {
	data := json.RawMessage(`{
		"metadata": {
			"history": ["parent-a", {"id":"parent-b"}],
			"concat_history": [{"clip_id":"seg-1"}],
			"cover_clip_id": "cover-src"
		}
	}`)
	refs := lineageParentRefs(data)
	got := map[string]string{}
	for _, r := range refs {
		got[r.id] = r.relation
	}
	want := map[string]string{
		"parent-a":  "extend",
		"parent-b":  "extend",
		"seg-1":     "concat",
		"cover-src": "cover",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d refs (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for id, rel := range want {
		if got[id] != rel {
			t.Fatalf("ref %q relation=%q want %q", id, got[id], rel)
		}
	}

	// No metadata -> no refs (single-node case).
	if refs := lineageParentRefs(json.RawMessage(`{"id":"x"}`)); len(refs) != 0 {
		t.Fatalf("expected no refs for clip without lineage metadata, got %v", refs)
	}
}
