// Copyright 2026 vinny-pasceri. Licensed under Apache-2.0. See LICENSE.
// Tests for the CSV + JSON caller-mapping import (Task 9).
// All fixtures are synthetic — no real tenant ticket-type or venue names.
package cli

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/store"
)

// openSeededStoreForImport opens an empty temp store for import tests.
func openSeededStoreForImport(t *testing.T) *store.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("opening import test store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestImportMappingCSVAndJSON(t *testing.T) {
	s := openSeededStoreForImport(t)
	csvData := "entity_type,source_value,canonical_name,external_id\nticket_type,weird vip name,vip experience,sanity-123\n"
	n, err := importMapping(s, "dice", []byte(csvData), "csv")
	if err != nil || n != 1 {
		t.Fatalf("csv import: n=%d err=%v", n, err)
	}
	jsonDoc := `[{"entity_type":"venue","source_value":"odd venue","canonical_name":"northside hall","external_id":"sanity-456"}]`
	if n, err := importMapping(s, "dice", []byte(jsonDoc), "json"); err != nil || n != 1 {
		t.Fatalf("json import: n=%d err=%v", n, err)
	}
	cw, _ := s.ListCrosswalk("ticket_type", "dice")
	if len(cw) != 1 || cw[0].Method != "manual" {
		t.Fatalf("want manual crosswalk row, got %+v", cw)
	}
}

func TestImportMappingCSVNoExternalID(t *testing.T) {
	s := openSeededStoreForImport(t)
	// CSV without the optional external_id column.
	csvData := "entity_type,source_value,canonical_name\nticket_type,basic name,general admission\n"
	n, err := importMapping(s, "dice", []byte(csvData), "csv")
	if err != nil || n != 1 {
		t.Fatalf("csv no-external-id import: n=%d err=%v", n, err)
	}
	cw, _ := s.ListCrosswalk("ticket_type", "dice")
	if len(cw) != 1 || cw[0].Method != "manual" {
		t.Fatalf("want 1 manual row, got %+v", cw)
	}
}

func TestImportMappingJSONNoExternalID(t *testing.T) {
	s := openSeededStoreForImport(t)
	jsonDoc := `[{"entity_type":"venue","source_value":"plain venue","canonical_name":"northside hall"}]`
	n, err := importMapping(s, "dice", []byte(jsonDoc), "json")
	if err != nil || n != 1 {
		t.Fatalf("json no-external-id import: n=%d err=%v", n, err)
	}
	cw, _ := s.ListCrosswalk("venue", "dice")
	if len(cw) != 1 || cw[0].Method != "manual" {
		t.Fatalf("want 1 manual venue row, got %+v", cw)
	}
}

func TestImportMappingCanonicalizesName(t *testing.T) {
	s := openSeededStoreForImport(t)
	// canonical_name has extra whitespace and mixed case; it should be normalized.
	csvData := "entity_type,source_value,canonical_name\nticket_type,raw source,  General  Admission  \n"
	if _, err := importMapping(s, "dice", []byte(csvData), "csv"); err != nil {
		t.Fatalf("import: %v", err)
	}
	cw, _ := s.ListCrosswalk("ticket_type", "dice")
	if len(cw) != 1 {
		t.Fatalf("want 1 row, got %d", len(cw))
	}
	// The canonical_id should be derived from the normalized form.
	want := mintCanonicalID("ticket_type", "general admission")
	if cw[0].CanonicalID != want {
		t.Errorf("canonical_id = %q, want %q (derived from normalized name)", cw[0].CanonicalID, want)
	}
}

func TestImportMappingUnknownFormat(t *testing.T) {
	s := openSeededStoreForImport(t)
	_, err := importMapping(s, "dice", []byte("data"), "xml")
	if err == nil {
		t.Error("want error for unknown format, got nil")
	}
}

