// Copyright 2026 Isaac Marks and contributors. Licensed under Apache-2.0. See LICENSE.
// Behavioral acceptance tests for the hand-written Clarify novel commands.
// Each test builds a fixture mirror and asserts output CONTENT, not just exit
// codes: stale must find the idle deal and not the fresh one, followup must
// flag the silent meeting, dupes must group shared emails, velocity must
// record observations, dossier/prep must assemble the right related records.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/clarify/internal/store"
)

func writeFixtureMirror(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "clarify.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("opening fixture store: %v", err)
	}
	defer db.Close()

	now := time.Now()
	iso := func(t time.Time) string { return t.UTC().Format(time.RFC3339) }
	rows := []map[string]any{
		{
			"type": "company", "id": "co-acme",
			"attributes": map[string]any{
				"name":        "Acme, Inc.",
				"domains":     map[string]any{"items": []any{"acme.com"}},
				"_updated_at": iso(now.AddDate(0, 0, -30)),
			},
		},
		{
			"type": "company", "id": "co-acme2",
			"attributes": map[string]any{
				"name":        "Acme Inc",
				"domains":     map[string]any{"items": []any{"acme.com"}},
				"_updated_at": iso(now.AddDate(0, 0, -3)),
			},
		},
		{
			// Real Clarify shape: company link is a plain attribute, not a
			// JSON:API relationship.
			"type": "person", "id": "p-jane",
			"attributes": map[string]any{
				"name":            map[string]any{"first_name": "Jane", "last_name": "Doe"},
				"email_addresses": map[string]any{"items": []any{"jane@acme.com"}},
				"company_id":      "co-acme",
			},
		},
		{
			"type": "person", "id": "p-jane2",
			"attributes": map[string]any{
				"name":            map[string]any{"first_name": "Jane", "last_name": "D."},
				"email_addresses": map[string]any{"items": []any{"jane@acme.com"}},
			},
		},
		{
			"type": "deal", "id": "d-stale",
			"attributes": map[string]any{
				"name":        "Acme expansion",
				"stage":       "Negotiation",
				"amount":      50000.0,
				"_updated_at": iso(now.AddDate(0, 0, -30)),
				"company_id":  "co-acme",
			},
		},
		{
			"type": "deal", "id": "d-fresh",
			"attributes": map[string]any{
				"name":        "Acme renewal",
				"stage":       "Discovery",
				"_updated_at": iso(now.AddDate(0, 0, -1)),
				"company_id":  "co-acme",
			},
		},
		{
			"type": "deal", "id": "d-won",
			"attributes": map[string]any{
				"name":        "Acme closed",
				"stage":       "Closed Won",
				"_updated_at": iso(now.AddDate(0, 0, -60)),
			},
		},
		{
			// Meeting today, so brief picks it up; ended silently 2 days ago
			// variant below for followup.
			"type": "meeting", "id": "m-today",
			"attributes": map[string]any{
				"title": "Acme sync call",
				"start": iso(time.Date(now.Year(), now.Month(), now.Day(), 23, 0, 0, 0, time.UTC)),
				"participants": map[string]any{"items": []any{
					map[string]any{"name": "Jane Doe", "email": "jane@acme.com", "status": "yes", "organizer": false},
				}},
			},
		},
		{
			"type": "meeting", "id": "m-silent",
			"attributes": map[string]any{
				"title": "Acme discovery call",
				"start": iso(now.AddDate(0, 0, -2)),
				"end":   iso(now.AddDate(0, 0, -2).Add(time.Hour)),
				"participants": map[string]any{"items": []any{
					map[string]any{"name": "Jane Doe", "email": "jane@acme.com", "status": "yes", "organizer": false},
				}},
			},
		},
		{
			"type": "task", "id": "t-old",
			"attributes": map[string]any{
				"name":        "Send proposal",
				"_created_at": iso(now.AddDate(0, 0, -10)),
			},
		},
	}
	for _, row := range rows {
		data, err := json.Marshal(row)
		if err != nil {
			t.Fatalf("marshaling fixture row: %v", err)
		}
		if err := db.Upsert("resources", row["id"].(string), data); err != nil {
			t.Fatalf("upserting fixture row: %v", err)
		}
	}
	return dbPath
}

