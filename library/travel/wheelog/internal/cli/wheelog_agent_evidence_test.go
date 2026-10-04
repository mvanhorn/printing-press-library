// Copyright 2026 Jet Sng and contributors. Licensed under Apache-2.0.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/travel/wheelog/internal/wheelog"
	"github.com/spf13/cobra"
	"path/filepath"
	"testing"
	"time"
)

func runWheelogAgentPayload(t *testing.T, factory func(*rootFlags) *cobra.Command, args ...string) map[string]any {
	t.Helper()
	flags := &rootFlags{asJSON: true, agent: true, compact: true, noLearn: true, dataSource: "local", timeout: time.Minute}
	cmd := factory(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	payload, ok := doc["results"].(map[string]any)
	if !ok {
		t.Fatalf("invalid agent envelope: %s", out.Bytes())
	}
	return payload
}

func agentRows(t *testing.T, payload map[string]any) []any {
	t.Helper()
	rows, ok := payload["results"].([]any)
	if !ok {
		t.Fatalf("missing result collection: %v", payload)
	}
	return rows
}

func assertAgentQuestionEvidence(t *testing.T, spot map[string]any) {
	t.Helper()
	qs, ok := spot["questions"].([]any)
	if !ok || len(qs) != 1 {
		t.Fatalf("question evidence dropped: %v", spot)
	}
	q := qs[0].(map[string]any)
	if q["label"] != "Turning space" || q["positive_reports"] != float64(2) || q["negative_reports"] != float64(1) {
		t.Fatalf("counts/label lost: %v", q)
	}
	if value, present := q["report_observed_at"]; !present || value != nil {
		t.Fatalf("unknown report date lost: %v", q)
	}
	requirements, ok := spot["requirements"].([]any)
	if !ok || len(requirements) != 1 {
		t.Fatalf("requirements dropped: %v", spot)
	}
	r := requirements[0].(map[string]any)
	if r["question_id"] != float64(102) || r["positive_reports"] != float64(2) || r["negative_reports"] != float64(1) || r["state"] != "conflicting" {
		t.Fatalf("assessment lost: %v", r)
	}
}

func TestWheelogAgentCategoriesKeepExactValuesAndQuestionIDs(t *testing.T) {
	p := runWheelogAgentPayload(t, newWheelogCategoriesCmd)
	rows := agentRows(t, p)
	if len(rows) != 10 {
		t.Fatalf("category coverage %v", rows)
	}
	found := false
	for _, raw := range rows {
		row := raw.(map[string]any)
		if row["value"] == nil || row["question_ids"] == nil {
			t.Fatalf("category vocabulary lost: %v", row)
		}
		if row["value"] == "toilet" {
			found = true
			if len(row["question_ids"].([]any)) != 9 {
				t.Fatalf("question IDs lost: %v", row)
			}
		}
	}
	if !found {
		t.Fatal("restroom category missing")
	}
}

func TestWheelogAgentCollectionsKeepDecisionEvidence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "saved.db")
	db, err := openWheelogStore(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	one, zero, two := 1, 0, 2
	toilet := wheelog.Spot{ID: 166345, Name: "Public restroom", Category: "toilet", DetailStatus: "checked", Source: "live", ObservedAt: time.Now().Add(-14 * 24 * time.Hour).UTC().Format(time.RFC3339), Location: &wheelog.Coordinate{Latitude: 35.7742, Longitude: 140.388}, Questions: []wheelog.Question{{ID: 102, Label: "Turning space", Positive: &one, Negative: &zero, State: "reported_affirmative"}}, Gaps: []string{"individual_report_dates"}}
	if err := db.ObserveWheelog(ctx, toilet); err != nil {
		t.Fatal(err)
	}
	toilet.Name = "Recorded restroom"
	toilet.Questions[0].Positive = &two
	toilet.Questions[0].Negative = &one
	toilet.Questions[0].State = "conflicting"
	if err := db.ObserveWheelog(ctx, toilet); err != nil {
		t.Fatal(err)
	}
	elevator := wheelog.Spot{ID: 166344, Name: "Public elevator", Category: "elevator", DetailStatus: "checked", Source: "live", ObservedAt: toilet.ObservedAt, Location: &wheelog.Coordinate{Latitude: 35.7741, Longitude: 140.389}, Questions: []wheelog.Question{}, Gaps: []string{"individual_report_dates"}}
	if err := db.ObserveWheelog(ctx, elevator); err != nil {
		t.Fatal(err)
	}
	db.Close()
	inspect := runWheelogAgentPayload(t, newSpotsInspectCmd, "166345", "--db", path, "--require-question", "102")
	assertAgentQuestionEvidence(t, inspect)
	for _, factory := range []func(*rootFlags) *cobra.Command{newSpotsSearchCmd, newNovelSpotsCompareCmd} {
		args := []string{"--db", path, "--require-question", "102"}
		if factoryName := factory(&rootFlags{}).Name(); factoryName == "compare" {
			args = append(args, "166345", "166344")
		}
		p := runWheelogAgentPayload(t, factory, args...)
		found := false
		for _, raw := range agentRows(t, p) {
			row := raw.(map[string]any)
			if row["id"] == float64(166345) {
				found = true
				assertAgentQuestionEvidence(t, row)
			}
			if row["id"] == float64(166344) {
				r := row["requirements"].([]any)[0].(map[string]any)
				if r["state"] != "inapplicable" || r["reason"] == nil {
					t.Fatalf("inapplicable explanation lost: %v", r)
				}
			}
		}
		if !found {
			t.Fatal("requested source missing")
		}
	}
	audit := runWheelogAgentPayload(t, newNovelShortlistListCmd, "--db", path, "--audit", "--require-question", "102")
	for _, raw := range agentRows(t, audit) {
		row := raw.(map[string]any)
		if reasons, ok := row["recheck_reasons"].([]any); !ok || len(reasons) == 0 {
			t.Fatalf("audit reasons lost: %v", row)
		}
	}
	nearby := runWheelogAgentPayload(t, newNovelShortlistListCmd, "--db", path, "--origin", "35.7742,140.3879")
	for _, raw := range agentRows(t, nearby) {
		row := raw.(map[string]any)
		if _, ok := row["straight_line_distance_m"].(float64); !ok {
			t.Fatalf("distance lost: %v", row)
		}
	}
	changes := runWheelogAgentPayload(t, newNovelShortlistChangesCmd, "--db", path)
	found := false
	for _, raw := range agentRows(t, changes) {
		row := raw.(map[string]any)
		if row["id"] != float64(166345) {
			continue
		}
		found = true
		delta, ok := row["changes"].([]any)
		if !ok || len(delta) == 0 {
			t.Fatalf("changes lost: %v", row)
		}
		for _, rawChange := range delta {
			c := rawChange.(map[string]any)
			for _, key := range []string{"field", "previous", "current"} {
				if _, present := c[key]; !present {
					t.Fatalf("change values lost: %v", c)
				}
			}
		}
	}
	if !found {
		t.Fatal("changed source missing")
	}
}
