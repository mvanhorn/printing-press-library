// Copyright 2026 Derick Ng and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored unit tests for the financial report + JSON:API write helpers.

package cli

import (
	"encoding/json"
	"testing"
)

func TestRelID(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"to-one", `{"data":{"id":"42","type":"deals"}}`, "42"},
		{"null data", `{"data":null}`, ""},
		{"empty", ``, ""},
		{"to-many array", `{"data":[{"id":"1"}]}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := relID(json.RawMessage(tc.in)); got != tc.want {
				t.Fatalf("relID(%s) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestRowMoneyCents(t *testing.T) {
	attrs := map[string]json.RawMessage{
		"total_recognized_revenue": json.RawMessage(`500000`),
		"total_invoiced":           json.RawMessage(`0`),
		"as_float":                 json.RawMessage(`1234.0`),
		"missing_is_zero":          json.RawMessage(`null`),
	}
	if got := rowMoneyCents(attrs, "total_recognized_revenue"); got != 500000 {
		t.Fatalf("recognized = %d, want 500000", got)
	}
	if got := rowMoneyCents(attrs, "total_invoiced"); got != 0 {
		t.Fatalf("invoiced = %d, want 0", got)
	}
	if got := rowMoneyCents(attrs, "as_float"); got != 1234 {
		t.Fatalf("as_float = %d, want 1234", got)
	}
	if got := rowMoneyCents(attrs, "absent"); got != 0 {
		t.Fatalf("absent = %d, want 0", got)
	}
}

func TestMetaInt(t *testing.T) {
	meta := map[string]json.RawMessage{"total_pages": json.RawMessage(`3`), "junk": json.RawMessage(`"x"`)}
	if n, ok := metaInt(meta, "total_pages"); !ok || n != 3 {
		t.Fatalf("total_pages = %d ok=%v, want 3 true", n, ok)
	}
	if _, ok := metaInt(meta, "missing"); ok {
		t.Fatalf("missing should not be ok")
	}
	if _, ok := metaInt(meta, "junk"); ok {
		t.Fatalf("non-numeric should not be ok")
	}
}

func TestFlattenReportRow(t *testing.T) {
	row := japiResource{
		ID:   "r1",
		Type: "financial_item_reports",
		Attributes: map[string]json.RawMessage{
			"date":                     json.RawMessage(`"2026-01-01"`),
			"total_recognized_revenue": json.RawMessage(`250000`),
		},
		Relationships: map[string]json.RawMessage{
			"budget": json.RawMessage(`{"data":{"id":"9","type":"deals"}}`),
		},
	}
	included := map[string]japiResource{
		"deals:9": {ID: "9", Type: "deals", Attributes: map[string]json.RawMessage{"name": json.RawMessage(`"Acme Retainer"`)}},
	}
	out := flattenReportRow(row, included)
	if out["budget_id"] != "9" {
		t.Fatalf("budget_id = %v, want 9", out["budget_id"])
	}
	if out["budget_name"] != "Acme Retainer" {
		t.Fatalf("budget_name = %v, want Acme Retainer", out["budget_name"])
	}
	if out["date"] != "2026-01-01" {
		t.Fatalf("date = %v, want 2026-01-01", out["date"])
	}
}

func TestBuildJSONAPIBody(t *testing.T) {
	// From --set / --set-json / --rel.
	body, err := buildJSONAPIBody("tasks", "", []string{"title=Hi"}, []string{"is_private=true"}, []string{"project=projects:5"}, "")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	data := body["data"].(map[string]any)
	if data["type"] != "tasks" {
		t.Fatalf("type = %v", data["type"])
	}
	attrs := data["attributes"].(map[string]any)
	if attrs["title"] != "Hi" {
		t.Fatalf("title = %v", attrs["title"])
	}
	if attrs["is_private"] != true {
		t.Fatalf("is_private = %v (want bool true)", attrs["is_private"])
	}
	rel := data["relationships"].(map[string]any)["project"].(map[string]any)["data"].(map[string]any)
	if rel["type"] != "projects" || rel["id"] != "5" {
		t.Fatalf("relationship = %v", rel)
	}

	// Update injects id.
	body, err = buildJSONAPIBody("deals", "123", []string{"probability=75"}, nil, nil, "")
	if err != nil {
		t.Fatalf("build update: %v", err)
	}
	if body["data"].(map[string]any)["id"] != "123" {
		t.Fatalf("update id not injected: %v", body["data"])
	}

	// Raw --data with existing "data" envelope passes through.
	body, err = buildJSONAPIBody("tasks", "", nil, nil, nil, `{"data":{"type":"tasks","attributes":{"title":"Z"}}}`)
	if err != nil {
		t.Fatalf("build raw: %v", err)
	}
	if body["data"].(map[string]any)["attributes"].(map[string]any)["title"] != "Z" {
		t.Fatalf("raw passthrough failed: %v", body)
	}

	// No fields at all is an error.
	if _, err := buildJSONAPIBody("tasks", "", nil, nil, nil, ""); err == nil {
		t.Fatalf("expected error when no fields provided")
	}
}
