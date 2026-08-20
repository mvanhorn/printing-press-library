// Copyright 2026 Isaac Marks and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored shared helpers for the Clarify novel commands (brief, prep,
// followup, stale, velocity, dossier, dupes). All mirror reads go through
// loadClarifyObjects with drain-first scanning; attribute access is
// key-variant tolerant because Clarify's field names are workspace-schema
// driven.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/clarify/internal/store"

	"github.com/spf13/cobra"
)

// clarifyObj is one JSON:API resource row from the local mirror.
type clarifyObj struct {
	ID    string
	Type  string
	Attrs map[string]any
	Rels  map[string][]string // relationship name -> related record IDs
}

// clarifyMirrorGuard returns (db, true) when the local mirror exists and
// opens; otherwise it prints the sync hint (and empty JSON for machine
// callers) and returns (nil, false). Callers must Close the returned store.
func clarifyMirrorGuard(cmd *cobra.Command, flags *rootFlags, ctx context.Context, dbPath string) (*store.Store, bool, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("clarify-pp-cli")
	}
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: clarify-pp-cli sync --resources resources --path-context object=deal --db %s\n(repeat with object=person, company, meeting, task to mirror every built-in object)\n", dbPath, dbPath)
		if flags.asJSON || flags.agent {
			fmt.Fprintln(cmd.OutOrStdout(), "[]")
		}
		return nil, false, nil
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, false, fmt.Errorf("opening database: %w", err)
	}
	return db, true, nil
}

// loadClarifyObjects returns every mirrored row whose JSON:API type matches
// objType. Rows land in the generic resources table under the sync resource
// name; the object type lives inside the payload.
func loadClarifyObjects(ctx context.Context, db *store.Store, objType string) ([]clarifyObj, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT id, data FROM resources
		WHERE json_extract(data, '$.type') = ?`, objType)
	if err != nil {
		return nil, fmt.Errorf("querying %s rows: %w", objType, err)
	}
	type rawRow struct {
		id   string
		data []byte
	}
	raws := make([]rawRow, 0)
	for rows.Next() {
		var r rawRow
		if err := rows.Scan(&r.id, &r.data); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning %s row: %w", objType, err)
		}
		raws = append(raws, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterating %s rows: %w", objType, err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("closing %s rows: %w", objType, err)
	}

	out := make([]clarifyObj, 0, len(raws))
	for _, r := range raws {
		obj, ok := parseClarifyResource(r.data)
		if !ok {
			continue
		}
		if obj.ID == "" {
			obj.ID = r.id
		}
		out = append(out, obj)
	}
	return out, nil
}

func parseClarifyResource(data []byte) (clarifyObj, bool) {
	var envelope struct {
		Type          string                     `json:"type"`
		ID            string                     `json:"id"`
		Attributes    map[string]any             `json:"attributes"`
		Relationships map[string]json.RawMessage `json:"relationships"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return clarifyObj{}, false
	}
	obj := clarifyObj{ID: envelope.ID, Type: envelope.Type, Attrs: envelope.Attributes, Rels: map[string][]string{}}
	if obj.Attrs == nil {
		obj.Attrs = map[string]any{}
	}
	for name, raw := range envelope.Relationships {
		obj.Rels[name] = relatedIDs(raw)
	}
	return obj, true
}

// relatedIDs extracts record IDs from a JSON:API relationship value, which is
// either {"data": {"id": ...}} or {"data": [{"id": ...}, ...]}.
func relatedIDs(raw json.RawMessage) []string {
	var single struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &single); err == nil && single.Data.ID != "" {
		return []string{single.Data.ID}
	}
	var many struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &many); err == nil && len(many.Data) > 0 {
		ids := make([]string, 0, len(many.Data))
		for _, d := range many.Data {
			if d.ID != "" {
				ids = append(ids, d.ID)
			}
		}
		return ids
	}
	return nil
}

// attrString returns the first non-empty string-ish value among the candidate
// keys. Name-shaped objects ({first_name, last_name}) are joined.
func attrString(attrs map[string]any, keys ...string) string {
	for _, k := range keys {
		v, ok := attrs[k]
		if !ok || v == nil {
			continue
		}
		switch t := v.(type) {
		case string:
			if strings.TrimSpace(t) != "" {
				return strings.TrimSpace(t)
			}
		case map[string]any:
			first, _ := t["first_name"].(string)
			last, _ := t["last_name"].(string)
			joined := strings.TrimSpace(strings.TrimSpace(first) + " " + strings.TrimSpace(last))
			if joined != "" {
				return joined
			}
		case float64:
			return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", t), "0"), ".")
		}
	}
	return ""
}

