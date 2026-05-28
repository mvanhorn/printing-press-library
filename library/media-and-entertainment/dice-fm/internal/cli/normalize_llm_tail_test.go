// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// TDD tests for the LLM-tail classification round-trip (export-format prompt +
// extended import with tier-axis columns). All fixtures use synthetic,
// generic names — no real tenant data.
package cli

import (
	"bytes"
	"context"
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/store"
)

// --- helpers ----------------------------------------------------------------

// seedWithUnmatched seeds a store with two tickets whose names are
// deliberately opaque so classifyTiers marks them unmatched, then runs
// classifyTiers so entity_crosswalk rows with method='unmatched' exist.
func seedWithUnmatched(t *testing.T) *store.Store {
	t.Helper()
	s := seedStore(t, map[string]map[string]string{
		"tickets": {
			"u1": `{"id":"u1","ticketType":{"name":"alpha package"}}`,
			"u2": `{"id":"u2","ticketType":{"name":"beta bundle"}}`,
		},
	})
	if _, err := classifyTiers(context.Background(), s, classifyOpts{ClassifierVersion: 1}); err != nil {
		t.Fatalf("seedWithUnmatched classifyTiers: %v", err)
	}
	return s
}

// --- export-format tests ----------------------------------------------------

// TestExportFormatPromptContainsRubric verifies that --export-format prompt
// writes a file that includes all five axis names and their allowed values,
// the import-schema column list, and the seeded unmatched source_value names.
func TestExportFormatPromptContainsRubric(t *testing.T) {
	s := seedWithUnmatched(t)
	outPath := filepath.Join(t.TempDir(), "tail.txt")

	if err := exportUnmatchedWithFormat(context.Background(), s, "ticket_type", outPath, "prompt"); err != nil {
		t.Fatalf("exportUnmatchedWithFormat prompt: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read exported file: %v", err)
	}
	content := string(data)

	// All five axis names must appear.
	for _, axis := range []string{"access_class", "sales_stage", "entry_window_type", "entry_window_time", "group_size", "comp_flag"} {
		if !strings.Contains(content, axis) {
			t.Errorf("prompt export missing axis %q", axis)
		}
	}

	// Allowed values for access_class.
	for _, v := range []string{"ga", "vip", "premium"} {
		if !strings.Contains(content, v) {
			t.Errorf("prompt export missing access_class value %q", v)
		}
	}

	// Allowed values for sales_stage.
	for _, v := range []string{"super_early_bird", "early_bird", "final_release", "last_chance", "tier_n"} {
		if !strings.Contains(content, v) {
			t.Errorf("prompt export missing sales_stage value %q", v)
		}
	}

	// Entry window allowed values.
	for _, v := range []string{"deadline", "anytime", "door"} {
		if !strings.Contains(content, v) {
			t.Errorf("prompt export missing entry_window_type value %q", v)
		}
	}

	// Import schema column list must appear.
	for _, col := range []string{"source_value", "access_class", "sales_stage", "entry_window_type", "entry_window_time", "group_size", "comp_flag"} {
		if !strings.Contains(content, col) {
			t.Errorf("prompt export missing import schema column %q", col)
		}
	}

	// Seeded unmatched names must appear.
	for _, name := range []string{"alpha package", "beta bundle"} {
		if !strings.Contains(content, name) {
			t.Errorf("prompt export missing unmatched name %q", name)
		}
	}
}

// TestExportFormatNamesIsJustNames verifies that --export-format names writes
// only the source_value names (original CSV behaviour) with no rubric text.
func TestExportFormatNamesIsJustNames(t *testing.T) {
	s := seedWithUnmatched(t)
	outPath := filepath.Join(t.TempDir(), "names.csv")

	if err := exportUnmatchedWithFormat(context.Background(), s, "ticket_type", outPath, "names"); err != nil {
		t.Fatalf("exportUnmatchedWithFormat names: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read exported file: %v", err)
	}
	content := string(data)

	// Must NOT contain rubric text.
	for _, rubricWord := range []string{"access_class", "sales_stage", "entry_window_type", "CLASSIFICATION", "rubric", "allowed values"} {
		if strings.Contains(strings.ToLower(content), strings.ToLower(rubricWord)) {
			t.Errorf("names export must not contain rubric text %q; got:\n%s", rubricWord, content)
		}
	}

	// Must contain the names.
	for _, name := range []string{"alpha package", "beta bundle"} {
		if !strings.Contains(content, name) {
			t.Errorf("names export missing name %q", name)
		}
	}
}

