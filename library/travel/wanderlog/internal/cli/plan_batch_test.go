// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestPlanBatchSwapDaysOps(t *testing.T) {
	trip := testPlanTrip("Swap target")
	opts := planEditOptions{day: 1}
	result, err := buildSwapDaysOps(trip, opts, 2)
	if err != nil {
		t.Fatalf("buildSwapDaysOps: %v", err)
	}
	if len(result.Ops) != 2 {
		t.Fatalf("len(ops) = %d, want 2", len(result.Ops))
	}
	paths := opPaths(result.Ops)
	if paths[0] != "itinerary.sections.0.blocks" || paths[1] != "itinerary.sections.1.blocks" {
		t.Fatalf("paths = %#v", paths)
	}
	day1, _ := resolveSection(trip, 1, -1, 0)
	day2, _ := resolveSection(trip, 2, -1, 0)
	if intAny(result.Ops[0]["od"].([]any)[0].(map[string]any)["id"]) != 2001 {
		t.Fatalf("day 1 od first block = %#v", result.Ops[0]["od"])
	}
	if len(result.Ops[0]["oi"].([]any)) != len(day2.Blocks) {
		t.Fatalf("day 1 oi len = %d", len(result.Ops[0]["oi"].([]any)))
	}
	if len(result.Ops[1]["oi"].([]any)) != len(day1.Blocks) {
		t.Fatalf("day 2 oi len = %d", len(result.Ops[1]["oi"].([]any)))
	}
	if result.Report.Section == nil || result.Report.Destination == nil {
		t.Fatalf("missing section summaries: %#v", result.Report)
	}
	if result.Report.Section.Day != 1 || result.Report.Destination.Day != 2 {
		t.Fatalf("section days = %d / %d", result.Report.Section.Day, result.Report.Destination.Day)
	}
	if result.Report.Section.BlockCount != 1 || result.Report.Destination.BlockCount != 2 {
		t.Fatalf("swapped counts = %d / %d", result.Report.Section.BlockCount, result.Report.Destination.BlockCount)
	}
}

func TestPlanBatchFillDayBuildsNLiOps(t *testing.T) {
	trip := testPlanTrip("Fill target")
	opts := planEditOptions{day: 3, sectionIndex: -1}
	stops := []fillDayStop{
		{PlaceID: "p1", Start: "09:00", End: "10:30", Note: "cafe"},
		{PlaceID: "p2", NoteMD: "- item"},
	}
	places := map[string]map[string]any{
		"id:p1": {"place_id": "p1", "name": "Cafe"},
		"id:p2": {"place_id": "p2", "name": "Park"},
	}
	result, err := buildFillDayOps(trip, opts, stops, places, "block")
	if err != nil {
		t.Fatalf("buildFillDayOps: %v", err)
	}
	if len(result.Ops) != 2 {
		t.Fatalf("len(ops) = %d, want 2 li ops", len(result.Ops))
	}
	for i, op := range result.Ops {
		if _, ok := op["li"]; !ok {
			t.Fatalf("op %d missing li: %#v", i, op)
		}
		path, _ := op["p"].([]any)
		if len(path) != 5 || intAny(path[2]) != 2 || intAny(path[4]) != i {
			t.Fatalf("op %d path = %#v", i, path)
		}
	}
	first := result.Ops[0]["li"].(map[string]any)
	if first["startTime"] != "09:00" || first["endTime"] != "10:30" {
		t.Fatalf("schedule on first block = %#v", first)
	}
	if stringField(mapField(first, "place"), "place_id") != "p1" {
		t.Fatalf("first place = %#v", first["place"])
	}
	if got := plainRichText(mapField(first, "text")); got != "cafe" {
		t.Fatalf("first note = %q", got)
	}
	second := result.Ops[1]["li"].(map[string]any)
	ops, _ := mapField(second, "text")["ops"].([]any)
	if len(ops) < 2 {
		t.Fatalf("markdown ops = %#v", ops)
	}
	newline := ops[len(ops)-1].(map[string]any)
	if stringAny(mapField(newline, "attributes")["list"]) != "bullet" {
		t.Fatalf("expected list:bullet, ops = %#v", ops)
	}
}

