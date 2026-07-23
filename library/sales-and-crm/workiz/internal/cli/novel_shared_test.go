// Copyright 2026 Eldar and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseWorkizTime(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantOK  bool
		wantISO string
	}{
		{"standard format", "2026-07-06 09:00:00", true, "2026-07-06T09:00:00Z"},
		{"rfc3339", "2026-07-06T09:00:00Z", true, "2026-07-06T09:00:00Z"},
		{"literal null", "null", false, ""},
		{"empty string", "", false, ""},
		{"date only", "2026-07-06", true, "2026-07-06T00:00:00Z"},
		{"garbage", "not-a-date", false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseWorkizTime(tc.input)
			if ok != tc.wantOK {
				t.Fatalf("parseWorkizTime(%q) ok = %v, want %v", tc.input, ok, tc.wantOK)
			}
			if ok {
				want, _ := time.Parse(time.RFC3339, tc.wantISO)
				if !got.Equal(want) {
					t.Fatalf("parseWorkizTime(%q) = %v, want %v", tc.input, got, want)
				}
			}
		})
	}
}

func TestParseMoney(t *testing.T) {
	cases := []struct {
		input string
		want  float64
	}{
		{"450.00", 450.00},
		{"", 0},
		{"not-a-number", 0},
		{"0.00", 0},
		{"1234.56", 1234.56},
	}
	for _, tc := range cases {
		if got := parseMoney(flexibleMoney(tc.input)); got != tc.want {
			t.Errorf("parseMoney(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

// TestIsMissingMoney is the Greptile 4/5 → 5/5 fix: job audit must not treat
// an explicit $0 total (warranty / free estimate) as "price not recorded".
func TestIsMissingMoney(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"", true},
		{"   ", true},
		{"0", false},
		{"0.00", false},
		{"450.00", false},
		{"171.36", false},
	}
	for _, tc := range cases {
		if got := isMissingMoney(flexibleMoney(tc.input)); got != tc.want {
			t.Errorf("isMissingMoney(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

// TestJobAuditDoesNotFlagZeroPrice ensures the audit predicate used at the
// call site treats a recorded zero total as present (not a billing gap).
func TestJobAuditDoesNotFlagZeroPrice(t *testing.T) {
	// Wire shape confirmed live: JobTotalPrice arrives as JSON number 0.
	raw := `{"UUID":"FREE01","JobTotalPrice":0,"Phone":"3035550100","Email":"a@b.c","Address":"1 Main","Team":[{"Name":"Tech"}]}`
	var j wzJob
	if err := json.Unmarshal([]byte(raw), &j); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if isMissingMoney(j.JobTotalPrice) {
		t.Fatalf("JobTotalPrice=0 must not be treated as missing; got missing=true (would false-positive warranty/free jobs)")
	}
	if parseMoney(j.JobTotalPrice) != 0 {
		t.Fatalf("parseMoney still returns 0 for free jobs; got %v", parseMoney(j.JobTotalPrice))
	}

	// Truly missing (null) still flags.
	rawMissing := `{"UUID":"MISS01","JobTotalPrice":null}`
	var missing wzJob
	if err := json.Unmarshal([]byte(rawMissing), &missing); err != nil {
		t.Fatalf("unmarshal missing: %v", err)
	}
	if !isMissingMoney(missing.JobTotalPrice) {
		t.Fatalf("null JobTotalPrice must be treated as missing")
	}
}

func TestWzCommentsUnmarshal(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty string means no comments", `""`, nil},
		{"array of comment objects", `[{"Comment":"first"},{"Comment":"second"}]`, []string{"first", "second"}},
		{"null", `null`, nil},
		{"unrecognized shape treated as no comments", `{"unexpected":"object"}`, nil},
		{"single free-text string (confirmed live wire shape)", `"Left VM for second visit"`, []string{"Left VM for second visit"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var c wzComments
			if err := json.Unmarshal([]byte(tc.input), &c); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			if len(c) != len(tc.want) {
				t.Fatalf("got %v, want %v", []string(c), tc.want)
			}
			for i := range tc.want {
				if c[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", []string(c), tc.want)
				}
			}
		})
	}
}

// TestFlexibleMoneyUnmarshal regression-tests a bug found via live testing:
// wzJob.JobTotalPrice/JobAmountDue were declared as plain string, but live
// Workiz responses return these as JSON numbers (int or float), so
// json.Unmarshal on the whole wzJob struct failed and loadJobs silently
// dropped every real job row (json.Unmarshal fails the entire struct on any
// single field type mismatch, not just that field).
func TestFlexibleMoneyUnmarshal(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  flexibleMoney
	}{
		{"integer (confirmed live wire shape)", `0`, "0"},
		{"float (confirmed live wire shape)", `171.36`, "171.36"},
		{"string", `"450.00"`, "450.00"},
		{"null", `null`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var f flexibleMoney
			if err := json.Unmarshal([]byte(tc.input), &f); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			if f != tc.want {
				t.Fatalf("got %q, want %q", f, tc.want)
			}
		})
	}
}

// TestWzJobUnmarshalWithNumericPrice is a regression test at the whole-struct
// level (not just the flexibleMoney type) for the same live-data bug: this is
// exactly the JSON shape a real Workiz job returns.
func TestWzJobUnmarshalWithNumericPrice(t *testing.T) {
	raw := `{"UUID":"Y388BN","JobTotalPrice":0,"JobAmountDue":0,"Comments":"Left VM for second visit"}`
	var j wzJob
	if err := json.Unmarshal([]byte(raw), &j); err != nil {
		t.Fatalf("unmarshal error (this used to silently drop the whole row): %v", err)
	}
	if j.UUID != "Y388BN" {
		t.Fatalf("UUID = %q, want Y388BN", j.UUID)
	}
	if len(j.Comments) != 1 || j.Comments[0] != "Left VM for second visit" {
		t.Fatalf("Comments = %v, want single free-text comment", []string(j.Comments))
	}
}

func TestFlexibleIDUnmarshal(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  flexibleID
	}{
		{"numeric id (Job.Team[].id)", `1`, "1"},
		{"string id (Lead.Team[].id)", `"1"`, "1"},
		{"unrecognized shape falls back to zero value", `true`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var f flexibleID
			if err := json.Unmarshal([]byte(tc.input), &f); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			if f != tc.want {
				t.Fatalf("got %q, want %q", f, tc.want)
			}
		})
	}
}

func TestSnippetAround(t *testing.T) {
	cases := []struct {
		name string
		text string
		term string
		want string
	}{
		{"short text returned as-is", "short note", "note", "short note"},
		{"match near start truncates with trailing ellipsis", "leak under the sink was fixed today", "leak", "leak under the sink was fixed toda..."},
		{"no match falls back to prefix", "some unrelated text here", "zzz", "some unrelated text here"},
		{
			// snippetAround expects an already-lowercased term (its only real
			// caller, job_search.go, lowercases the query before calling it).
			"match in the middle produces both leading and trailing ellipsis",
			"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA MATCH BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
			"match",
			"...AAAAAAAAAAAAAAAAAAAAAAAAAAAAA MATCH BBBBBBBBBBBBBBBBBBBBBBBBBBBBB...",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := snippetAround(tc.text, tc.term)
			if got != tc.want {
				t.Fatalf("snippetAround(%q, %q) = %q, want %q", tc.text, tc.term, got, tc.want)
			}
		})
	}
}

