package cli

import "encoding/json"

// envelopeItemCount counts the items in the named arrays of an object-shaped answer, for the interactive provenance line.
// Hand-written (board #95 parity r2): the duplicate proposals answers are objects ({ rows, meta } for an import,
// { proposals, heldIdentifiers, ... } for a contact), so the generated top-level-array count printed "0 results (live)"
// for a non-empty answer. A reprint must keep this. Also looks one level down in a { data } wrapper.
func envelopeItemCount(data json.RawMessage, keys ...string) int {
	var obj map[string]json.RawMessage
	if json.Unmarshal(data, &obj) != nil {
		return 0
	}
	if inner, ok := obj["data"]; ok {
		var nested map[string]json.RawMessage
		if json.Unmarshal(inner, &nested) == nil {
			obj = nested
		}
	}
	n := 0
	for _, k := range keys {
		var items []json.RawMessage
		if json.Unmarshal(obj[k], &items) == nil {
			n += len(items)
		}
	}
	return n
}