// attrNumber returns the first numeric value among the candidate keys.
func attrNumber(attrs map[string]any, keys ...string) (float64, bool) {
	for _, k := range keys {
		switch t := attrs[k].(type) {
		case float64:
			return t, true
		case string:
			var f float64
			if _, err := fmt.Sscanf(strings.ReplaceAll(t, ",", ""), "%f", &f); err == nil {
				return f, true
			}
		case map[string]any:
			// currency-style {"amount": 50000, "currency": "USD"}
			if inner, ok := t["amount"].(float64); ok {
				return inner, true
			}
		}
	}
	return 0, false
}

var clarifyTimeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02",
}

// attrTime returns the first parseable timestamp among the candidate keys.
func attrTime(attrs map[string]any, keys ...string) (time.Time, bool) {
	for _, k := range keys {
		s, ok := attrs[k].(string)
		if !ok || s == "" {
			continue
		}
		for _, layout := range clarifyTimeLayouts {
			// Zone-less layouts must parse in local time: a bare due date
			// like "2026-08-17" means the user's day, not UTC midnight.
			if layout == "2006-01-02T15:04:05" || layout == "2006-01-02" {
				if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
					return t, true
				}
				continue
			}
			if t, err := time.Parse(layout, s); err == nil {
				return t, true
			}
		}
	}
	return time.Time{}, false
}

