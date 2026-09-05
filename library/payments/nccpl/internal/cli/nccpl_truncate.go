package cli

import (
	"context"
	"encoding/json"
)

// truncateJSONArray honors --limit when the API accepts but ignores ?limit=N.
//
// The generator emits a call to this helper for any GET endpoint that declares a
// `limit` param, but does not emit the definition itself (cli-printing-press
// v4.31.7). Without this file the module does not compile. Kept in a separate
// hand-authored file so regen preserves it.
func truncateJSONArray(_ context.Context, data json.RawMessage, limit int) json.RawMessage {
	if limit <= 0 || !isJSONArray(data) {
		return data
	}
	var items []json.RawMessage
	if err := json.Unmarshal(data, &items); err != nil {
		return data
	}
	if len(items) <= limit {
		return data
	}
	out, err := json.Marshal(items[:limit])
	if err != nil {
		return data
	}
	return out
}
