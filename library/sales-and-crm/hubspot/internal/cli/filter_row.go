// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

// Helper that projects HubSpot's `data` blob into the flat map[string]string
// shape cliutil.FilterExpr.Match expects.
//
// Used by the in-memory --filter path in stale / nurture-queue / meetings
// ever-had so each command doesn't re-implement the same json_extract walk.

package cli

import (
	"encoding/json"
	"fmt"
)

// extractPropertiesRow reads the requested fields out of a HubSpot CRM object
// JSON blob (the SQLite `data` column) and returns them as the flat row map
// cliutil.FilterExpr.Match consumes.
//
// rawJSON is the full object JSON (e.g. `{"id":"123","properties":{...}}`);
// fields is the list of property names the filter references (call
// expr.FieldsReferenced() for this). Missing properties map to empty string
// — which makes NOT_HAS naturally match unset fields.
//
// Values that aren't strings (numbers, booleans, null) are stringified via
// fmt.Sprintf("%v", v); HubSpot's API typically returns properties as
// strings already, but the local store occasionally normalizes amounts and
// counts to numeric JSON, so this handles both shapes.
func extractPropertiesRow(rawJSON string, fields []string) map[string]string {
	out := make(map[string]string, len(fields))
	if rawJSON == "" || len(fields) == 0 {
		return out
	}
	var envelope struct {
		Properties map[string]any `json:"properties"`
	}
	if err := json.Unmarshal([]byte(rawJSON), &envelope); err != nil {
		return out
	}
	for _, f := range fields {
		v, ok := envelope.Properties[f]
		if !ok || v == nil {
			continue
		}
		switch x := v.(type) {
		case string:
			out[f] = x
		default:
			out[f] = fmt.Sprintf("%v", x)
		}
	}
	return out
}
