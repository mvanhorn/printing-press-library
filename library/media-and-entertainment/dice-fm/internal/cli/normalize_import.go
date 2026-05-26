// Copyright 2026 vinny-pasceri. Licensed under Apache-2.0. See LICENSE.
// Hand-authored CSV + JSON caller-mapping import for the entity normalization
// layer. Converts operator-supplied mapping files into method="manual" crosswalk
// rows that survive re-classification runs. When axis columns are present in the
// input, tier_attributes rows are written with method="manual" as well.
package cli

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/store"
)

// currentClassifierVersion is the classifier_version stamped on import rows.
const currentClassifierVersion = 1

// importRow is the parsed shape of a single mapping entry from CSV or JSON.
// All fields are optional depending on which columns appear in the input.
type importRow struct {
	// Core mapping fields (backward-compatible columns).
	EntityType    string `json:"entity_type"`
	SourceValue   string `json:"source_value"`
	CanonicalName string `json:"canonical_name"` // optional when axis columns present
	ExternalID    string `json:"external_id"`    // optional

	// Tier-axis columns — present when the LLM-tail classification result is
	// imported. Any of these being non-empty triggers a tier_attributes upsert.
	AccessClass     string   `json:"access_class"`
	SalesStage      string   `json:"sales_stage"`
	EntryWindowType string   `json:"entry_window_type"`
	EntryWindowTime string   `json:"entry_window_time"`
	GroupSize       flexInt  `json:"group_size"`
	CompFlag        flexBool `json:"comp_flag"`
	hasAxes         bool     // true if any axis column was populated
}

// flexInt accepts a JSON number or a JSON string containing a decimal integer.
type flexInt int

func (f *flexInt) UnmarshalJSON(data []byte) error {
	// Try number first.
	var n int
	if err := json.Unmarshal(data, &n); err == nil {
		*f = flexInt(n)
		return nil
	}
	// Try quoted string.
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("group_size: expected integer or quoted integer, got %s", data)
	}
	s = strings.TrimSpace(s)
	if s == "" {
		*f = 0
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("group_size: cannot parse %q as integer: %w", s, err)
	}
	*f = flexInt(n)
	return nil
}

// flexBool accepts a JSON boolean, the strings "true"/"false", or "1"/"0".
type flexBool bool

func (f *flexBool) UnmarshalJSON(data []byte) error {
	// Try native bool.
	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		*f = flexBool(b)
		return nil
	}
	// Try quoted string.
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("comp_flag: expected bool or quoted bool, got %s", data)
	}
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "yes":
		*f = true
	case "false", "0", "no", "":
		*f = false
	default:
		return fmt.Errorf("comp_flag: cannot parse %q as bool", s)
	}
	return nil
}

// importMapping parses data in the given format ("csv" or "json"), canonicalizes
// each source value, mints a canonical ID, upserts the canonical entity and a
// method="manual" crosswalk row. When axis columns are present it also upserts
// tier_attributes with method="manual". Backward compatible with the original
// entity_type,source_value,canonical_name[,external_id] shape.
// Returns the number of rows imported.
func importMapping(s *store.Store, sourceSystem string, data []byte, format string) (int, error) {
	rows, err := parseImportData(data, format)
	if err != nil {
		return 0, err
	}
	for _, r := range rows {
		// Determine entity type: rows produced by the LLM-tail prompt export
		// omit entity_type; default to "ticket_type".
		entityType := r.EntityType
		if entityType == "" {
			entityType = "ticket_type"
		}

		// Determine canonical name: use the provided name if present; fall back
		// to the source value itself as a placeholder for axis-only rows.
		canonName := r.CanonicalName
		if canonName == "" {
			canonName = r.SourceValue
		}
		canon := canonicalizeName(canonName)
		cid := mintCanonicalID(entityType, canon)

		if err := s.UpsertCanonicalEntity(entityType, cid, canon); err != nil {
			return 0, fmt.Errorf("upsert canonical entity for %q: %w", r.SourceValue, err)
		}
		if err := s.UpsertCrosswalk(store.CrosswalkRow{
			EntityType:        entityType,
			SourceSystem:      sourceSystem,
			SourceValue:       r.SourceValue,
			CanonicalID:       cid,
			Method:            "manual",
			ClassifierVersion: currentClassifierVersion,
		}); err != nil {
			return 0, fmt.Errorf("upsert crosswalk for %q: %w", r.SourceValue, err)
		}
		if r.ExternalID != "" {
			if err := s.UpsertExternalRef(entityType, cid, sourceSystem, r.ExternalID); err != nil {
				return 0, fmt.Errorf("upsert external ref for %q: %w", r.SourceValue, err)
			}
		}

		// Write tier_attributes when any axis column is populated.
		if r.hasAxes {
			if err := s.UpsertTierAttributes(cid, store.TierAttributesRow{
				CanonicalID:       cid,
				AccessClass:       r.AccessClass,
				SalesStage:        r.SalesStage,
				EntryWindowType:   r.EntryWindowType,
				EntryWindowTime:   r.EntryWindowTime,
				GroupSize:         int(r.GroupSize),
				CompFlag:          bool(r.CompFlag),
				ClassifierVersion: currentClassifierVersion,
				Method:            "manual",
			}); err != nil {
				return 0, fmt.Errorf("upsert tier attributes for %q: %w", r.SourceValue, err)
			}
		}
	}
	return len(rows), nil
}