func TestSnippetAroundUnicodePanicRegression(t *testing.T) {
	prefix := ""
	for i := 0; i < 60; i++ {
		prefix += "Ⱥ"
	}
	text := prefix + "leak"
	got := snippetAround(text, "leak")
	if got == "" {
		t.Fatal("expected non-empty snippet")
	}
}

// TestMatchLeadToJobMissingCreatedDate regression-tests a Greptile finding:
// when a lead's CreatedDate is missing/unparseable, the chronology guard used
// to short-circuit entirely, silently accepting the first contact-matching
// job in iteration order regardless of when it was created. matchLeadToJob
// must instead pick deterministically (the earliest-created match) rather
// than depend on iteration order.
func TestMatchLeadToJobMissingCreatedDate(t *testing.T) {
	lead := wzLead{Email: "jane@example.com", CreatedDate: "not-a-date"}
	jobs := []wzJob{
		{UUID: "job-late", Email: "jane@example.com", CreatedDate: "2026-08-01 10:00:00"},
		{UUID: "job-early", Email: "jane@example.com", CreatedDate: "2026-07-01 10:00:00"},
		{UUID: "job-no-date", Email: "jane@example.com", CreatedDate: ""},
	}
	got, found := matchLeadToJob(lead, jobs)
	if !found {
		t.Fatal("expected a match")
	}
	if got.UUID != "job-early" {
		t.Fatalf("expected the earliest-created contact match (job-early), got %q", got.UUID)
	}
}

// TestMatchLeadToJobUndatedFirstDoesNotBlockDatedCandidate regression-tests a
// Greptile finding on the initial fix: when the first contact-matching job in
// iteration order has no parseable CreatedDate, bestCreated stayed at the zero
// time.Time value, and no later job with a real date could satisfy
// jobCreated.Before(zero) to displace it — an undated match would win forever
// even when a dated (more trustworthy) candidate existed later in the slice.
func TestMatchLeadToJobUndatedFirstDoesNotBlockDatedCandidate(t *testing.T) {
	lead := wzLead{Email: "jane@example.com", CreatedDate: "2026-07-01 10:00:00"}
	jobs := []wzJob{
		{UUID: "job-no-date-first", Email: "jane@example.com", CreatedDate: ""},
		{UUID: "job-dated", Email: "jane@example.com", CreatedDate: "2026-08-01 10:00:00"},
	}
	got, found := matchLeadToJob(lead, jobs)
	if !found {
		t.Fatal("expected a match")
	}
	if got.UUID != "job-dated" {
		t.Fatalf("expected the dated candidate to win over the undated first match, got %q", got.UUID)
	}
}