// TestExportFormatPromptIsDefault verifies that the default export format
// (no explicit --export-format) writes a prompt file (rubric present).
func TestExportFormatPromptIsDefault(t *testing.T) {
	s := seedWithUnmatched(t)
	outPath := filepath.Join(t.TempDir(), "out.txt")

	// exportUnmatchedWithFormat with empty format string must behave as "prompt".
	if err := exportUnmatchedWithFormat(context.Background(), s, "ticket_type", outPath, ""); err != nil {
		t.Fatalf("exportUnmatchedWithFormat empty: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read exported file: %v", err)
	}
	if !strings.Contains(string(data), "access_class") {
		t.Error("default export format must be 'prompt' (rubric expected)")
	}
}

// TestExportFormatFlagRegistered verifies --export-format is wired to the
// normalize cobra command.
func TestExportFormatFlagRegistered(t *testing.T) {
	flags := &rootFlags{}
	cmd := newNormalizeCmd(flags)
	if cmd.Flags().Lookup("export-format") == nil {
		t.Error("flag --export-format not registered on normalize")
	}
}

// TestExportFormatFlagInvalidRejected verifies that an unknown export-format
// value propagates an error when the user would execute the command.
func TestExportFormatFlagInvalidRejected(t *testing.T) {
	err := exportUnmatchedWithFormat(context.Background(), nil, "ticket_type", "/dev/null", "xml")
	if err == nil {
		t.Error("expected error for unknown export-format 'xml', got nil")
	}
}

// --- import-with-axes tests -------------------------------------------------

// TestImportWithAxesCSV verifies that a CSV with axis columns writes
// tier_attributes rows with method="manual" and the correct axis values.
func TestImportWithAxesCSV(t *testing.T) {
	s := openSeededStoreForImport(t)

	csvData := "source_value,access_class,sales_stage,entry_window_type,entry_window_time,group_size,comp_flag\n" +
		"alpha package,ga,early_bird,anytime,,0,false\n"

	n, err := importMapping(s, "dice", "ticket_type", []byte(csvData), "csv")
	if err != nil {
		t.Fatalf("importMapping csv with axes: %v", err)
	}
	if n != 1 {
		t.Fatalf("want 1 row imported, got %d", n)
	}

	// Crosswalk must have method=manual.
	cw, err := s.ListCrosswalk("ticket_type", "dice")
	if err != nil {
		t.Fatalf("ListCrosswalk: %v", err)
	}
	if len(cw) != 1 || cw[0].Method != "manual" {
		t.Fatalf("want 1 manual crosswalk row, got %+v", cw)
	}

	// tier_attributes must exist with the right values.
	attrs, err := s.ListTierAttributes("ticket_type")
	if err != nil {
		t.Fatalf("ListTierAttributes: %v", err)
	}
	if len(attrs) != 1 {
		t.Fatalf("want 1 tier_attributes row, got %d", len(attrs))
	}
	a := attrs[0]
	if a.AccessClass != "ga" {
		t.Errorf("access_class: got %q, want %q", a.AccessClass, "ga")
	}
	if a.SalesStage != "early_bird" {
		t.Errorf("sales_stage: got %q, want %q", a.SalesStage, "early_bird")
	}
	if a.EntryWindowType != "anytime" {
		t.Errorf("entry_window_type: got %q, want %q", a.EntryWindowType, "anytime")
	}
	if a.EntryWindowTime != "" {
		t.Errorf("entry_window_time: got %q, want empty", a.EntryWindowTime)
	}
	if a.GroupSize != 0 {
		t.Errorf("group_size: got %d, want 0", a.GroupSize)
	}
	if a.CompFlag {
		t.Errorf("comp_flag: got true, want false")
	}
	if a.Method != "manual" {
		t.Errorf("tier_attributes method: got %q, want manual", a.Method)
	}
}

