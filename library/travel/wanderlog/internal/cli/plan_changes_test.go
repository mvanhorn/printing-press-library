// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"bytes"
	"encoding/json"
	"github.com/spf13/cobra"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestPlanChangesRejectAmbiguousIntent(t *testing.T) {
	for _, input := range []string{
		`[]`, `null`, `[{"block_id":101,"name":"a","NAME":"b"}]`, `[{"block_id":101}]`, `[{"block_id":101,"name":""}]`,
		`[{"block_id":101,"markdown":"ok","action":"delete"}]`,
		`[{"block_id":101,"name":"a"},{"block_id":101,"name":"b"}]`,
		`[{"block_id":101,"start":"25:00"}]`, `[{"block_id":101,"duration_minutes":-1}]`, `[{"block_id":101,"duration_minutes":1441}]`,
		`[{"block_id":101,"name":"a","name":"b"}]`, `[{"block_id":101,"markdown":"ok","name":null}]`,
		`[{"block_id":101,"name":"a"}] []`,
	} {
		if _, err := parsePlanChanges([]byte(input)); err == nil {
			t.Errorf("accepted %s", input)
		}
	}
}

func TestPlanChangesBuildsAtomicSnapshotWithoutMutatingInput(t *testing.T) {
	trip := blockReadFixture()
	before, _ := json.Marshal(trip)
	changes, err := parsePlanChanges([]byte(`[{"block_id":101,"name":"New museum","start":"10:30","duration_minutes":90},{"block_id":102,"markdown":"**Reminder**\n- Tickets"}]`))
	if err != nil {
		t.Fatal(err)
	}
	result, err := buildPlanChanges(trip, planEditOptions{}, changes)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(trip)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("builder mutated snapshot")
	}
	if len(result.Report.Changes) != 2 {
		t.Fatalf("missing readable changes: %#v", result.Report)
	}
	var end, name, markdown bool
	seenPaths := map[string]bool{}
	for _, op := range result.Ops {
		path := opPaths([]map[string]any{op})[0]
		if seenPaths[path] {
			t.Fatalf("conflicting operations at %s", path)
		}
		seenPaths[path] = true
		if strings.HasSuffix(path, "endTime") && op["oi"] == "12:00" {
			end = true
		}
		if strings.HasSuffix(path, "place.name") && op["oi"] == "New museum" {
			name = true
		}
		if path == "itinerary.sections.1.blocks.0.text" {
			markdown = true
		}
		if op["li"] != nil || op["ld"] != nil {
			t.Fatal("update batch unexpectedly inserts/deletes blocks")
		}
	}
	if !end || !name || !markdown {
		t.Fatalf("missing semantic changes: %#v", result.Ops)
	}
	preview := result.Report.Changes[0]["after"].(map[string]any)
	if preview["endTime"] != "12:00" || preview["name"] != "New museum" {
		t.Fatalf("bad preview: %#v", preview)
	}
}

func TestPlanChangesAnyInvalidBlockProducesNoPartialOps(t *testing.T) {
	for _, input := range []string{
		`[{"block_id":101,"name":"Updated"},{"block_id":999,"markdown":"bad"}]`,
		`[{"block_id":101,"name":"Updated"},{"block_id":102,"name":"Not a place"}]`,
		`[{"block_id":101,"start":"09:00","end":"10:00","duration_minutes":90}]`,
	} {
		changes, err := parsePlanChanges([]byte(input))
		if err != nil {
			t.Fatal(err)
		}
		result, err := buildPlanChanges(blockReadFixture(), planEditOptions{}, changes)
		if err == nil || len(result.Ops) != 0 {
			t.Fatalf("partial ops escaped on error: %s %#v %v", input, result, err)
		}
	}
}

func TestPlanChangesDryRunParsesFileBeforeClient(t *testing.T) {
	file := t.TempDir() + "/changes.json"
	if err := os.WriteFile(file, []byte(`[{"block_id":101,"action":"delete"}]`), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := newNovelPlanEditCmd(&rootFlags{dryRun: true})
	cmd.SetArgs([]string{"--changes-file", file})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("dry-run skipped real validation: %v", err)
	}
}

func TestPlanChangesRenamePreviewDoesNotEchoNote(t *testing.T) {
	changes, err := parsePlanChanges([]byte(`[{"block_id":101,"name":"New name"}]`))
	if err != nil {
		t.Fatal(err)
	}
	result, err := buildPlanChanges(blockReadFixture(), planEditOptions{}, changes)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(result.Report.Changes)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > 600 || strings.Contains(string(data), "A long complete note") {
		t.Fatalf("rename preview too large: %d", len(data))
	}
}

func TestPlanModelCommandsHelpExamplesDescribeRealInputs(t *testing.T) {
	for _, cmd := range []*cobra.Command{newNovelPlanBlockGetCmd(&rootFlags{}), newNovelPlanEditCmd(&rootFlags{})} {
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--help"})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		text := out.String()
		if !strings.Contains(text, "Examples:") || !strings.Contains(text, "--target-key YOUR_TRIP_KEY") {
			t.Fatalf("missing input example: %s", text)
		}
		if cmd.Name() == "get" && !strings.Contains(text, "--block-id 123") {
			t.Fatal("block example omitted stable id")
		}
		if cmd.Name() == "edit" && (!strings.Contains(text, "--changes-file changes.json --dry-run") || !strings.Contains(text, "still reads and validates")) {
			t.Fatal("batch example omitted real file validation")
		}
	}
}