// TestMatchLeadToJobRejectsJobPredatingLead confirms the chronology guard
// still rejects an obviously-earlier job when both dates are known.
func TestMatchLeadToJobRejectsJobPredatingLead(t *testing.T) {
	lead := wzLead{Email: "jane@example.com", CreatedDate: "2026-07-01 10:00:00"}
	jobs := []wzJob{
		{UUID: "job-before-lead", Email: "jane@example.com", CreatedDate: "2026-06-01 10:00:00"},
	}
	_, found := matchLeadToJob(lead, jobs)
	if found {
		t.Fatal("expected no match: the only candidate job predates the lead")
	}
}

// TestMatchLeadToJobToleratesNearSimultaneousCreation regression-tests a bug
// found via live testing against a real Workiz account: the "AI Call" intake
// integration creates the job record 2-3 seconds *before* the lead record
// for the same contact (both written near-simultaneously by the same
// automated flow). A strict jobCreated >= leadCreated guard rejected every
// real conversion from that source across 6 confirmed live examples.
func TestMatchLeadToJobToleratesNearSimultaneousCreation(t *testing.T) {
	lead := wzLead{Email: "jane@example.com", CreatedDate: "2026-07-03 10:31:45"}
	jobs := []wzJob{
		{UUID: "job-ai-call", Email: "jane@example.com", CreatedDate: "2026-07-03 10:31:43"},
	}
	got, found := matchLeadToJob(lead, jobs)
	if !found {
		t.Fatal("expected a match: job created 2s before the lead is within the chronology grace window")
	}
	if got.UUID != "job-ai-call" {
		t.Fatalf("got %q, want job-ai-call", got.UUID)
	}
}

// TestMatchLeadToJobNoContactMatch confirms unrelated jobs are never matched.
func TestMatchLeadToJobNoContactMatch(t *testing.T) {
	lead := wzLead{Email: "jane@example.com", CreatedDate: "2026-07-01 10:00:00"}
	jobs := []wzJob{
		{UUID: "job-unrelated", Email: "someone-else@example.com", CreatedDate: "2026-08-01 10:00:00"},
	}
	_, found := matchLeadToJob(lead, jobs)
	if found {
		t.Fatal("expected no match: no contact-identity overlap")
	}
}

// TestWzLeadSourceWireTag regression-tests a research bug found via live
// testing: leads report their source under the "JobSource" wire key, not
// "LeadSource" as originally documented from the create-lead body fields.
// LeadSource never appears in any real synced lead across 53 live records.
func TestWzLeadSourceWireTag(t *testing.T) {
	raw := `{"UUID":"0FX7LS","JobSource":"Yelp"}`
	var l wzLead
	if err := json.Unmarshal([]byte(raw), &l); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if l.LeadSource != "Yelp" {
		t.Fatalf("LeadSource = %q, want %q (wire key is JobSource, not LeadSource)", l.LeadSource, "Yelp")
	}
}

// TestFilterLeadsSince regression-tests a Greptile finding: the --since
// window filter's condition `ok && t.Before(cutoff)` short-circuited to false
// for leads with an empty/unparseable CreatedDate, silently including them
// regardless of the requested time window.
func TestFilterLeadsSince(t *testing.T) {
	cutoff, _ := time.Parse("2006-01-02 15:04:05", "2026-06-01 00:00:00")
	leads := []wzLead{
		{UUID: "lead-recent", CreatedDate: "2026-06-15 10:00:00"},
		{UUID: "lead-old", CreatedDate: "2026-05-01 10:00:00"},
		{UUID: "lead-undated", CreatedDate: ""},
		{UUID: "lead-unparseable", CreatedDate: "not-a-date"},
	}
	got := filterLeadsSince(leads, cutoff)
	if len(got) != 1 || got[0].UUID != "lead-recent" {
		ids := make([]string, len(got))
		for i, l := range got {
			ids[i] = l.UUID
		}
		t.Fatalf("filterLeadsSince = %v, want only [lead-recent] (old, undated, and unparseable leads must all be excluded)", ids)
	}
}

// TestResolveTimeOffOwner regression-tests a Greptile finding: TimeOff.UserName
// is a login/username, not the display name job.Team[].Name reports, so a bare
// string comparison between the two never matched and time_off_conflict
// entries in team bottleneck were unreachable in practice. Resolution goes
// through the shared team roster's email field (the only field common to
// both the timeoff and team resources).
func TestResolveTimeOffOwner(t *testing.T) {
	roster := buildUsernameToDisplayName([]wzTeamMember{
		{Name: "Dmitriy Petrenko", Email: "dmitriy.petrenko@example.com"},
		{Name: "No Email Member", Email: ""},
	})

	cases := []struct {
		name     string
		userName string
		want     string
	}{
		{"matches full email case-insensitively", "Dmitriy.Petrenko@Example.com", "Dmitriy Petrenko"},
		{"matches email local-part", "dmitriy.petrenko", "Dmitriy Petrenko"},
		{"no roster match falls back to raw value", "someone.else", "someone.else"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveTimeOffOwner(tc.userName, roster)
			if got != tc.want {
				t.Fatalf("resolveTimeOffOwner(%q) = %q, want %q", tc.userName, got, tc.want)
			}
		})
	}
}