// attrItems returns the string items of a Clarify collection field, which is
// {"items": [...]} on the wire but tolerated here as a bare list too.
func attrItems(attrs map[string]any, keys ...string) []string {
	for _, k := range keys {
		v, ok := attrs[k]
		if !ok || v == nil {
			continue
		}
		var list []any
		switch t := v.(type) {
		case map[string]any:
			if items, ok := t["items"].([]any); ok {
				list = items
			}
		case []any:
			list = t
		}
		out := make([]string, 0, len(list))
		for _, item := range list {
			switch e := item.(type) {
			case string:
				if e != "" {
					out = append(out, e)
				}
			case map[string]any:
				for _, candidate := range []string{"value", "email", "address", "domain", "number"} {
					if s, ok := e[candidate].(string); ok && s != "" {
						out = append(out, s)
						break
					}
				}
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

// Candidate attribute keys shared across the novel commands. Clarify's
// concrete field names are schema-driven per workspace; these cover the
// built-in shapes plus common variants.
var (
	clarifyNameKeys       = []string{"name", "title", "subject", "display_name"}
	clarifyStageKeys      = []string{"stage", "stage_name", "pipeline_stage", "status"}
	clarifyAmountKeys     = []string{"amount", "value", "deal_amount"}
	clarifyUpdatedKeys    = []string{"_updated_at", "updated_at", "modified_at", "last_activity_at"}
	clarifyCreatedKeys    = []string{"_created_at", "created_at"}
	clarifyStartKeys      = []string{"start_time", "start_at", "starts_at", "scheduled_start", "start", "begin_at"}
	clarifyEndKeys        = []string{"end_time", "end_at", "ends_at", "scheduled_end", "end"}
	clarifyDueKeys        = []string{"due_date", "due_at", "due_on", "due"}
	clarifyEmailKeys      = []string{"email_addresses", "emails", "email"}
	clarifyDomainKeys     = []string{"domains", "domain"}
	clarifyCompanyRelKeys = []string{"company_id", "company", "companies"}
	clarifyPeopleRelKeys  = []string{"people", "persons", "attendees", "participants", "person_ids", "contacts"}
	clarifyDealRelKeys    = []string{"deals", "deal_ids", "opportunities"}
)

// relIDsAny returns related IDs for the first relationship name that has any.
func relIDsAny(obj clarifyObj, names []string) []string {
	for _, n := range names {
		if ids := obj.Rels[n]; len(ids) > 0 {
			return ids
		}
	}
	return nil
}

// clarifyStageClosed reports whether a pipeline stage looks terminal.
func clarifyStageClosed(stage string) bool {
	s := strings.ToLower(stage)
	for _, marker := range []string{"won", "lost", "closed", "dead", "churn"} {
		if strings.Contains(s, marker) {
			return true
		}
	}
	return false
}

// normalizeClarifyName lowercases and strips punctuation and legal suffixes
// so "Acme, Inc." and "acme inc" collide for dupe detection.
func normalizeClarifyName(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	replacer := strings.NewReplacer(",", " ", ".", " ", "-", " ", "'", "", "\"", "", "(", " ", ")", " ")
	s = replacer.Replace(s)
	fields := strings.Fields(s)
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		switch f {
		case "inc", "llc", "ltd", "corp", "co", "gmbh", "the":
			continue
		}
		out = append(out, f)
	}
	return strings.Join(out, " ")
}

// indexByID builds an ID lookup for a slice of mirror rows.
func indexByID(objs []clarifyObj) map[string]clarifyObj {
	m := make(map[string]clarifyObj, len(objs))
	for _, o := range objs {
		m[o.ID] = o
	}
	return m
}

// objUpdatedAt returns the best-known update time for a row: payload
// timestamps first, empty time when none parse.
func objUpdatedAt(obj clarifyObj) (time.Time, bool) {
	if t, ok := attrTime(obj.Attrs, clarifyUpdatedKeys...); ok {
		return t, true
	}
	return attrTime(obj.Attrs, clarifyCreatedKeys...)
}

// --- Attribute-based links (verified against live Clarify payloads) ---
// Clarify's resources list endpoints return record links as plain attributes
// (person.company_id, task.deal_id/meeting_id/person_id) and meeting
// attendees as attributes.participants {items:[{name,email,...}]}. The
// JSON:API relationships object is typically empty on list responses, so
// every join helper checks relationships first and falls back to attributes.

// attrLinkIDs returns ID-valued attribute links (string or list of strings).
func attrLinkIDs(obj clarifyObj, keys ...string) []string {
	var out []string
	for _, k := range keys {
		switch t := obj.Attrs[k].(type) {
		case string:
			if t != "" {
				out = append(out, t)
			}
		case []any:
			for _, v := range t {
				if s, ok := v.(string); ok && s != "" {
					out = append(out, s)
				}
			}
		}
	}
	return out
}

// linkedCompanyIDs returns the company records an object points at, via
// relationships or the company_id attribute.
func linkedCompanyIDs(obj clarifyObj) []string {
	if ids := relIDsAny(obj, clarifyCompanyRelKeys); len(ids) > 0 {
		return ids
	}
	return attrLinkIDs(obj, "company_id")
}

// linkedDealIDs returns the deal records an object points at.
func linkedDealIDs(obj clarifyObj) []string {
	if ids := relIDsAny(obj, clarifyDealRelKeys); len(ids) > 0 {
		return ids
	}
	return attrLinkIDs(obj, "deal_id")
}

// meetingParticipant is one attendee row from a meeting's participants
// collection attribute.
type meetingParticipant struct {
	Name  string
	Email string
}

// meetingParticipants parses attributes.participants / attendees items.
func meetingParticipants(obj clarifyObj) []meetingParticipant {
	for _, key := range []string{"participants", "attendees"} {
		raw, ok := obj.Attrs[key]
		if !ok || raw == nil {
			continue
		}
		var items []any
		switch t := raw.(type) {
		case map[string]any:
			if list, ok := t["items"].([]any); ok {
				items = list
			}
		case []any:
			items = t
		}
		out := make([]meetingParticipant, 0, len(items))
		for _, item := range items {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			p := meetingParticipant{}
			p.Name, _ = entry["name"].(string)
			p.Email, _ = entry["email"].(string)
			if p.Name != "" || p.Email != "" {
				out = append(out, p)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

// emailIndex maps every lowercased email address to the person records that
// carry it. Auto-captured CRMs hold duplicate people per email, so the index
// keeps all of them — join helpers walk every match rather than letting the
// last-parsed duplicate shadow the record with the real company link.
func emailIndex(people []clarifyObj) map[string][]clarifyObj {
	m := map[string][]clarifyObj{}
	for _, p := range people {
		for _, e := range attrItems(p.Attrs, clarifyEmailKeys...) {
			key := strings.ToLower(strings.TrimSpace(e))
			m[key] = append(m[key], p)
		}
	}
	return m
}

// meetingCompanyIDs resolves the companies behind a meeting: explicit
// relationship/attribute links first, then participants' person records.
func meetingCompanyIDs(m clarifyObj, byEmail map[string][]clarifyObj, personByID map[string]clarifyObj) []string {
	ids := linkedCompanyIDs(m)
	if len(ids) > 0 {
		return ids
	}
	seen := map[string]bool{}
	for _, pid := range relIDsAny(m, clarifyPeopleRelKeys) {
		if p, ok := personByID[pid]; ok {
			for _, cid := range linkedCompanyIDs(p) {
				if !seen[cid] {
					seen[cid] = true
					ids = append(ids, cid)
				}
			}
		}
	}
	for _, part := range meetingParticipants(m) {
		if part.Email == "" {
			continue
		}
		for _, p := range byEmail[strings.ToLower(part.Email)] {
			for _, cid := range linkedCompanyIDs(p) {
				if !seen[cid] {
					seen[cid] = true
					ids = append(ids, cid)
				}
			}
		}
	}
	return ids
}

// taskDone reports whether a task status string is terminal.
func taskDone(status string) bool {
	s := strings.ToLower(strings.TrimSpace(status))
	switch s {
	case "done", "completed", "complete", "canceled", "cancelled":
		return true
	}
	return clarifyStageClosed(s)
}
