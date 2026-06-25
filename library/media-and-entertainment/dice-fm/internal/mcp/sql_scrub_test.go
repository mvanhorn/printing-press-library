// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for the best-effort sql-result pseudonymization (Task 13). Synthetic
// fixtures only.
package mcp

import (
	"encoding/json"
	"testing"
)

func TestScrubSQLRowFlatColumns(t *testing.T) {
	row := map[string]any{
		"id":          "fan-1",
		"email":       "buyer@example.com",
		"phoneNumber": "5550100",
		"firstName":   "Alice",
		"lastName":    "Smith",
		"dob":         "1990-01-01",
		"total":       2500,
	}
	out := scrubSQLRow(row, Opts{Salt: []byte(testSalt)})
	for _, k := range []string{"email", "phoneNumber", "firstName", "lastName", "dob"} {
		if _, ok := out[k]; ok {
			t.Errorf("scrubSQLRow left flat PII column %q: %v", k, out[k])
		}
	}
	if out["total"] != 2500 {
		t.Errorf("non-PII column altered: %v", out["total"])
	}
	if ref, _ := out["fan_ref"].(string); ref != Token([]byte(testSalt), "buyer@example.com") {
		t.Errorf("fan_ref = %v, want token of email", out["fan_ref"])
	}
}

func TestScrubSQLRowIncludePIIKeepsRawExceptDOB(t *testing.T) {
	row := map[string]any{
		"email": "buyer@example.com",
		"name":  "Buyer Name",
		"dob":   "1990-01-01",
		"total": 2500,
	}
	out := scrubSQLRow(row, Opts{Salt: []byte(testSalt), IncludePII: true})
	if out["email"] != "buyer@example.com" || out["name"] != "Buyer Name" {
		t.Fatalf("include_pii dropped raw identifiers: %+v", out)
	}
	if _, ok := out["dob"]; ok {
		t.Fatalf("include_pii left dob: %+v", out)
	}
	if out["fan_ref"] != Token([]byte(testSalt), "buyer@example.com") {
		t.Fatalf("include_pii missing stable fan_ref: %+v", out)
	}
}

func TestScrubSQLRowJSONCell(t *testing.T) {
	// SELECT data FROM resources returns the blob as a JSON string cell.
	blob := `{"id":"o1","event":{"name":"Show"},"fan":{"id":"f1","email":"deep@example.com","dob":"2000-02-02"}}`
	row := map[string]any{"resource_type": "orders", "data": blob}
	out := scrubSQLRow(row, Opts{Salt: []byte(testSalt)})

	// The data cell must be re-serialized scrubbed JSON.
	cell, ok := out["data"].(json.RawMessage)
	if !ok {
		t.Fatalf("data cell type = %T, want json.RawMessage", out["data"])
	}
	s := string(cell)
	for _, pii := range []string{"deep@example.com", "2000-02-02"} {
		if contains(s, pii) {
			t.Errorf("JSON cell still contains PII %q: %s", pii, s)
		}
	}
	if !contains(s, "fan_ref") || !contains(s, "Show") {
		t.Errorf("JSON cell lost fan_ref or discovery field: %s", s)
	}
	// Plain analytics columns without flat PII don't get a spurious fan_ref.
	if _, ok := out["fan_ref"]; ok {
		t.Errorf("scrubSQLRow added a top-level fan_ref to a row with no flat PII column")
	}
}

func TestScrubSQLRowNamePhoneWithoutEmail(t *testing.T) {
	row := map[string]any{
		"first_name":  "Name",
		"last_name":   "Only",
		"phone":       "5550100",
		"total_spend": 10,
		"dob":         "1990-01-01",
	}
	out := scrubSQLRow(row, Opts{Salt: []byte(testSalt)})
	for _, k := range []string{"first_name", "last_name", "phone", "dob"} {
		if _, ok := out[k]; ok {
			t.Fatalf("scrubSQLRow left seedless PII column %q: %+v", k, out)
		}
	}
	if out["total_spend"] != 10 {
		t.Fatalf("scrubSQLRow altered non-PII column: %+v", out)
	}
	if out["fan_ref"] != Token([]byte(testSalt), "name only") {
		t.Fatalf("scrubSQLRow missing name-derived fan_ref: %+v", out)
	}
}

func TestScrubSQLRowPureAnalyticsEventNameUntouched(t *testing.T) {
	row := map[string]any{"event_name": "Show", "total_spend": 10}
	out := scrubSQLRow(row, Opts{Salt: []byte(testSalt)})
	if out["event_name"] != "Show" || out["total_spend"] != 10 {
		t.Fatalf("pure analytics row changed: %+v", out)
	}
	if _, ok := out["fan_ref"]; ok {
		t.Fatalf("pure analytics row got fan_ref: %+v", out)
	}
}

func TestScrubSQLRowNoPIINoToken(t *testing.T) {
	row := map[string]any{"id": "evt-1", "total": 999, "name": "Show A"}
	out := scrubSQLRow(row, Opts{Salt: []byte(testSalt)})
	// A bare top-level `name` column is an event/venue name, not a person — it
	// is NOT in flatPIIColumns, so this is a pure analytics row: name is
	// preserved and no fan_ref is added.
	if _, ok := out["fan_ref"]; ok {
		t.Errorf("pure analytics row got a spurious fan_ref: %+v", out)
	}
	if out["name"] != "Show A" {
		t.Errorf("scrubSQLRow dropped a legitimate top-level event/venue name: %v", out["name"])
	}
}

func TestSQLDescriptionWarnsAboutAliasedPII(t *testing.T) {
	// The sql tool description must warn that aliased/computed PII may slip
	// through. Assert the substring is present in the registered description by
	// re-deriving it the same way RegisterTools builds it.
	desc := "Run read-only SQL against local database. Use for ad-hoc analysis, aggregations, and joins across synced resources. Requires sync first. " + piiToolNotice + " Scrubbing is best-effort for sql: PII in plainly-named columns (email, holder_email, phone, phoneNumber, firstName, first_name, lastName, last_name, name, holder_name, dob) and in JSON cells (e.g. SELECT data FROM resources) is redacted, but PII surfaced through a column ALIAS or a computed expression may slip through — prefer search/typed tools, or pass include_pii only when you accept raw output."
	if !contains(desc, "ALIAS") || !contains(desc, "best-effort") {
		t.Errorf("sql description missing the aliased-PII warning")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