// TestImportCSVAllEmptyAxisValues verifies that when a CSV has axis columns but
// every axis value for a row is empty/zero, no tier_attributes row is written
// for that row. The crosswalk method=manual row must still be written.
func TestImportCSVAllEmptyAxisValues(t *testing.T) {
	s := openSeededStoreForImport(t)
	// CSV has all axis columns, but values are empty for "empty name".
	csvData := "source_value,access_class,sales_stage,entry_window_type,entry_window_time,group_size,comp_flag\n" +
		"empty name,,,,,,\n"
	n, err := importMapping(s, "dice", []byte(csvData), "csv")
	if err != nil {
		t.Fatalf("csv import: %v", err)
	}
	if n != 1 {
		t.Fatalf("want 1 row imported, got %d", n)
	}

	// Crosswalk row must exist with method=manual.
	cw, err := s.ListCrosswalk("ticket_type", "dice")
	if err != nil {
		t.Fatalf("ListCrosswalk: %v", err)
	}
	if len(cw) != 1 || cw[0].Method != "manual" {
		t.Fatalf("want 1 manual crosswalk row, got %+v", cw)
	}

	// tier_attributes must NOT be written when all axis values are empty.
	ta, err := s.ListTierAttributes("ticket_type")
	if err != nil {
		t.Fatalf("ListTierAttributes: %v", err)
	}
	if len(ta) != 0 {
		t.Errorf("want 0 tier_attributes rows for all-empty axis row, got %d: %+v", len(ta), ta)
	}
}

// TestImportJSONAllEmptyAxisValues verifies the same guarantee on the JSON path:
// a row with all-empty axis values must not write a tier_attributes row.
func TestImportJSONAllEmptyAxisValues(t *testing.T) {
	s := openSeededStoreForImport(t)
	jsonDoc := `[{"source_value":"empty json row","access_class":"","sales_stage":"","entry_window_type":"","entry_window_time":"","group_size":0,"comp_flag":false}]`
	n, err := importMapping(s, "dice", []byte(jsonDoc), "json")
	if err != nil {
		t.Fatalf("json import: %v", err)
	}
	if n != 1 {
		t.Fatalf("want 1 row imported, got %d", n)
	}

	// Crosswalk row must exist with method=manual.
	cw, err := s.ListCrosswalk("ticket_type", "dice")
	if err != nil {
		t.Fatalf("ListCrosswalk: %v", err)
	}
	if len(cw) != 1 || cw[0].Method != "manual" {
		t.Fatalf("want 1 manual crosswalk row, got %+v", cw)
	}

	// tier_attributes must NOT be written.
	ta, err := s.ListTierAttributes("ticket_type")
	if err != nil {
		t.Fatalf("ListTierAttributes: %v", err)
	}
	if len(ta) != 0 {
		t.Errorf("want 0 tier_attributes rows for all-empty axis row, got %d: %+v", len(ta), ta)
	}
}

// TestImportCSVNonEmptyAxisValuesWritesTierAttributes verifies that when a CSV
// row has at least one non-empty axis value, tier_attributes is written
// (regression-guard for the existing behavior).
func TestImportCSVNonEmptyAxisValuesWritesTierAttributes(t *testing.T) {
	s := openSeededStoreForImport(t)
	csvData := "source_value,access_class,sales_stage\n" +
		"vip name,vip,\n"
	n, err := importMapping(s, "dice", []byte(csvData), "csv")
	if err != nil {
		t.Fatalf("csv import: %v", err)
	}
	if n != 1 {
		t.Fatalf("want 1 row imported, got %d", n)
	}

	ta, err := s.ListTierAttributes("ticket_type")
	if err != nil {
		t.Fatalf("ListTierAttributes: %v", err)
	}
	if len(ta) != 1 {
		t.Fatalf("want 1 tier_attributes row for non-empty axis, got %d", len(ta))
	}
	if ta[0].AccessClass != "vip" {
		t.Errorf("access_class = %q, want %q", ta[0].AccessClass, "vip")
	}
}

// TestImportJSONNonEmptyAxisValueWritesTierAttributes verifies that a JSON row
// with at least one non-empty axis value writes tier_attributes.
func TestImportJSONNonEmptyAxisValueWritesTierAttributes(t *testing.T) {
	s := openSeededStoreForImport(t)
	jsonDoc := `[{"source_value":"ga name","access_class":"ga","sales_stage":"","group_size":0,"comp_flag":false}]`
	n, err := importMapping(s, "dice", []byte(jsonDoc), "json")
	if err != nil {
		t.Fatalf("json import: %v", err)
	}
	if n != 1 {
		t.Fatalf("want 1 row imported, got %d", n)
	}

	ta, err := s.ListTierAttributes("ticket_type")
	if err != nil {
		t.Fatalf("ListTierAttributes: %v", err)
	}
	if len(ta) != 1 {
		t.Fatalf("want 1 tier_attributes row, got %d", len(ta))
	}
	if ta[0].AccessClass != "ga" {
		t.Errorf("access_class = %q, want %q", ta[0].AccessClass, "ga")
	}
}