func runNovel(t *testing.T, args ...string) string {
	t.Helper()
	cmd := RootCmd()
	cmd.SetArgs(args)
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("%v error = %v\nstdout:\n%s\nstderr:\n%s", args, err, out.String(), errOut.String())
	}
	return out.String()
}

func TestStaleFindsIdleDealOnly(t *testing.T) {
	dbPath := writeFixtureMirror(t)
	out := runNovel(t, "stale", "--days", "14", "--json", "--db", dbPath)
	var view struct {
		StaleDeals []struct {
			ID    string `json:"id"`
			Stage string `json:"stage"`
		} `json:"stale_deals"`
		ScannedDeals int `json:"scanned_deals"`
	}
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("stale --json output is not valid JSON: %v\n%s", err, out)
	}
	if view.ScannedDeals != 3 {
		t.Fatalf("scanned_deals = %d, want 3", view.ScannedDeals)
	}
	if len(view.StaleDeals) != 1 || view.StaleDeals[0].ID != "d-stale" {
		t.Fatalf("stale_deals = %+v, want exactly d-stale (fresh and closed-won excluded)", view.StaleDeals)
	}
}

func TestBriefJoinsMeetingToCompanyAndDeals(t *testing.T) {
	dbPath := writeFixtureMirror(t)
	out := runNovel(t, "brief", "--json", "--db", dbPath)
	var view struct {
		TodaysMeetings []struct {
			ID        string   `json:"id"`
			Company   string   `json:"company"`
			OpenDeals []string `json:"open_deals"`
			Attendees []string `json:"attendees"`
		} `json:"todays_meetings"`
		OpenDealCount int `json:"open_deal_count"`
	}
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("brief --json output is not valid JSON: %v\n%s", err, out)
	}
	if view.OpenDealCount != 2 {
		t.Fatalf("open_deal_count = %d, want 2 (closed-won excluded)", view.OpenDealCount)
	}
	if len(view.TodaysMeetings) != 1 || view.TodaysMeetings[0].ID != "m-today" {
		t.Fatalf("todays_meetings = %+v, want exactly m-today", view.TodaysMeetings)
	}
	m := view.TodaysMeetings[0]
	if m.Company == "" {
		t.Fatalf("meeting company not joined: %+v", m)
	}
	if len(m.OpenDeals) != 2 {
		t.Fatalf("meeting open_deals = %v, want the 2 open Acme deals", m.OpenDeals)
	}
	if len(m.Attendees) != 1 || m.Attendees[0] != "Jane Doe" {
		t.Fatalf("attendees = %v, want [Jane Doe]", m.Attendees)
	}
}

func TestFollowupFlagsSilentMeeting(t *testing.T) {
	dbPath := writeFixtureMirror(t)
	out := runNovel(t, "followup", "--since", "7d", "--json", "--db", dbPath)
	var view struct {
		Gaps []struct {
			ID string `json:"id"`
		} `json:"gaps"`
		ScannedMeetings int `json:"scanned_meetings"`
	}
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("followup --json output is not valid JSON: %v\n%s", err, out)
	}
	if view.ScannedMeetings == 0 {
		t.Fatalf("scanned_meetings = 0, want at least the silent meeting scanned")
	}
	found := false
	for _, g := range view.Gaps {
		if g.ID == "m-silent" {
			found = true
		}
	}
	// co-acme's _updated_at is 30 days ago (before the meeting) and the only
	// task was created 10 days ago (also before), so m-silent has no
	// follow-up signal and must be flagged.
	if !found {
		t.Fatalf("gaps = %+v, want m-silent flagged", view.Gaps)
	}
}

