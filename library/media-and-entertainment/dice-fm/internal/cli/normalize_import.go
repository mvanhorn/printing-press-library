// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
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
	"sort"
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
// method="manual" crosswalk row. Returns the number of rows imported.
//
// The entityType parameter selects the import schema:
//   - "ticket_type": the fixed tier-axis schema. Rows carrying an embedded
//     entity_type column are honored (so a single file may also mix in venue or
//     other canonical/crosswalk rows). When tier-axis columns are present, a
//     tier_attributes row is written via UpsertTierAttributes. This is the
//     historical, backward-compatible path; its wire schema is unchanged.
//   - any other entity type: the GENERIC schema. The first column is
//     source_value; every other column is an attribute key. Each non-empty
//     attribute is written to entity_attributes via UpsertEntityAttribute with
//     method="manual" so the row survives re-classification, unlocking
//     LLM-tail value classification for entities with no typed attribute table.
func importMapping(s *store.Store, sourceSystem, entityType string, data []byte, format string) (int, error) {
	if entityType == "" {
		entityType = "ticket_type"
	}
	if entityType != "ticket_type" {
		return importGenericMapping(s, sourceSystem, entityType, data, format)
	}
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
			Method:            methodManual,
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
				Method:            methodManual,
			}); err != nil {
				return 0, fmt.Errorf("upsert tier attributes for %q: %w", r.SourceValue, err)
			}
		}
	}
	return len(rows), nil
}

// genericImportRow is one parsed entry from a generic-schema import: a source
// value plus an ordered set of attribute key/value pairs. Empty attribute
// values are dropped at parse time so they are never written.
type genericImportRow struct {
	SourceValue string
	Attrs       []genericAttr
}

// genericAttr is a single attribute column for a generic-schema import row.
type genericAttr struct {
	Key   string
	Value string
}

// importGenericMapping parses a generic-schema import (any entity type other
// than ticket_type) and writes, per row: a canonical entity, a method="manual"
// crosswalk row, and a method="manual" entity_attributes row for each non-empty
// attribute column. Returns the number of source rows imported.
func importGenericMapping(s *store.Store, sourceSystem, entityType string, data []byte, format string) (int, error) {
	rows, err := parseGenericImportData(data, format)
	if err != nil {
		return 0, err
	}
	for _, r := range rows {
		canon := canonicalizeName(r.SourceValue)
		cid := mintCanonicalID(entityType, canon)

		if err := s.UpsertCanonicalEntity(entityType, cid, canon); err != nil {
			return 0, fmt.Errorf("upsert canonical entity for %q: %w", r.SourceValue, err)
		}
		if err := s.UpsertCrosswalk(store.CrosswalkRow{
			EntityType:        entityType,
			SourceSystem:      sourceSystem,
			SourceValue:       r.SourceValue,
			CanonicalID:       cid,
			Method:            methodManual,
			ClassifierVersion: currentClassifierVersion,
		}); err != nil {
			return 0, fmt.Errorf("upsert crosswalk for %q: %w", r.SourceValue, err)
		}
		for _, a := range r.Attrs {
			if err := s.UpsertEntityAttribute(cid, entityType, a.Key, a.Value, methodManual, currentClassifierVersion); err != nil {
				return 0, fmt.Errorf("upsert entity attribute %q for %q: %w", a.Key, r.SourceValue, err)
			}
		}
	}
	return len(rows), nil
}

// parseGenericImportData parses the raw bytes as a generic-schema CSV or JSON.
func parseGenericImportData(data []byte, format string) ([]genericImportRow, error) {
	switch strings.ToLower(format) {
	case "csv":
		return parseGenericCSV(data)
	case "json":
		return parseGenericJSON(data)
	default:
		return nil, fmt.Errorf("unsupported import format %q: must be csv or json", format)
	}
}

