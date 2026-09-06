// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// These outputs are synthetic report-writer projections, not real mutations.
// Keep file payload visible in the manifest so batching does not hide input cost.
func TestCreateBatchTokenWorkload(t *testing.T) {
	input := `[{"type":"place","day":1,"place_id":"synthetic-museum","text":"Collect advance tickets before entering."},{"type":"note","day":1,"text":"Global booking constraint: allow 45 minutes before boarding."},{"type":"checklist","day":2,"title":"Before departure","items":["Bring printed ticket","Carry water"]}]`
	entries, err := parsePlanCreateEntries([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	places := map[string]map[string]any{"id:synthetic-museum": {"place_id": "synthetic-museum", "name": "Synthetic museum", "business_status": "OPERATIONAL"}}
	trip := blockReadFixture()
	preview, err := buildPlanCreateBatch(trip, planEditOptions{}, entries, places, "block", false)
	if err != nil {
		t.Fatal(err)
	}
	applied, err := buildPlanCreateBatch(trip, planEditOptions{apply: true}, entries, places, "block", true)
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"Collect advance tickets", "45 minutes before boarding", "Bring printed ticket"} {
		b, _ := json.Marshal(preview.Report.Changes)
		if !strings.Contains(string(b), text) {
			t.Fatalf("preview lost %q", text)
		}
	}
	dir := os.Getenv("WANDERLOG_TOKEN_BENCH_DIR")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	write := func(name string, data []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	// Normalize only allocated IDs in synthetic stdout, retaining the real report
	// writer's ordering/format. Otherwise each generation changes artifact bytes.
	canonicalIDs := map[int]int{}
	for _, result := range []planEditBuildResult{preview, applied} {
		for i, op := range result.Ops {
			block := op["li"].(map[string]any)
			canonicalIDs[intAny(block["id"])] = 800000001 + i
			items, _ := block["items"].([]any)
			for j, raw := range items {
				item := raw.(map[string]any)
				canonicalIDs[intAny(item["id"])] = 810000001 + i*100 + j
			}
		}
	}
	idToken := regexp.MustCompile(`\b[0-9]{9}\b`)
	render := func(report planEditReport, apply bool) []byte {
		t.Helper()
		report.TargetKey = "naertjcoixqrgrfc"
		report.Validation = "valid"
		report.Applied = apply
		report.ApplyRequested = apply
		report.DryRun = !apply
		if apply {
			report.Version = 42
		}
		var out bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetOut(&out)
		if err := printPlanEditReport(cmd, &rootFlags{agent: true, asJSON: true, compact: true}, report); err != nil {
			t.Fatal(err)
		}
		return idToken.ReplaceAllFunc(out.Bytes(), func(raw []byte) []byte {
			id, _ := strconv.Atoi(string(raw))
			if canonical, ok := canonicalIDs[id]; ok {
				return []byte(strconv.Itoa(canonical))
			}
			return raw
		})
	}
	write("create-input.json", []byte(input))
	write("after-create-preview.json", render(preview.Report, false))
	write("after-create-applied.json", render(applied.Report, true))
	beforeCommands := []string{
		"wanderlog-pp-cli plan place add --target-key naertjcoixqrgrfc --day 1 --place-id synthetic-museum --text 'Collect advance tickets before entering.' --agent",
		"wanderlog-pp-cli plan note add --target-key naertjcoixqrgrfc --day 1 --text 'Global booking constraint: allow 45 minutes before boarding.' --agent",
		"wanderlog-pp-cli plan checklist add --target-key naertjcoixqrgrfc --day 2 --title 'Before departure' --item 'Bring printed ticket' --item 'Carry water' --agent",
	}
	beforePaths, appliedPaths := []string{}, []string{}
	commandNames := []string{"plan place add", "plan note add", "plan checklist add"}
	for i, op := range preview.Ops {
		block := op["li"].(map[string]any)
		section, err := resolveSection(trip, entries[i].Day, -1, 0)
		if err != nil {
			t.Fatal(err)
		}
		report := baseEditReport(commandNames[i], planEditOptions{}, trip)
		report.Section = ptrSectionReport(section.Report)
		report.Block = summarizeBlock(block)
		report.BlockID = intAny(block["id"])
		path := op["p"].([]any)
		report.BlockIndex = intAny(path[len(path)-1])
		report.Section.BlockCount = report.BlockIndex
		report.Operation = "insert " + entries[i].Type + " block"
		name := "before-create-" + entries[i].Type + "-preview.json"
		beforePaths = append(beforePaths, name)
		write(name, render(report, false))
		name = "before-create-" + entries[i].Type + "-applied.json"
		appliedPaths = append(appliedPaths, name)
		write(name, render(report, true))
	}
	applyCommands := []string{}
	for _, command := range beforeCommands {
		applyCommands = append(applyCommands, command+" --apply")
	}
	manifest := map[string]any{"description": "Synthetic report writers for three equivalent individual creations versus one atomic batch; both use candidate agent serialization; allocated synthetic IDs normalized deterministically; no actual write. Applied receipts are simulated outcomes. Batch payload charged once in preview and combined workflow, not again when reusing it for apply.", "cases": map[string]any{
		"before_create_preview": map[string]any{"commands": beforeCommands, "outputs": beforePaths},
		"before_create_applied": map[string]any{"commands": applyCommands, "outputs": appliedPaths},
		"after_create_preview":  map[string]any{"commands": []string{"wanderlog-pp-cli plan block add-batch --target-key naertjcoixqrgrfc --blocks-file create-input.json --agent"}, "outputs": []string{"after-create-preview.json"}, "payload_files": []string{"create-input.json"}},
		"after_create_applied":  map[string]any{"commands": []string{"wanderlog-pp-cli plan block add-batch --target-key naertjcoixqrgrfc --blocks-file create-input.json --agent --apply"}, "outputs": []string{"after-create-applied.json"}},
	}}
	encoded, _ := json.MarshalIndent(manifest, "", "  ")
	write("create-workload.json", encoded)
}