func TestFollowupNoDealAbsence(t *testing.T) {
	dbPath := writeFixtureMirror(t)
	out := runNovel(t, "followup", "--since", "7d", "--no-deal", "--json", "--db", dbPath)
	var view struct {
		NoDealCompanies []struct {
			ID string `json:"id"`
		} `json:"no_deal_companies"`
	}
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("followup --no-deal output is not valid JSON: %v\n%s", err, out)
	}
	// Acme has open deals, so the no-deal list must be empty — absence
	// correctness: no fabricated entries.
	if len(view.NoDealCompanies) != 0 {
		t.Fatalf("no_deal_companies = %+v, want empty (Acme has open deals)", view.NoDealCompanies)
	}
}

func TestDupesGroupsSharedEmailAndDomain(t *testing.T) {
	dbPath := writeFixtureMirror(t)
	out := runNovel(t, "dupes", "--type", "person", "--json", "--db", dbPath)
	var view struct {
		Groups []struct {
			MatchedOn    string   `json:"matched_on"`
			RecordIDs    []string `json:"record_ids"`
			MergeCommand string   `json:"merge_command"`
		} `json:"groups"`
	}
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("dupes --json output is not valid JSON: %v\n%s", err, out)
	}
	if len(view.Groups) == 0 {
		t.Fatalf("no dupe groups found; p-jane and p-jane2 share jane@acme.com\n%s", out)
	}
	g := view.Groups[0]
	if len(g.RecordIDs) != 2 || g.MergeCommand == "" {
		t.Fatalf("group = %+v, want both jane records plus a merge command", g)
	}

	out = runNovel(t, "dupes", "--type", "company", "--json", "--db", dbPath)
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("dupes company output is not valid JSON: %v\n%s", err, out)
	}
	if len(view.Groups) == 0 {
		t.Fatalf("no company dupe groups; co-acme and co-acme2 share acme.com\n%s", out)
	}
}

func TestDupesNegativeNoFalsePositives(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "clarify.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	for i := 0; i < 3; i++ {
		row := map[string]any{
			"type": "person", "id": fmt.Sprintf("p-%d", i),
			"attributes": map[string]any{
				"name":            fmt.Sprintf("Person %d", i),
				"email_addresses": map[string]any{"items": []any{fmt.Sprintf("p%d@example%d.com", i, i)}},
			},
		}
		data, _ := json.Marshal(row)
		if err := db.Upsert("resources", row["id"].(string), data); err != nil {
			t.Fatalf("upsert: %v", err)
		}
	}
	db.Close()
	out := runNovel(t, "dupes", "--type", "person", "--json", "--db", dbPath)
	var view struct {
		Groups []any `json:"groups"`
	}
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(view.Groups) != 0 {
		t.Fatalf("groups = %+v, want none for distinct people", view.Groups)
	}
}

func TestVelocityRecordsObservationsAndDistribution(t *testing.T) {
	dbPath := writeFixtureMirror(t)
	out := runNovel(t, "velocity", "--json", "--db", dbPath)
	var view struct {
		Stages []struct {
			Stage     string `json:"stage"`
			OpenDeals int    `json:"open_deals"`
		} `json:"stages"`
		ObservationsNew int `json:"observations_recorded_this_run"`
	}
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("velocity --json output is not valid JSON: %v\n%s", err, out)
	}
	if view.ObservationsNew != 3 {
		t.Fatalf("observations_recorded_this_run = %d, want 3 (one per staged deal)", view.ObservationsNew)
	}
	stages := map[string]int{}
	for _, s := range view.Stages {
		stages[s.Stage] = s.OpenDeals
	}
	if stages["Negotiation"] != 1 || stages["Discovery"] != 1 {
		t.Fatalf("stage distribution = %v, want Negotiation=1 Discovery=1", stages)
	}

	// Second run: no stage changed, so zero new observations (idempotent).
	out = runNovel(t, "velocity", "--json", "--db", dbPath)
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("second velocity run invalid JSON: %v", err)
	}
	if view.ObservationsNew != 0 {
		t.Fatalf("second run observations = %d, want 0", view.ObservationsNew)
	}
}