// parseGenericCSV reads a header-row CSV where the first column is source_value
// and every other column is an attribute key. Rows with an empty source_value
// are skipped; empty attribute cells are dropped (not written as empty).
func parseGenericCSV(data []byte) ([]genericImportRow, error) {
	r := csv.NewReader(bytes.NewReader(data))
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parsing CSV: %w", err)
	}
	if len(records) < 1 {
		return nil, fmt.Errorf("CSV is empty")
	}

	header := records[0]
	svCol := -1
	attrCols := make([]struct {
		idx int
		key string
	}, 0, len(header))
	for i, h := range header {
		name := strings.TrimSpace(strings.ToLower(h))
		if name == "source_value" {
			svCol = i
			continue
		}
		if name == "" {
			continue
		}
		attrCols = append(attrCols, struct {
			idx int
			key string
		}{idx: i, key: name})
	}
	if svCol < 0 {
		return nil, fmt.Errorf("CSV missing required column %q", "source_value")
	}

	rows := make([]genericImportRow, 0, len(records)-1)
	for _, rec := range records[1:] {
		if len(rec) == 0 || svCol >= len(rec) {
			continue
		}
		sv := strings.TrimSpace(rec[svCol])
		if sv == "" {
			continue
		}
		row := genericImportRow{SourceValue: sv}
		for _, ac := range attrCols {
			if ac.idx >= len(rec) {
				continue
			}
			v := strings.TrimSpace(rec[ac.idx])
			if v == "" {
				continue
			}
			row.Attrs = append(row.Attrs, genericAttr{Key: ac.key, Value: v})
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// parseGenericJSON reads a JSON array of objects. Each object must carry a
// source_value key; every other key is an attribute. Non-string attribute
// values are stringified; empty attribute values are dropped. Attributes are
// emitted in sorted-key order so writes are deterministic.
func parseGenericJSON(data []byte) ([]genericImportRow, error) {
	var rawSlice []map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawSlice); err != nil {
		return nil, fmt.Errorf("parsing JSON import: %w", err)
	}

	rows := make([]genericImportRow, 0, len(rawSlice))
	for _, raw := range rawSlice {
		sv := ""
		if v, ok := raw["source_value"]; ok {
			sv = strings.TrimSpace(jsonScalarString(v))
		}
		if sv == "" {
			continue
		}
		row := genericImportRow{SourceValue: sv}

		keys := make([]string, 0, len(raw))
		for k := range raw {
			if k == "source_value" {
				continue
			}
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := strings.TrimSpace(jsonScalarString(raw[k]))
			if v == "" {
				continue
			}
			row.Attrs = append(row.Attrs, genericAttr{Key: k, Value: v})
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// jsonScalarString renders a JSON scalar (string, number, or bool) as a string.
// A JSON string is unquoted; everything else is returned as its raw token. This
// lets the generic importer accept e.g. {"group_size": 3} or {"comp_flag": true}
// alongside string-encoded values without a per-attribute schema.
func jsonScalarString(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return strings.TrimSpace(string(raw))
}

// axisCols is the ordered list of tier-axis column names for column-index lookup.
var axisCols = []string{
	axisAccessClass,
	axisSalesStage,
	axisEntryWindowType,
	axisEntryWindowTime,
	axisGroupSize,
	axisCompFlag,
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
			if ci, ok := axisColIdx[axisAccessClass]; ok && ci < len(rec) {
				row.AccessClass = strings.TrimSpace(rec[ci])
			}
			if ci, ok := axisColIdx[axisSalesStage]; ok && ci < len(rec) {
				row.SalesStage = strings.TrimSpace(rec[ci])
			}
			if ci, ok := axisColIdx[axisEntryWindowType]; ok && ci < len(rec) {
				row.EntryWindowType = strings.TrimSpace(rec[ci])
			}
			if ci, ok := axisColIdx[axisEntryWindowTime]; ok && ci < len(rec) {
				row.EntryWindowTime = strings.TrimSpace(rec[ci])
			}
			if ci, ok := axisColIdx[axisGroupSize]; ok && ci < len(rec) {
				s := strings.TrimSpace(rec[ci])
				if s != "" && s != "0" {
					n, err := strconv.Atoi(s)
					if err != nil {
						return nil, fmt.Errorf("group_size %q for row %q: %w", s, sv, err)
					}
					row.GroupSize = flexInt(n)
				}
			}
			if ci, ok := axisColIdx[axisCompFlag]; ok && ci < len(rec) {
				s := strings.ToLower(strings.TrimSpace(rec[ci]))
				row.CompFlag = flexBool(s == "true" || s == "1" || s == "yes")
			}
			// hasAxes is true only when at least one axis field carries a meaningful
			// value; a row with all-empty axis values must not write tier_attributes.
			row.hasAxes = row.AccessClass != "" || row.SalesStage != "" ||
				row.EntryWindowType != "" || row.EntryWindowTime != "" ||
				row.GroupSize > 0 || bool(row.CompFlag)
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

		// hasAxes is true only when at least one axis field carries a meaningful
		// value; a row where every axis field is empty/zero must not write
		// tier_attributes.
		r.hasAxes = r.AccessClass != "" || r.SalesStage != "" ||
			r.EntryWindowType != "" || r.EntryWindowTime != "" ||
			r.GroupSize > 0 || bool(r.CompFlag)

		if r.SourceValue == "" {
			continue
		}
		rows = append(rows, r)
	}
	return rows, nil
}