func TestPlanBatchPlaceReplaceOnlyTouchesPlace(t *testing.T) {
	trip := testPlanTrip("Replace target")
	sec := sections(trip)[0].(map[string]any)
	block := sec["blocks"].([]any)[0].(map[string]any)
	block["place"] = map[string]any{"place_id": "old-place", "name": "Old Cafe"}
	block["startTime"] = "09:00"
	block["hotel"] = map[string]any{"checkIn": "2026-08-30"}
	opts := planEditOptions{day: 1, sectionIndex: -1, blockID: 2001, blockIndex: -1}
	place := map[string]any{"place_id": "new-place", "name": "New Cafe"}
	result, err := buildPlaceReplaceOps(trip, opts, place, "block")
	if err != nil {
		t.Fatalf("buildPlaceReplaceOps: %v", err)
	}
	if len(result.Ops) != 1 {
		t.Fatalf("len(ops) = %d, want 1", len(result.Ops))
	}
	op := result.Ops[0]
	for key := range op {
		if key != "p" && key != "od" && key != "oi" {
			t.Fatalf("unexpected op key %q: %#v", key, op)
		}
	}
	paths := opPaths(result.Ops)
	if paths[0] != "itinerary.sections.0.blocks.0.place" {
		t.Fatalf("path = %q", paths[0])
	}
	newPlace, _ := op["oi"].(map[string]any)
	if stringField(newPlace, "place_id") != "new-place" {
		t.Fatalf("oi place = %#v", op["oi"])
	}
	oldPlace, _ := op["od"].(map[string]any)
	if stringField(oldPlace, "place_id") != "old-place" {
		t.Fatalf("od place = %#v", op["od"])
	}
	if intAny(result.Report.Block["id"]) != 2001 || result.Report.Block["type"] != "place" {
		t.Fatalf("id/type changed: %#v", result.Report.Block)
	}
	if result.Report.Block["startTime"] != "09:00" {
		t.Fatalf("schedule was not kept: %#v", result.Report.Block)
	}
	if mapField(result.Report.Block, "hotel") == nil {
		t.Fatalf("hotel was not kept: %#v", result.Report.Block)
	}
}

func TestPlanBatchParseOpsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ops.json")
	if err := os.WriteFile(path, []byte(`[{"p":["title"],"od":"Old","oi":"New"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	ops, err := readJSON0OpsFile(path)
	if err != nil {
		t.Fatalf("readJSON0OpsFile: %v", err)
	}
	if len(ops) != 1 || opPaths(ops)[0] != "title" {
		t.Fatalf("ops = %#v", ops)
	}
	if ops[0]["od"] != "Old" || ops[0]["oi"] != "New" {
		t.Fatalf("op = %#v", ops[0])
	}

	loaded, err := loadJSON0Ops("", path)
	if err != nil {
		t.Fatalf("loadJSON0Ops file: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("loaded = %#v", loaded)
	}
	if _, err := loadJSON0Ops(`[{"p":["days"],"oi":3}]`, path); err == nil {
		t.Fatal("expected both --op and --ops-file to fail")
	}
	if _, err := loadJSON0Ops("", ""); err == nil {
		t.Fatal("expected missing source to fail")
	}
	emptyPath := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(emptyPath, []byte(`[]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readJSON0OpsFile(emptyPath); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("empty array error = %v", err)
	}
}

func TestPlanBatchCommandWiring(t *testing.T) {
	flags := &rootFlags{}
	plan := newNovelPlanCmd(flags)
	if plan.Commands() == nil {
		t.Fatal("plan has no commands")
	}
	fillDay := findNamedCmd(t, plan, "fill-day")
	for _, name := range []string{"target-key", "day", "stops-json", "apply", "closed-place-policy"} {
		if fillDay.Flags().Lookup(name) == nil {
			t.Errorf("plan fill-day missing --%s", name)
		}
	}
	section := findNamedCmd(t, plan, "section")
	swap := findNamedCmd(t, section, "swap-days")
	if swap.Flags().Lookup("day") == nil || swap.Flags().Lookup("with-day") == nil {
		t.Fatal("plan section swap-days missing --day/--with-day")
	}
	place := findNamedCmd(t, plan, "place")
	replace := findNamedCmd(t, place, "replace")
	for _, name := range []string{"day", "block-id", "place-id", "query"} {
		if replace.Flags().Lookup(name) == nil {
			t.Errorf("plan place replace missing --%s", name)
		}
	}
	block := findNamedCmd(t, plan, "block")
	apply := findNamedCmd(t, block, "apply")
	if apply.Flags().Lookup("ops-file") == nil {
		t.Fatal("plan block apply missing --ops-file")
	}
	raw := findNamedCmd(t, plan, "raw")
	op := findNamedCmd(t, raw, "op")
	if op.Flags().Lookup("ops-file") == nil {
		t.Fatal("plan raw op missing --ops-file")
	}
	if op.Flags().Lookup("op") == nil {
		t.Fatal("plan raw op missing --op")
	}
}

func findNamedCmd(t *testing.T, parent *cobra.Command, name string) *cobra.Command {
	t.Helper()
	for _, cmd := range parent.Commands() {
		if cmd.Name() == name {
			return cmd
		}
	}
	t.Fatalf("command %q not found under %s", name, parent.Name())
	return nil
}
