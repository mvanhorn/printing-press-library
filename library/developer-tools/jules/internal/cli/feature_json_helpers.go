// Shared decode helpers for the hand-coded novel feature commands
// (feature_archival.go, feature_continuous_monitoring.go,
// feature_diff_validation.go, feature_checkpoint.go).
//
// The Jules API documents list endpoints as an envelope object
// (e.g. {"sessions": [...]}), but some backends -- including generic
// test/mock harnesses -- return the bare JSON array instead. Decoding
// straight into map[string]any breaks hard on that shape ("json: cannot
// unmarshal array into Go value of type map[string]interface {}"), which
// turns an inert shape difference into a command-level failure. These
// helpers accept either shape so the feature commands degrade gracefully
// instead of erroring out.
package cli

import "encoding/json"

// decodeJSONList extracts a list of items from an API response that may be
// shaped as an envelope object ({"<key>": [...]}) or as a bare JSON array.
// Returns nil if neither shape is present or the payload isn't valid JSON.
func decodeJSONList(data []byte, key string) []any {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err == nil {
		if list, ok := obj[key].([]any); ok {
			return list
		}
		return nil
	}

	var list []any
	if err := json.Unmarshal(data, &list); err == nil {
		return list
	}

	return nil
}
