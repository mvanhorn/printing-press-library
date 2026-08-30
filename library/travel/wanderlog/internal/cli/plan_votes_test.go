// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestCollectPlanVotesPlaceAndHotel(t *testing.T) {
	trip := testOutlineTrip()
	secs := sections(trip)
	sec0, _ := secs[0].(map[string]any)
	blocks, _ := sec0["blocks"].([]any)
	hotel, _ := blocks[0].(map[string]any)
	hotel["upvotedBy"] = []any{"alice", map[string]any{"userId": "bob"}}
	place, _ := blocks[2].(map[string]any)
	place["upvotedBy"] = []any{float64(9)}

	got := collectPlanVotes(trip)
	if len(got) != 4 {
		t.Fatalf("len(votes) = %d want 4 (place/hotel only): %#v", len(got), got)
	}
	if got[0].Name != "Hotel Sun" || got[0].Day != 1 || got[0].UpvotedByCount != 2 {
		t.Fatalf("hotel vote = %#v", got[0])
	}
	if strings.Join(got[0].UpvotedBy, ",") != "alice,bob" {
		t.Fatalf("upvoted_by = %#v", got[0].UpvotedBy)
	}
	if got[1].Name != "Shuri Castle" || got[1].UpvotedByCount != 1 || got[1].UpvotedBy[0] != "9" {
		t.Fatalf("castle vote = %#v", got[1])
	}
	for _, row := range got {
		if row.Name == "Pack snacks" {
			t.Fatalf("note block should be omitted: %#v", row)
		}
	}
}

func TestPrintPlanEditReportOmitsOpPathsAndSectionsUnlessVerbose(t *testing.T) {
	report := planEditReport{
		Command:   "plan note add",
		TargetKey: "abcdefghijklmnop",
		Applied:   false,
		DryRun:    true,
		BlockID:   11,
		Block:     map[string]any{"id": 11, "type": "note"},
		Section:   ptrSectionReport(planSectionReport{Index: 0, Day: 1, BlockCount: 2}),
		OpPaths:   []string{"itinerary.sections.0.blocks.2"},
		Sections:  []planSectionReport{{Index: 0, Day: 1, BlockCount: 2}, {Index: 1, Day: 2, BlockCount: 0}},
		Warnings:  []string{"preview only"},
		Stripped:  []string{"header"},
	}

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := printPlanEditReport(cmd, &rootFlags{asJSON: true}, report); err != nil {
		t.Fatalf("print: %v", err)
	}
	var terse map[string]any
	if err := json.Unmarshal(buf.Bytes(), &terse); err != nil {
		t.Fatalf("unmarshal terse: %v\n%s", err, buf.String())
	}
	if _, ok := terse["op_paths"]; ok {
		t.Fatalf("terse should omit op_paths: %s", buf.String())
	}
	if _, ok := terse["sections"]; ok {
		t.Fatalf("terse should omit sections: %s", buf.String())
	}
	if terse["command"] != "plan note add" || terse["target_key"] != "abcdefghijklmnop" || terse["block_id"].(float64) != 11 {
		t.Fatalf("kept fields missing: %#v", terse)
	}
	if _, ok := terse["section"]; !ok {
		t.Fatalf("section summary missing: %#v", terse)
	}
	if _, ok := terse["warnings"]; !ok {
		t.Fatalf("warnings missing: %#v", terse)
	}
	if _, ok := terse["stripped"]; !ok {
		t.Fatalf("stripped missing: %#v", terse)
	}

	buf.Reset()
	if err := printPlanEditReport(cmd, &rootFlags{asJSON: true, verbose: true}, report); err != nil {
		t.Fatalf("print verbose: %v", err)
	}
	var verbose map[string]any
	if err := json.Unmarshal(buf.Bytes(), &verbose); err != nil {
		t.Fatalf("unmarshal verbose: %v\n%s", err, buf.String())
	}
	paths, _ := verbose["op_paths"].([]any)
	secs, _ := verbose["sections"].([]any)
	if len(paths) != 1 || len(secs) != 2 {
		t.Fatalf("verbose restore failed: %s", buf.String())
	}
}
