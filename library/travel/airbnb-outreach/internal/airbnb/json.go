// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.

package airbnb

import "encoding/json"

// rawAtPath descends into a JSON object along the given keys and returns the
// raw value at the end of the path, or nil if any segment is missing or not an
// object. Used to pluck the meaningful sub-tree out of Airbnb's deeply nested
// GraphQL envelopes (e.g. presentation.staysSearch.results.searchResults).
func rawAtPath(data json.RawMessage, path ...string) json.RawMessage {
	cur := data
	for _, key := range path {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(cur, &obj); err != nil {
			return nil
		}
		next, ok := obj[key]
		if !ok {
			return nil
		}
		cur = next
	}
	return cur
}
