// Copyright 2026 vinny-pasceri. Licensed under Apache-2.0. See LICENSE.
// Hand-authored CSV + JSON caller-mapping import for the entity normalization
// layer. Converts operator-supplied mapping files into method="manual" crosswalk
// rows that survive re-classification runs.
package cli

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/store"
)

// importRow is the parsed shape of a single mapping entry from CSV or JSON.
type importRow struct {
	EntityType    string `json:"entity_type"`
	SourceValue   string `json:"source_value"`
	CanonicalName string `json:"canonical_name"`
	ExternalID    string `json:"external_id"` // optional
}

// importMapping parses data in the given format ("csv" or "json"), canonicalizes
// each canonical_name, mints a canonical ID, upserts the canonical entity and a
// method="manual" crosswalk row, and—when external_id is non-empty—writes a row
// to entity_external_ref. Returns the number of rows imported.
func importMapping(s *store.Store, sourceSystem string, data []byte, format string) (int, error) {
	rows, err := parseImportData(data, format)
	if err != nil {
		return 0, err
	}
	for _, r := range rows {
		canon := canonicalizeName(r.CanonicalName)
		cid := mintCanonicalID(r.EntityType, canon)

		if err := s.UpsertCanonicalEntity(r.EntityType, cid, canon); err != nil {
			return 0, fmt.Errorf("upsert canonical entity for %q: %w", r.SourceValue, err)
		}
		if err := s.UpsertCrosswalk(store.CrosswalkRow{
			EntityType:        r.EntityType,
			SourceSystem:      sourceSystem,
			SourceValue:       r.SourceValue,
			CanonicalID:       cid,
			Method:            "manual",
			ClassifierVersion: 1,
		}); err != nil {
			return 0, fmt.Errorf("upsert crosswalk for %q: %w", r.SourceValue, err)
		}
		if r.ExternalID != "" {
			if err := s.UpsertExternalRef(r.EntityType, cid, sourceSystem, r.ExternalID); err != nil {
				return 0, fmt.Errorf("upsert external ref for %q: %w", r.SourceValue, err)
			}
		}
	}
	return len(rows), nil
}

// parseImportData parses the raw bytes as either CSV or JSON.
func parseImportData(data []byte, format string) ([]importRow, error) {
	switch strings.ToLower(format) {
	case "csv":
		return parseCSV(data)
	case "json":
		return parseJSON(data)
	default:
		return nil, fmt.Errorf("unsupported import format %q: must be csv or json", format)
	}
}

// parseCSV reads a CSV byte slice with header row:
// entity_type,source_value,canonical_name[,external_id]
func parseCSV(data []byte) ([]importRow, error) {
	r := csv.NewReader(bytes.NewReader(data))
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parsing CSV: %w", err)
	}
	if len(records) < 1 {
		return nil, fmt.Errorf("CSV is empty")
	}

	// Build a column index from the header row.
	header := records[0]
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.TrimSpace(strings.ToLower(h))] = i
	}
	for _, required := range []string{"entity_type", "source_value", "canonical_name"} {
		if _, ok := idx[required]; !ok {
			return nil, fmt.Errorf("CSV missing required column %q", required)
		}
	}
	extIDCol, hasExtID := idx["external_id"]

	rows := make([]importRow, 0, len(records)-1)
	for _, rec := range records[1:] {
		if len(rec) == 0 {
			continue
		}
		row := importRow{
			EntityType:    strings.TrimSpace(rec[idx["entity_type"]]),
			SourceValue:   strings.TrimSpace(rec[idx["source_value"]]),
			CanonicalName: strings.TrimSpace(rec[idx["canonical_name"]]),
		}
		if hasExtID && extIDCol < len(rec) {
			row.ExternalID = strings.TrimSpace(rec[extIDCol])
		}
		if row.EntityType == "" || row.SourceValue == "" || row.CanonicalName == "" {
			continue
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// parseJSON reads a JSON array of objects with fields matching importRow.
func parseJSON(data []byte) ([]importRow, error) {
	var rows []importRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("parsing JSON import: %w", err)
	}
	return rows, nil
}
