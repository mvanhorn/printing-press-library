// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored: supplies truncateJSONArray, which generated list commands
// reference but the generator did not emit (generator gap). Kept in its own
// file so it survives `generate --force` regeneration.

package cli

import "encoding/json"

// truncateJSONArray honors a client-side --limit for list endpoints whose
// upstream accepts but ignores ?limit=N. If data decodes to a JSON array and
// limit > 0 and the array is longer than limit, it returns the first limit
// elements re-encoded; otherwise it returns data unchanged.
func truncateJSONArray(data json.RawMessage, limit int) json.RawMessage {
	if limit <= 0 || len(data) == 0 {
		return data
	}
	var items []json.RawMessage
	if err := json.Unmarshal(data, &items); err != nil {
		return data
	}
	if len(items) <= limit {
		return data
	}
	truncated, err := json.Marshal(items[:limit])
	if err != nil {
		return data
	}
	return truncated
}
