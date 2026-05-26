// Copyright 2026 jmbernabotto. Licensed under Apache-2.0. See LICENSE.

package cli

import "encoding/json"

// navitiaItems extracts a named array from a Navitia wrapped response.
// Navitia always returns {"<resource>": [...], "pagination": {...}, ...};
// this unwraps the inner list for the given resource key.
func navitiaItems(data json.RawMessage, key string) []map[string]any {
	var resp map[string]any
	if json.Unmarshal(data, &resp) != nil {
		return nil
	}
	list, _ := resp[key].([]any)
	result := make([]map[string]any, 0, len(list))
	for _, d := range list {
		if m, ok := d.(map[string]any); ok {
			result = append(result, m)
		}
	}
	return result
}
