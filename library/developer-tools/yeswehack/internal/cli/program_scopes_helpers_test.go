package cli

import (
	"encoding/json"
	"testing"
)

func TestAnnotateProgramScopesAddsStableIDAndProgramSlug(t *testing.T) {
	raw := json.RawMessage(`{"items":[{"asset_value":"https://example.test","scope_type":"web-application","scope_type_name":"Web Application"}]}`)

	annotated := annotateProgramScopes(raw, "program-one")

	var envelope struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(annotated, &envelope); err != nil {
		t.Fatalf("unmarshal annotated scopes: %v", err)
	}
	if len(envelope.Items) != 1 {
		t.Fatalf("item count = %d, want 1", len(envelope.Items))
	}
	if got := stringField(envelope.Items[0], "program_slug"); got != "program-one" {
		t.Fatalf("program_slug = %q, want program-one", got)
	}
	if got := stringField(envelope.Items[0], "id"); got == "" {
		t.Fatalf("id was not populated")
	}
}

func TestFilterProgramScopes(t *testing.T) {
	raw := json.RawMessage(`[
		{"id":"a","program_slug":"program-one","asset_value":"a.test"},
		{"id":"b","program_slug":"program-two","asset_value":"b.test"}
	]`)

	filtered := filterProgramScopes(raw, "program-two")

	var rows []map[string]any
	if err := json.Unmarshal(filtered, &rows); err != nil {
		t.Fatalf("unmarshal filtered scopes: %v", err)
	}
	if len(rows) != 1 || stringField(rows[0], "id") != "b" {
		t.Fatalf("filtered rows = %#v, want only id b", rows)
	}
}
