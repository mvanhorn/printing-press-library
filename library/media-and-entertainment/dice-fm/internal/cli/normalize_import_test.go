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