// TestImportWithAxesJSON verifies that a JSON array with axis keys writes
// tier_attributes rows with method="manual" and the correct axis values.
func TestImportWithAxesJSON(t *testing.T) {
	s := openSeededStoreForImport(t)

	jsonDoc := `[{
		"source_value": "beta bundle",
		"access_class": "vip",
		"sales_stage": "final_release",
		"entry_window_type": "deadline",
		"entry_window_time": "22:00",
		"group_size": 3,
		"comp_flag": true
	}]`

	n, err := importMapping(s, "dice", "ticket_type", []byte(jsonDoc), "json")
	if err != nil {
		t.Fatalf("importMapping json with axes: %v", err)
	}
	if n != 1 {
		t.Fatalf("want 1 row imported, got %d", n)
	}

	attrs, err := s.ListTierAttributes("ticket_type")
	if err != nil {
		t.Fatalf("ListTierAttributes: %v", err)
	}
	if len(attrs) != 1 {
		t.Fatalf("want 1 tier_attributes row, got %d", len(attrs))
	}
	a := attrs[0]
	if a.AccessClass != "vip" {
		t.Errorf("access_class: got %q, want vip", a.AccessClass)
	}
	if a.SalesStage != "final_release" {
		t.Errorf("sales_stage: got %q, want final_release", a.SalesStage)
	}
	if a.EntryWindowType != "deadline" {
		t.Errorf("entry_window_type: got %q, want deadline", a.EntryWindowType)
	}
	if a.EntryWindowTime != "22:00" {
		t.Errorf("entry_window_time: got %q, want 22:00", a.EntryWindowTime)
	}
	if a.GroupSize != 3 {
		t.Errorf("group_size: got %d, want 3", a.GroupSize)
	}
	if !a.CompFlag {
		t.Error("comp_flag: got false, want true")
	}
	if a.Method != "manual" {
		t.Errorf("tier_attributes method: got %q, want manual", a.Method)
	}
}

// TestImportWithAxesGroupSizeString verifies that group_size as a quoted
// string integer in JSON is parsed correctly.
func TestImportWithAxesGroupSizeString(t *testing.T) {
	s := openSeededStoreForImport(t)

	// Simulate an LLM that emits group_size as a JSON string rather than number.
	jsonDoc := `[{"source_value":"gamma ticket","access_class":"ga","group_size":"2","comp_flag":"false"}]`

	_, err := importMapping(s, "dice", "ticket_type", []byte(jsonDoc), "json")
	if err != nil {
		t.Fatalf("importMapping json group_size string: %v", err)
	}
	attrs, err := s.ListTierAttributes("ticket_type")
	if err != nil {
		t.Fatalf("ListTierAttributes: %v", err)
	}
	if len(attrs) != 1 {
		t.Fatalf("want 1 tier_attributes row, got %d", len(attrs))
	}
	if attrs[0].GroupSize != 2 {
		t.Errorf("group_size: got %d, want 2", attrs[0].GroupSize)
	}
	if attrs[0].CompFlag {
		t.Error("comp_flag: got true, want false")
	}
}

