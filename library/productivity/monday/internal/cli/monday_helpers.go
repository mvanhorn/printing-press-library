// Shared helpers for monday-pp-cli's hand-written GraphQL surface.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
)

// pluckJSONField returns data[key] as RawMessage, returning the original data
// when key is empty. If key is not present, returns null. Used after GraphQL
// returns to expose the inner field as the command's JSON output.
func pluckJSONField(data json.RawMessage, key string) (json.RawMessage, error) {
	if key == "" {
		return data, nil
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("parsing JSON envelope: %w", err)
	}
	if v, ok := envelope[key]; ok {
		return v, nil
	}
	return json.RawMessage("null"), nil
}

// pluckFirstFromArrayField returns the first element of the JSON array at
// data[key]. Convenience for queries like `boards(ids:[$id])` which always
// return a single-element array.
func pluckFirstFromArrayField(data json.RawMessage, key string) (json.RawMessage, error) {
	raw, err := pluckJSONField(data, key)
	if err != nil {
		return nil, err
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		// Maybe data[key] was already a single object — return as-is.
		return raw, nil
	}
	if len(arr) == 0 {
		return json.RawMessage("null"), nil
	}
	return arr[0], nil
}

// splitCSV cleans a comma-separated flag value: trims whitespace, drops empty
// entries, returns nil for empty input. Used by every filter flag that takes
// "id1,id2,id3".
func splitCSV(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// gqlListIDs renders a comma-separated string of unquoted ids for inlining
// into a GraphQL query. monday.com accepts either ID! or [ID!]; we send
// integers when possible (the standard form for board/item/user IDs) and
// strings otherwise. The result is meant to be interpolated into a query
// segment like `boards(ids: [<rendered>])`.
func gqlListIDs(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	return strings.Join(ids, ",")
}

// pluckFirstFromArrayObject returns the value at key from the first element
// of a JSON-encoded array. Used to reach into nested-list responses like
// `boards[0].activity_logs` or `boards[0].items_page.items`.
func pluckFirstFromArrayObject(arr json.RawMessage, key string) (json.RawMessage, error) {
	if len(arr) == 0 || string(arr) == "null" {
		return json.RawMessage("null"), nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(arr, &items); err != nil {
		// Maybe it's already a single object.
		var obj map[string]json.RawMessage
		if err2 := json.Unmarshal(arr, &obj); err2 == nil {
			if v, ok := obj[key]; ok {
				return v, nil
			}
		}
		return nil, fmt.Errorf("parsing array: %w", err)
	}
	if len(items) == 0 {
		return json.RawMessage("[]"), nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(items[0], &obj); err != nil {
		return nil, fmt.Errorf("parsing first element: %w", err)
	}
	if v, ok := obj[key]; ok {
		return v, nil
	}
	return json.RawMessage("null"), nil
}
