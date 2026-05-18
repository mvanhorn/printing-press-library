// Copyright 2026 jacobprice. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
)

// jsonStringField returns the first non-empty string-valued field found at
// any of the given dotted paths in the JSON document. Missing keys, null
// values, and non-string types are skipped silently — the audit commands
// run over heterogeneous payload shapes (server-scoped vs agency-scoped),
// and a missing field on one variant is not an error.
func jsonStringField(raw string, paths ...string) string {
	if raw == "" {
		return ""
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return ""
	}
	for _, p := range paths {
		if v := lookupDotted(obj, p); v != "" {
			return v
		}
	}
	return ""
}

// jsonAnyField is like jsonStringField but also converts numeric values to
// their canonical string form. Used for IDs that some endpoints serialize
// as numbers and others as strings.
func jsonAnyField(raw string, paths ...string) string {
	if raw == "" {
		return ""
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return ""
	}
	for _, p := range paths {
		v := lookupDottedAny(obj, p)
		switch t := v.(type) {
		case string:
			if t != "" {
				return t
			}
		case float64:
			return fmt.Sprintf("%g", t)
		case bool:
			return fmt.Sprintf("%t", t)
		}
	}
	return ""
}

func lookupDotted(obj map[string]any, path string) string {
	v := lookupDottedAny(obj, path)
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func lookupDottedAny(obj map[string]any, path string) any {
	parts := strings.Split(path, ".")
	var cur any = obj
	for _, p := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur, ok = m[p]
		if !ok {
			return nil
		}
	}
	return cur
}
