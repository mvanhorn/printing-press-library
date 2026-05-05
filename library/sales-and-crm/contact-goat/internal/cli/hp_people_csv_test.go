// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// hp_people_csv_test.go: tests for the U7 CSV emitter on hp people.
// The CSV path replaces the silently-broken global --csv that produced
// "[{...} {...}]" Go-slice junk for nested bridges; this file pins the
// stable column contract and the bridge denormalization rules.

package cli

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/client"
)

// runHPPeopleCSVOnFixture is the test driver: build a cobra command,
// pipe stdout into a buffer, run printHPPeopleCSV, return parsed rows.
func runHPPeopleCSVOnFixture(t *testing.T, res *client.PeopleSearchResult, currentUUID string) [][]string {
	t.Helper()
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := printHPPeopleCSV(cmd, res, currentUUID); err != nil {
		t.Fatalf("printHPPeopleCSV: %v", err)
	}
	r := csv.NewReader(strings.NewReader(buf.String()))
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("parse CSV output: %v\nraw:\n%s", err, buf.String())
	}
	return rows
}

func TestPrintHPPeopleCSV_HeaderColumns(t *testing.T) {
	rows := runHPPeopleCSVOnFixture(t, &client.PeopleSearchResult{}, "")
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1 (header only)", len(rows))
	}
	want := []string{
		"name", "current_title", "current_company", "linkedin_url", "score",
		"relationship_tier", "bridge_count", "bridge_names", "bridge_kinds",
		"top_bridge_affinity", "rationale",
	}
	if len(rows[0]) != len(want) {
		t.Fatalf("header has %d columns, want %d. got: %v", len(rows[0]), len(want), rows[0])
	}
	for i, name := range want {
		if rows[0][i] != name {
			t.Errorf("col[%d] = %q, want %q", i, rows[0][i], name)
		}
	}
}

func TestPrintHPPeopleCSV_SelfGraphIs1stDegree(t *testing.T) {
	res := &client.PeopleSearchResult{
		People: []client.Person{{
			Name:           "Alice Self",
			CurrentTitle:   "VP",
			CurrentCompany: "Acme",
			LinkedInURL:    "https://linkedin.com/in/alice",
			Score:          50,
			Bridges: []client.Bridge{
				{Name: "Matt Self", Kind: client.BridgeKindSelfGraph, AffinityScore: 50},
			},
		}},
	}
	rows := runHPPeopleCSVOnFixture(t, res, "")
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (header + 1)", len(rows))
	}
	row := rows[1]
	if got, want := row[5], string(client.TierFirstDegree); got != want {
		t.Errorf("relationship_tier = %q, want %q (self_graph bridge present)", got, want)
	}
	if got, want := row[6], "1"; got != want {
		t.Errorf("bridge_count = %q, want %q", got, want)
	}
	if got, want := row[7], "Matt Self"; got != want {
		t.Errorf("bridge_names = %q, want %q", got, want)
	}
	if got, want := row[8], "self_graph"; got != want {
		t.Errorf("bridge_kinds = %q, want %q", got, want)
	}
}

func TestPrintHPPeopleCSV_MultipleBridgesSemicolonJoined(t *testing.T) {
	res := &client.PeopleSearchResult{
		People: []client.Person{{
			Name:  "Bob Multi",
			Score: 30,
			Bridges: []client.Bridge{
				{Name: "Carol", Kind: client.BridgeKindFriend, AffinityScore: 25},
				{Name: "Dave", Kind: client.BridgeKindFriend, AffinityScore: 40},
				{Name: "Eve", Kind: client.BridgeKindFriend, AffinityScore: 10},
			},
		}},
	}
	rows := runHPPeopleCSVOnFixture(t, res, "")
	row := rows[1]
	if got, want := row[6], "3"; got != want {
		t.Errorf("bridge_count = %q, want %q", got, want)
	}
	if got, want := row[7], "Carol;Dave;Eve"; got != want {
		t.Errorf("bridge_names = %q, want %q (semicolon-joined, original order)", got, want)
	}
	if got, want := row[8], "friend;friend;friend"; got != want {
		t.Errorf("bridge_kinds = %q, want %q", got, want)
	}
	if got, want := row[9], "40"; got != want {
		t.Errorf("top_bridge_affinity = %q, want %q (max of 25, 40, 10)", got, want)
	}
}

func TestPrintHPPeopleCSV_NoBridgesEmptyCells(t *testing.T) {
	res := &client.PeopleSearchResult{
		People: []client.Person{{
			Name:  "Carol Public",
			Score: 2,
		}},
	}
	rows := runHPPeopleCSVOnFixture(t, res, "")
	row := rows[1]
	if got, want := row[6], "0"; got != want {
		t.Errorf("bridge_count = %q, want %q (no bridges)", got, want)
	}
	if row[7] != "" {
		t.Errorf("bridge_names = %q, want empty string for zero bridges", row[7])
	}
	if row[9] != "" {
		t.Errorf("top_bridge_affinity = %q, want empty string for zero bridges", row[9])
	}
}

func TestPrintHPPeopleCSV_NameWithCommaIsQuoted(t *testing.T) {
	res := &client.PeopleSearchResult{
		People: []client.Person{{
			Name: "Smith, John",
		}},
	}
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := printHPPeopleCSV(cmd, res, ""); err != nil {
		t.Fatalf("printHPPeopleCSV: %v", err)
	}
	// stdlib encoding/csv quotes fields containing commas. Just confirm
	// that the first data line begins with a quoted name.
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d output lines, want 2", len(lines))
	}
	if !strings.HasPrefix(lines[1], `"Smith, John"`) {
		t.Errorf("expected first column to be quoted name, got: %q", lines[1])
	}
}

func TestPrintHPPeopleCSV_EmptyResultProducesHeaderOnly(t *testing.T) {
	rows := runHPPeopleCSVOnFixture(t, &client.PeopleSearchResult{}, "")
	if len(rows) != 1 {
		t.Errorf("got %d rows, want 1 (header only on empty result)", len(rows))
	}
}

func TestSummarizeBridges_OrderPreservation(t *testing.T) {
	in := []client.Bridge{
		{Name: "B", Kind: "friend", AffinityScore: 5},
		{Name: "A", Kind: "self_graph", AffinityScore: 50},
		{Name: "C", Kind: "friend", AffinityScore: 12},
	}
	count, names, kinds, top := summarizeBridges(in)
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
	if names != "B;A;C" {
		t.Errorf("names = %q, want B;A;C (preserve input order)", names)
	}
	if kinds != "friend;self_graph;friend" {
		t.Errorf("kinds = %q, want friend;self_graph;friend (preserve input order)", kinds)
	}
	if top != 50 {
		t.Errorf("top = %g, want 50", top)
	}
}