// TestImportWithAxesSurvivesClearNormalization verifies that tier_attributes
// rows written via importMapping with method="manual" survive a subsequent
// ClearNormalization call (the manual-preserving contract).
func TestImportWithAxesSurvivesClearNormalization(t *testing.T) {
	s := openSeededStoreForImport(t)

	csvData := "source_value,access_class,sales_stage\nalpha package,vip,early_bird\n"
	if _, err := importMapping(s, "dice", "ticket_type", []byte(csvData), "csv"); err != nil {
		t.Fatalf("importMapping: %v", err)
	}

	// Confirm tier_attributes written.
	attrsBefore, err := s.ListTierAttributes("ticket_type")
	if err != nil {
		t.Fatalf("ListTierAttributes before clear: %v", err)
	}
	if len(attrsBefore) == 0 {
		t.Fatal("tier_attributes row expected before ClearNormalization")
	}

	// Run ClearNormalization — manual rows must be preserved.
	if err := s.ClearNormalization("ticket_type"); err != nil {
		t.Fatalf("ClearNormalization: %v", err)
	}

	// Crosswalk manual row must survive.
	cw, _ := s.ListCrosswalk("ticket_type", "dice")
	manualFound := false
	for _, r := range cw {
		if r.SourceValue == "alpha package" && r.Method == "manual" {
			manualFound = true
		}
	}
	if !manualFound {
		t.Errorf("manual crosswalk row did not survive ClearNormalization; rows=%+v", cw)
	}

	// tier_attributes must also survive because the crosswalk row has method=manual
	// which prevents attribute deletion.
	attrsAfter, err := s.ListTierAttributes("ticket_type")
	if err != nil {
		t.Fatalf("ListTierAttributes after clear: %v", err)
	}
	if len(attrsAfter) == 0 {
		t.Error("tier_attributes row should survive ClearNormalization when crosswalk is manual")
	}
}

// TestImportAxesRunNormalizeSurvivesRerun verifies that manual tier_attributes
// rows survive a full runNormalize re-classify cycle (round-trip persistence).
func TestImportAxesRunNormalizeSurvivesRerun(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"tickets": {
			"t1": `{"id":"t1","ticketType":{"name":"alpha package"}}`,
		},
	})

	// Import with axes.
	csvData := "source_value,access_class,sales_stage\nalpha package,premium,last_chance\n"
	opts := normalizeOpts{
		Tiers:             true,
		ClassifierVersion: 1,
		ImportData:        []byte(csvData),
		ImportFormat:      "csv",
	}
	var buf bytes.Buffer
	if err := runNormalize(context.Background(), s, opts, &buf); err != nil {
		t.Fatalf("first runNormalize: %v", err)
	}

	// Second run (re-classify only).
	opts2 := normalizeOpts{Tiers: true, ClassifierVersion: 1}
	var buf2 bytes.Buffer
	if err := runNormalize(context.Background(), s, opts2, &buf2); err != nil {
		t.Fatalf("second runNormalize: %v", err)
	}

	// Manual tier_attributes must still be present.
	attrs, err := s.ListTierAttributes("ticket_type")
	if err != nil {
		t.Fatalf("ListTierAttributes after rerun: %v", err)
	}
	found := false
	for _, a := range attrs {
		if a.AccessClass == "premium" && a.SalesStage == "last_chance" && a.Method == "manual" {
			found = true
		}
	}
	if !found {
		t.Errorf("manual tier_attributes did not survive second runNormalize; attrs=%+v", attrs)
	}
}

// TestImportBackwardCompatCSVNoAxes verifies that a CSV with only
// source_value,canonical_name (no axis columns) still works and writes NO
// tier_attributes rows (backward compat).
func TestImportBackwardCompatCSVNoAxes(t *testing.T) {
	s := openSeededStoreForImport(t)
	csvData := "entity_type,source_value,canonical_name\nticket_type,plain name,general admission\n"
	n, err := importMapping(s, "dice", "ticket_type", []byte(csvData), "csv")
	if err != nil || n != 1 {
		t.Fatalf("backward compat csv import: n=%d err=%v", n, err)
	}
	cw, _ := s.ListCrosswalk("ticket_type", "dice")
	if len(cw) != 1 || cw[0].Method != "manual" {
		t.Fatalf("want 1 manual crosswalk row, got %+v", cw)
	}
	attrs, err := s.ListTierAttributes("ticket_type")
	if err != nil {
		t.Fatalf("ListTierAttributes: %v", err)
	}
	if len(attrs) != 0 {
		t.Errorf("no tier_attributes expected for axis-free import, got %+v", attrs)
	}
}

