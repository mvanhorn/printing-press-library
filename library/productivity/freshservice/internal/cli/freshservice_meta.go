// Copyright 2026 mark-van-de-ven. Licensed under Apache-2.0. See LICENSE.
//
// PATCH(freshservice-novel-commands): tenant-specific code→label resolver
// (Status/Priority/Type/Source/Urgency/Impact) backed by the synced
// ticket-form-fields choices; each Freshservice tenant defines its own
// integer codes, so the generator's static enum-to-label map is wrong.

package cli

import (
	"encoding/json"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/productivity/freshservice/internal/store"
)

// TenantMeta caches the per-tenant code→label maps that Freshservice's
// ticket_form_fields endpoint exposes. The CLI keeps the cache load lazy —
// a single Query against the synced `ticket-form-fields` corpus — so novel
// features can decorate codes (status, priority, type, source, urgency,
// impact) with friendly labels when the user has run sync, and fall back to
// the bare integer string when they haven't.
type TenantMeta struct {
	Status    map[int]string
	Priority  map[int]string
	Type      map[int]string
	Source    map[int]string
	Urgency   map[int]string
	Impact    map[int]string
	Loaded    bool
}

// LoadTenantMeta reads ticket-form-fields out of the local store and
// constructs the per-field code maps. Returns an empty (Loaded=false) value
// when the store is missing or the ticket-form-fields resource has not been
// synced yet — callers fall back to the integer-string defaults in that case.
//
// Each ticket field document in Freshservice has a `name` (e.g. "status")
// and a `choices` array whose elements are `{id: int, value: string, ...}`.
// Older or non-typed-list shapes are tolerated by sniffing the JSON.
func LoadTenantMeta(db *store.Store) (*TenantMeta, error) {
	meta := &TenantMeta{}
	if db == nil {
		return meta, nil
	}
	rows, err := db.Query(`SELECT data FROM resources WHERE resource_type = ?`, "ticket-form-fields")
	if err != nil {
		return meta, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		var field struct {
			Name    string            `json:"name"`
			Choices json.RawMessage   `json:"choices"`
		}
		if err := json.Unmarshal(raw, &field); err != nil {
			continue
		}
		target := meta.targetFor(field.Name)
		if target == nil {
			continue
		}
		decodeChoices(field.Choices, target)
		if len(*target) > 0 {
			meta.Loaded = true
		}
	}
	return meta, rows.Err()
}

// targetFor returns the map a given ticket-field name should populate, or
// nil when the field isn't one we surface in novel-feature output. The
// recognised field names match the canonical Freshservice ticket schema.
func (m *TenantMeta) targetFor(name string) *map[int]string {
	switch name {
	case "status":
		if m.Status == nil {
			m.Status = map[int]string{}
		}
		return &m.Status
	case "priority":
		if m.Priority == nil {
			m.Priority = map[int]string{}
		}
		return &m.Priority
	case "ticket_type", "type":
		if m.Type == nil {
			m.Type = map[int]string{}
		}
		return &m.Type
	case "source":
		if m.Source == nil {
			m.Source = map[int]string{}
		}
		return &m.Source
	case "urgency":
		if m.Urgency == nil {
			m.Urgency = map[int]string{}
		}
		return &m.Urgency
	case "impact":
		if m.Impact == nil {
			m.Impact = map[int]string{}
		}
		return &m.Impact
	}
	return nil
}

// decodeChoices fills a code→label map from a Freshservice `choices` JSON
// blob. Modern endpoints return a list of `{id, value, ...}` objects;
// some older endpoints return a `{"value": id}` dict. Both are decoded.
func decodeChoices(raw json.RawMessage, out *map[int]string) {
	if len(raw) == 0 {
		return
	}
	// List form: [{id: 2, value: "Open", ...}, ...]
	var list []struct {
		ID    json.Number `json:"id"`
		Value string      `json:"value"`
	}
	if err := json.Unmarshal(raw, &list); err == nil && len(list) > 0 {
		for _, c := range list {
			if id, err := c.ID.Int64(); err == nil && c.Value != "" {
				(*out)[int(id)] = c.Value
			}
		}
		return
	}
	// Dict form: {"Open": 2, "Pending": 3, ...}
	var dict map[string]json.Number
	if err := json.Unmarshal(raw, &dict); err == nil {
		for label, n := range dict {
			if id, err := n.Int64(); err == nil {
				(*out)[int(id)] = label
			}
		}
	}
}

// labelOrCode returns the tenant-specific label for a code, or the bare
// integer string when no label is known. Used by novel features so JSON
// output emits `status_code: 9, status_label: "BI Melding"` when sync has
// run, and `status: "9"` when it hasn't.
func (m *TenantMeta) labelOrCode(table map[int]string, code int) string {
	if label, ok := table[code]; ok {
		return label
	}
	return strconv.Itoa(code)
}

// StatusLabel returns the tenant-specific status name, with the canonical
// built-in fallback for codes 2/3/4/5 when no tenant override is present.
func (m *TenantMeta) StatusLabel(code int) string {
	if m != nil && m.Status != nil {
		if label, ok := m.Status[code]; ok {
			return label
		}
	}
	switch code {
	case 2:
		return "Open"
	case 3:
		return "Pending"
	case 4:
		return "Resolved"
	case 5:
		return "Closed"
	}
	return strconv.Itoa(code)
}

// PriorityLabel applies the same lookup pattern to priority codes.
func (m *TenantMeta) PriorityLabel(code int) string {
	if m != nil && m.Priority != nil {
		if label, ok := m.Priority[code]; ok {
			return label
		}
	}
	switch code {
	case 1:
		return "Low"
	case 2:
		return "Medium"
	case 3:
		return "High"
	case 4:
		return "Urgent"
	}
	return strconv.Itoa(code)
}
