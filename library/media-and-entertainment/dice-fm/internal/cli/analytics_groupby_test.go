// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Test for analytics --group-by dotted-path support (review nit #15): a dotted
// field like ticketType.name must resolve the nested value, not bucket all rows
// under <nil>. Synthetic fixtures only.
package cli

import "testing"

func TestResolveDottedFieldTopLevel(t *testing.T) {
	obj := map[string]any{"state": "on_sale", "id": "e1"}
	if got := resolveDottedField(obj, "state"); got != "on_sale" {
		t.Errorf("top-level resolve = %v, want on_sale", got)
	}
}

func TestResolveDottedFieldNested(t *testing.T) {
	obj := map[string]any{
		"id": "tk1",
		"ticketType": map[string]any{
			"name":  "General Admission",
			"price": float64(2500),
		},
	}
	if got := resolveDottedField(obj, "ticketType.name"); got != "General Admission" {
		t.Errorf("nested resolve = %v, want 'General Admission'", got)
	}
	if got := resolveDottedField(obj, "ticketType.price"); got != float64(2500) {
		t.Errorf("nested numeric resolve = %v, want 2500", got)
	}
}

func TestResolveDottedFieldMissing(t *testing.T) {
	obj := map[string]any{"id": "x"}
	if got := resolveDottedField(obj, "ticketType.name"); got != nil {
		t.Errorf("missing nested path = %v, want nil", got)
	}
	if got := resolveDottedField(obj, "absent"); got != nil {
		t.Errorf("missing top-level = %v, want nil", got)
	}
}
