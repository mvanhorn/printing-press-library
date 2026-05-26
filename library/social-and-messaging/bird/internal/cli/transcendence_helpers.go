// Copyright 2026 cleverai-zh. Licensed under Apache-2.0. See LICENSE.
package cli

import "encoding/json"

// parseMessagesEnvelope normalizes Bird message-list responses across the
// "raw array" and "wrapped {results|data|items}" shapes the various endpoints
// return. Shared by tenant_doctor and other novel features that consume
// /channels/.../messages style payloads.
func parseMessagesEnvelope(raw json.RawMessage) []map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr
	}
	var wrapped struct {
		Results []map[string]any `json:"results"`
		Data    []map[string]any `json:"data"`
		Items   []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return nil
	}
	switch {
	case len(wrapped.Results) > 0:
		return wrapped.Results
	case len(wrapped.Data) > 0:
		return wrapped.Data
	case len(wrapped.Items) > 0:
		return wrapped.Items
	}
	return nil
}
