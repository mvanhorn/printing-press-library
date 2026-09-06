// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestCreateBatchRejectsMalformedIntent(t *testing.T) {
	for _, raw := range []string{`null`, `[]`, `[null]`, `[{"type":"note","day":1,"text":"x","TEXT":"y"}]`, `[{"type":"note","day":1,"day":2,"text":"x"}]`, `[{"type":"note","day":1,"text":null}]`, `[{"type":"note","day":1,"section_id":2,"text":"x"}]`, `[{"type":"place","day":1}]`, `[{"type":"note","day":1,"text":"x","place_id":"p"}]`, `[{"type":"checklist","day":1,"items":[null]}]`, `[{"type":"checklist","day":1,"items":[]}]`, `[{"type":"note","day":1,"text":"x","duration_minutes":1441}]`, `[{"type":"note","day":1,"text":"x","start":"10:00","end":"11:00","duration_minutes":30}]`, `[{"type":"note","day":1,"text":"x","ref":"same"},{"type":"note","day":2,"text":"y","ref":"same"}]`, `[{"type":"note","day":1,"text":"x"}] {}`} {
		if _, err := parsePlanCreateEntries([]byte(raw)); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
}

func TestCreateBatchBuildsMixedAtomicAppendAndReceipt(t *testing.T) {
	trip := blockReadFixture()
	before, _ := json.Marshal(trip)
	entries, err := parsePlanCreateEntries([]byte(`[{"ref":"lunch","type":"place","day":1,"place_id":"p","name":"Chosen display name","markdown":"**Book ahead**","start":"12:00","duration_minutes":60},{"type":"note","section_id":11,"markdown":"**Reminder**\n- Bring tickets"},{"type":"checklist","day":1,"title":"Packing","items":["Water","Hat"]},{"type":"note","day":1,"text":"Meet at entrance"}]`))
	if err != nil {
		t.Fatal(err)
	}
	places := map[string]map[string]any{"id:p": {"place_id": "p", "name": "Original place", "business_status": "OPERATIONAL"}}
	result, err := buildPlanCreateBatch(trip, planEditOptions{}, entries, places, "block", false)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(trip)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("mutated source snapshot")
	}
	if places["id:p"]["name"] != "Original place" {
		t.Fatal("mutated shared resolved place")
	}
	paths := opPaths(result.Ops)
	want := []string{"itinerary.sections.0.blocks.1", "itinerary.sections.1.blocks.1", "itinerary.sections.0.blocks.2", "itinerary.sections.0.blocks.3"}
	if !reflect.DeepEqual(paths, want) {
		t.Fatal(paths)
	}
	ids := map[int]bool{101: true, 102: true}
	for i, op := range result.Ops {
		block := op["li"].(map[string]any)
		id := intAny(block["id"])
		if id <= 0 || ids[id] {
			t.Fatal("duplicate stable ID")
		}
		ids[id] = true
		if intAny(result.Report.Changes[i]["block_id"]) != id {
			t.Fatal("receipt ID mismatch")
		}
		if op["ld"] != nil {
			t.Fatal("unexpected replacement")
		}
	}
	place := result.Ops[0]["li"].(map[string]any)
	if place["endTime"] != "13:00" || !strings.Contains(readableBlockMarkdown(place), "**Book ahead**") {
		t.Fatal(place)
	}
	checklist := result.Ops[2]["li"].(map[string]any)
	for _, raw := range checklist["items"].([]any) {
		item := raw.(map[string]any)
		id := intAny(item["id"])
		if ids[id] || id <= 0 {
			t.Fatal("duplicate item ID")
		}
		ids[id] = true
		if mapField(item, "text") == nil {
			t.Fatal("checklist text not rich text")
		}
	}
	receipt, err := buildPlanCreateBatch(trip, planEditOptions{apply: true}, entries, places, "block", true)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range receipt.Report.Changes {
		if _, ok := change["after"]; ok {
			t.Fatal("receipt duplicates content")
		}
		if change["block_id"] == nil {
			t.Fatal("receipt omits ID")
		}
	}
	if receipt.Report.Changes[0]["ref"] != "lunch" {
		t.Fatal("lost caller correlation")
	}
}

func TestCreateBatchAnyInvalidEntryReturnsNoOps(t *testing.T) {
	for _, raw := range []string{`[{"type":"note","day":1,"text":"first"},{"type":"place","day":1,"place_id":"unresolved"}]`, `[{"type":"note","day":1,"text":"first"},{"type":"note","day":99,"text":"bad day"}]`, `[{"type":"note","day":1,"text":"first"},{"type":"place","day":1,"place_id":"closed"}]`} {
		entries, err := parsePlanCreateEntries([]byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		places := map[string]map[string]any{"id:closed": {"place_id": "closed", "business_status": "CLOSED_TEMPORARILY"}}
		got, err := buildPlanCreateBatch(blockReadFixture(), planEditOptions{}, entries, places, "block", false)
		if err == nil || len(got.Ops) > 0 || len(got.Report.Changes) > 0 {
			t.Fatal("partial batch escaped validation", got, err)
		}
	}
	trip := blockReadFixture()
	sections(trip)[1].(map[string]any)["id"] = 10
	entries, _ := parsePlanCreateEntries([]byte(`[{"type":"note","section_id":10,"text":"x"}]`))
	if _, err := buildPlanCreateBatch(trip, planEditOptions{}, entries, nil, "block", false); err == nil {
		t.Fatal("ambiguous section accepted")
	}
}

func TestCreateBatchDryRunValidatesBeforeNetwork(t *testing.T) {
	path := t.TempDir() + "/blocks.json"
	if err := os.WriteFile(path, []byte(`[{"type":"note","day":1,"text":"x","unknown":true}]`), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := newNovelPlanBlockAddBatchCmd(&rootFlags{dryRun: true})
	cmd.SetArgs([]string{"--blocks-file", path})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatal(err)
	}
}

func TestCreateBatchClosedPlaceWarnAndIgnore(t *testing.T) {
	entries, _ := parsePlanCreateEntries([]byte(`[{"type":"place","day":1,"place_id":"p"}]`))
	places := map[string]map[string]any{"id:p": {"place_id": "p", "business_status": "CLOSED_TEMPORARILY"}}
	for _, policy := range []string{"warn", "ignore"} {
		r, err := buildPlanCreateBatch(blockReadFixture(), planEditOptions{}, entries, places, policy, true)
		if err != nil {
			t.Fatal(err)
		}
		if (len(r.Report.Warnings) > 0) != (policy == "warn") {
			t.Fatal(r.Report.Warnings)
		}
	}
}
