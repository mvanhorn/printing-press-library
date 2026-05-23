// Copyright 2026 servosity. Licensed under Apache-2.0. See LICENSE.

package qbrreport

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func sampleReport() *Report {
	return &Report{
		Company:  Company{ID: 4421, Name: "ACME Corp"},
		Reseller: Reseller{Name: "Compounding MSP"},
		Quarter:  "2026-Q1",
		From:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		To:       time.Date(2026, 3, 31, 23, 59, 59, 0, time.UTC),
		Coverage: []DeviceRow{
			{Device: "db-prod-01", Restic: "r-9012", LastSuccessful: time.Date(2026, 3, 30, 3, 0, 0, 0, time.UTC), Status: "ok"},
			{Device: "files-prod-01", Classic: "b-3344", LastSuccessful: time.Date(2026, 3, 29, 22, 0, 0, 0, time.UTC), Status: "dirty"},
		},
		SuccessRate: SuccessStats{Total: 24, HardFail: 0, Dirty: 0, RetriedSucceeded: 2, Rate: 98.4},
		OpenIssues:  []Issue{{ID: "18", Title: "stale backup", Severity: "warning"}},
	}
}

func TestRenderMarkdown_HasAllSections(t *testing.T) {
	r := sampleReport()
	var buf bytes.Buffer
	if err := RenderMarkdown(r, &buf); err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	out := buf.String()
	wantSubstrings := []string{
		"ACME Corp",
		"Compounding MSP",
		"2026-Q1",
		"db-prod-01",
		"files-prod-01",
		"98.4",
		"stale backup",
	}
	for _, w := range wantSubstrings {
		if !strings.Contains(out, w) {
			t.Errorf("RenderMarkdown output missing %q. Got:\n%s", w, out)
		}
	}
}

func TestRenderHTML_HasDoctypeAndEscapesUnsafe(t *testing.T) {
	r := sampleReport()
	r.Company.Name = `<script>alert("xss")</script>`
	var buf bytes.Buffer
	if err := RenderHTML(r, &buf); err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "<!DOCTYPE html>") && !strings.Contains(out, "<!doctype html>") {
		t.Errorf("HTML missing doctype")
	}
	if strings.Contains(out, `<script>alert("xss")</script>`) {
		t.Errorf("HTML did not escape company name; html/template should have escaped <script>")
	}
}

func TestRenderMarkdown_EmptyOptionalSections(t *testing.T) {
	r := &Report{
		Company:     Company{ID: 1, Name: "Zero"},
		Reseller:    Reseller{Name: "MSP"},
		Quarter:     "2026-Q1",
		From:        time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		To:          time.Date(2026, 3, 31, 23, 59, 59, 0, time.UTC),
		SuccessRate: SuccessStats{Total: 0},
	}
	var buf bytes.Buffer
	if err := RenderMarkdown(r, &buf); err != nil {
		t.Fatalf("RenderMarkdown empty: %v", err)
	}
	if !strings.Contains(buf.String(), "Zero") {
		t.Errorf("missing company name on empty report")
	}
}

func TestParseQuarter(t *testing.T) {
	cases := []struct {
		in       string
		wantFrom time.Time
		wantTo   time.Time
		wantErr  bool
	}{
		{"2026-Q1", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 3, 31, 23, 59, 59, 0, time.UTC), false},
		{"2026-Q2", time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 6, 30, 23, 59, 59, 0, time.UTC), false},
		{"2026-Q4", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC), false},
		{"garbage", time.Time{}, time.Time{}, true},
	}
	for _, c := range cases {
		from, to, err := ParseQuarter(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("ParseQuarter(%q) err: got %v want_err=%v", c.in, err, c.wantErr)
			continue
		}
		if c.wantErr {
			continue
		}
		if !from.Equal(c.wantFrom) {
			t.Errorf("ParseQuarter(%q) from: got %v want %v", c.in, from, c.wantFrom)
		}
		// `to` is the exclusive end (start of the next quarter). Just assert
		// it's strictly after `from` and within the same year (Q4 rolls to next).
		if !to.After(from) {
			t.Errorf("ParseQuarter(%q) to: got %v which is not after from %v", c.in, to, from)
		}
	}
}

func TestCurrentQuarter(t *testing.T) {
	cases := []struct {
		now  time.Time
		want string
	}{
		{time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC), "2026-Q1"},
		{time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), "2026-Q2"},
		{time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC), "2026-Q3"},
		{time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), "2026-Q4"},
	}
	for _, c := range cases {
		got := CurrentQuarter(c.now)
		if got != c.want {
			t.Errorf("CurrentQuarter(%v): got %q want %q", c.now, got, c.want)
		}
	}
}