func TestDossierAssemblesRelatedRecords(t *testing.T) {
	dbPath := writeFixtureMirror(t)
	out := runNovel(t, "dossier", "co-acme", "--json", "--data-source", "local", "--db", dbPath)
	var view struct {
		Type    string `json:"type"`
		Related []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"related"`
		Meetings []struct {
			ID string `json:"id"`
		} `json:"meetings"`
	}
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("dossier --json output is not valid JSON: %v\n%s", err, out)
	}
	if view.Type != "company" {
		t.Fatalf("type = %q, want company", view.Type)
	}
	relatedTypes := map[string]bool{}
	for _, r := range view.Related {
		relatedTypes[r.Type] = true
	}
	if !relatedTypes["person"] || !relatedTypes["deal"] {
		t.Fatalf("related = %+v, want at least one person and one deal referencing co-acme", view.Related)
	}
	if len(view.Meetings) == 0 {
		t.Fatalf("meetings empty, want the Acme meetings attached")
	}
}

func TestPrepNextBuildsPack(t *testing.T) {
	dbPath := writeFixtureMirror(t)
	out := runNovel(t, "prep", "--next", "--json", "--data-source", "local", "--db", dbPath)
	var view struct {
		Meeting   map[string]any `json:"meeting"`
		Company   string         `json:"company"`
		OpenDeals []struct {
			ID string `json:"id"`
		} `json:"open_deals"`
		Attendees []struct {
			Name string `json:"name"`
		} `json:"attendees"`
	}
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("prep --json output is not valid JSON: %v\n%s", err, out)
	}
	if view.Meeting["id"] != "m-today" {
		t.Fatalf("meeting = %v, want m-today (the only upcoming meeting)", view.Meeting)
	}
	if view.Company == "" {
		t.Fatalf("company not resolved for prep pack")
	}
	if len(view.OpenDeals) != 2 {
		t.Fatalf("open_deals = %+v, want the 2 open Acme deals", view.OpenDeals)
	}
	if len(view.Attendees) != 1 || view.Attendees[0].Name != "Jane Doe" {
		t.Fatalf("attendees = %+v, want Jane Doe", view.Attendees)
	}
}

func TestAttrHelpers(t *testing.T) {
	cases := []struct {
		name  string
		attrs map[string]any
		want  string
	}{
		{"plain string", map[string]any{"name": "Acme"}, "Acme"},
		{"name object", map[string]any{"name": map[string]any{"first_name": "Jane", "last_name": "Doe"}}, "Jane Doe"},
		{"fallback key", map[string]any{"title": "Kickoff"}, "Kickoff"},
		{"missing", map[string]any{}, ""},
	}
	for _, tc := range cases {
		if got := attrString(tc.attrs, clarifyNameKeys...); got != tc.want {
			t.Errorf("%s: attrString = %q, want %q", tc.name, got, tc.want)
		}
	}

	items := attrItems(map[string]any{"email_addresses": map[string]any{"items": []any{"a@b.com", "c@d.com"}}}, clarifyEmailKeys...)
	if len(items) != 2 {
		t.Errorf("attrItems collection = %v, want 2 entries", items)
	}
	if got := normalizeClarifyName("Acme, Inc."); got != "acme" {
		t.Errorf("normalizeClarifyName = %q, want acme", got)
	}
	if !clarifyStageClosed("Closed Won") || clarifyStageClosed("Negotiation") {
		t.Errorf("clarifyStageClosed misclassifies stages")
	}
	if _, ok := attrTime(map[string]any{"_updated_at": "2026-08-01T10:00:00Z"}, clarifyUpdatedKeys...); !ok {
		t.Errorf("attrTime failed to parse RFC3339")
	}
}
