// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"strings"
)

// Helpers ported from the prior CLI's sync/helpers layer that the novel
// commands depend on. The fresh 4.27.1 sync/helpers use different internals,
// so these live in a dedicated novel-support file.

func extractSyncItems(data json.RawMessage) []map[string]any {
	if items := extractNamedArray(data, "value", "data"); len(items) > 0 {
		return items
	}
	if items := extractNamedArray(data, "data"); len(items) > 0 {
		return items
	}
	var direct []map[string]any
	if json.Unmarshal(data, &direct) == nil {
		return direct
	}
	return collectMaps(data)
}

func extractNamedArray(data json.RawMessage, path ...string) []map[string]any {
	var v any
	if json.Unmarshal(data, &v) != nil {
		return nil
	}
	for _, key := range path {
		obj, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = obj[key]
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func collectMaps(data json.RawMessage) []map[string]any {
	var v any
	if json.Unmarshal(data, &v) != nil {
		return nil
	}
	var out []map[string]any
	var walk func(any)
	walk = func(x any) {
		switch t := x.(type) {
		case []any:
			for _, item := range t {
				walk(item)
			}
		case map[string]any:
			if firstSyncString(t, "id", "appId", "screenId") != "" {
				out = append(out, t)
			}
			for _, val := range t {
				walk(val)
			}
		}
	}
	walk(v)
	return out
}

func firstSyncString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if s, ok := m[key].(string); ok {
			return s
		}
	}
	return ""
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func mustMarshalJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("null")
	}
	return b
}