// TestImportWithAxesCSVMixed verifies that a CSV carrying both canonical_name
// and axis columns sets both the crosswalk canonical_name AND tier_attributes.
func TestImportWithAxesCSVMixed(t *testing.T) {
	s := openSeededStoreForImport(t)

	csvData := "entity_type,source_value,canonical_name,access_class,sales_stage\n" +
		"ticket_type,delta pass,vip experience,vip,super_early_bird\n"

	n, err := importMapping(s, "dice", "ticket_type", []byte(csvData), "csv")
	if err != nil || n != 1 {
		t.Fatalf("mixed csv import: n=%d err=%v", n, err)
	}

	cw, _ := s.ListCrosswalk("ticket_type", "dice")
	if len(cw) != 1 || cw[0].Method != "manual" {
		t.Fatalf("want 1 manual crosswalk, got %+v", cw)
	}

	attrs, err := s.ListTierAttributes("ticket_type")
	if err != nil {
		t.Fatalf("ListTierAttributes: %v", err)
	}
	if len(attrs) != 1 {
		t.Fatalf("want 1 tier_attributes row for mixed import, got %d", len(attrs))
	}
	if attrs[0].AccessClass != "vip" || attrs[0].SalesStage != "super_early_bird" {
		t.Errorf("tier_attributes axes wrong: %+v", attrs[0])
	}
}

// TestImportAxesCSVRoundTripViaExportPrompt verifies the full LLM-tail
// round-trip: export prompt → parse names from it → build a classification
// CSV → import back → assert tier_attributes written.
func TestImportAxesCSVRoundTripViaExportPrompt(t *testing.T) {
	s := seedWithUnmatched(t)
	outPath := filepath.Join(t.TempDir(), "tail.txt")

	// Export as prompt.
	if err := exportUnmatchedWithFormat(context.Background(), s, "ticket_type", outPath, "prompt"); err != nil {
		t.Fatalf("export prompt: %v", err)
	}

	// Simulate the LLM classifying the names: write a classification CSV
	// for each name in the exported file.
	classificationCSV := "source_value,access_class,sales_stage\nalpha package,ga,early_bird\nbeta bundle,vip,final_release\n"

	// Import the classification CSV.
	n, err := importMapping(s, "dice", "ticket_type", []byte(classificationCSV), "csv")
	if err != nil {
		t.Fatalf("import classification csv: %v", err)
	}
	if n != 2 {
		t.Fatalf("want 2 rows imported, got %d", n)
	}

	attrs, err := s.ListTierAttributes("ticket_type")
	if err != nil {
		t.Fatalf("ListTierAttributes: %v", err)
	}
	if len(attrs) != 2 {
		t.Fatalf("want 2 tier_attributes rows after round-trip, got %d", len(attrs))
	}
}

// TestExportFormatPromptCSVNamesParseable verifies that the names section of a
// prompt export can be parsed by encoding/csv (names are CSV-safe for programmatic
// consumers even inside the prompt format).
func TestExportFormatPromptCSVNamesParseable(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"tickets": {
			"x1": `{"id":"x1","ticketType":{"name":"name with, comma"}}`,
			"x2": `{"id":"x2","ticketType":{"name":"name with \"quote\""}}`,
		},
	})
	if _, err := classifyTiers(context.Background(), s, classifyOpts{ClassifierVersion: 1}); err != nil {
		t.Fatalf("classifyTiers: %v", err)
	}

	outPath := filepath.Join(t.TempDir(), "tail.txt")
	if err := exportUnmatchedWithFormat(context.Background(), s, "ticket_type", outPath, "prompt"); err != nil {
		t.Fatalf("exportUnmatchedWithFormat: %v", err)
	}

	// The names in the prompt must appear in some form.
	data, _ := os.ReadFile(outPath)
	content := string(data)
	if !strings.Contains(content, "name with") {
		t.Error("prompt export must contain unmatched names section")
	}

	// The CSV names section must be parseable.
	// Extract the names section — everything after the last "---" separator.
	parts := strings.Split(content, "---")
	if len(parts) < 2 {
		t.Fatal("prompt format must use --- separator between instructions and names")
	}
	namesSection := strings.TrimSpace(parts[len(parts)-1])
	r := csv.NewReader(strings.NewReader(namesSection))
	r.Comment = '#'
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("names section of prompt is not valid CSV: %v\ncontent:\n%s", err, namesSection)
	}
	if len(records) < 1 {
		t.Fatal("names section is empty")
	}
}