// axisCols is the ordered list of tier-axis column names for column-index lookup.
var axisCols = []string{
	"access_class",
	"sales_stage",
	"entry_window_type",
	"entry_window_time",
	"group_size",
	"comp_flag",
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

// parseCSV reads a CSV byte slice with a header row. Supported column sets:
//
//   - Classic: entity_type, source_value, canonical_name [, external_id]
//   - Axis-only: source_value, access_class [, sales_stage, ...]
//   - Mixed: entity_type, source_value, canonical_name [, external_id], access_class [, ...]
//
// Any combination works; only source_value is always required.
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
	if _, ok := idx["source_value"]; !ok {
		return nil, fmt.Errorf("CSV missing required column %q", "source_value")
	}

	// Detect which optional columns are present.
	entityTypeCol, hasEntityType := idx["entity_type"]
	canonNameCol, hasCanonName := idx["canonical_name"]
	extIDCol, hasExtID := idx["external_id"]
	axisColIdx := map[string]int{}
	hasAnyAxis := false
	for _, ac := range axisCols {
		if ci, ok := idx[ac]; ok {
			axisColIdx[ac] = ci
			hasAnyAxis = true
		}
	}

	rows := make([]importRow, 0, len(records)-1)
	for _, rec := range records[1:] {
		if len(rec) == 0 {
			continue
		}
		sv := strings.TrimSpace(rec[idx["source_value"]])
		if sv == "" {
			continue
		}
		row := importRow{SourceValue: sv}

		if hasEntityType && entityTypeCol < len(rec) {
			row.EntityType = strings.TrimSpace(rec[entityTypeCol])
		}
		if hasCanonName && canonNameCol < len(rec) {
			row.CanonicalName = strings.TrimSpace(rec[canonNameCol])
		}
		if hasExtID && extIDCol < len(rec) {
			row.ExternalID = strings.TrimSpace(rec[extIDCol])
		}

		// Axis columns.
		if hasAnyAxis {
			if ci, ok := axisColIdx["access_class"]; ok && ci < len(rec) {
				row.AccessClass = strings.TrimSpace(rec[ci])
			}
			if ci, ok := axisColIdx["sales_stage"]; ok && ci < len(rec) {
				row.SalesStage = strings.TrimSpace(rec[ci])
			}
			if ci, ok := axisColIdx["entry_window_type"]; ok && ci < len(rec) {
				row.EntryWindowType = strings.TrimSpace(rec[ci])
			}
			if ci, ok := axisColIdx["entry_window_time"]; ok && ci < len(rec) {
				row.EntryWindowTime = strings.TrimSpace(rec[ci])
			}
			if ci, ok := axisColIdx["group_size"]; ok && ci < len(rec) {
				s := strings.TrimSpace(rec[ci])
				if s != "" && s != "0" {
					n, err := strconv.Atoi(s)
					if err != nil {
						return nil, fmt.Errorf("group_size %q for row %q: %w", s, sv, err)
					}
					row.GroupSize = flexInt(n)
				}
			}
			if ci, ok := axisColIdx["comp_flag"]; ok && ci < len(rec) {
				s := strings.ToLower(strings.TrimSpace(rec[ci]))
				row.CompFlag = flexBool(s == "true" || s == "1" || s == "yes")
			}
			row.hasAxes = true
		}

		rows = append(rows, row)
	}
	return rows, nil
}

// parseJSON reads a JSON array of objects. Supports all combinations of
// importRow fields; flex types handle LLM-emitted string-encoded numbers/bools.
func parseJSON(data []byte) ([]importRow, error) {
	// Use a raw decode pass to detect which fields are present so hasAxes can
	// be set correctly without requiring all axis fields to be explicit.
	var rawSlice []map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawSlice); err != nil {
		return nil, fmt.Errorf("parsing JSON import: %w", err)
	}

	rows := make([]importRow, 0, len(rawSlice))
	for _, raw := range rawSlice {
		var r importRow

		decodeStr := func(key string) string {
			if v, ok := raw[key]; ok {
				var s string
				if err := json.Unmarshal(v, &s); err == nil {
					return strings.TrimSpace(s)
				}
			}
			return ""
		}

		r.EntityType = decodeStr("entity_type")
		r.SourceValue = decodeStr("source_value")
		r.CanonicalName = decodeStr("canonical_name")
		r.ExternalID = decodeStr("external_id")

		// Axis fields.
		r.AccessClass = decodeStr("access_class")
		r.SalesStage = decodeStr("sales_stage")
		r.EntryWindowType = decodeStr("entry_window_type")
		r.EntryWindowTime = decodeStr("entry_window_time")

		if v, ok := raw["group_size"]; ok {
			if err := r.GroupSize.UnmarshalJSON(v); err != nil {
				return nil, fmt.Errorf("row %q: %w", r.SourceValue, err)
			}
		}
		if v, ok := raw["comp_flag"]; ok {
			if err := r.CompFlag.UnmarshalJSON(v); err != nil {
				return nil, fmt.Errorf("row %q: %w", r.SourceValue, err)
			}
		}

		// Determine hasAxes: any axis key present in the raw object.
		for _, ac := range axisCols {
			if _, ok := raw[ac]; ok {
				r.hasAxes = true
				break
			}
		}

		if r.SourceValue == "" {
			continue
		}
		rows = append(rows, r)
	}
	return rows, nil
}
