// Copyright 2026 pimmetjeoss. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ResourceIDString returns the stable text form used for resources.id.
//
// Ported from the current generator template during the 4.31.1 reprint
// (merge reconciliation): float64 IDs must render without an exponent
// (strconv.FormatFloat 'f') so large integer IDs survive sync round-trips.
// The preserved extractID path predates it and stringified floats with %v.
func ResourceIDString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return extendedJSONIDString(t)
	case map[string]any:
		return extendedJSONIDMapString(t)
	case json.Number:
		return strings.TrimSpace(t.String())
	case float64:
		if math.IsNaN(t) || math.IsInf(t, 0) {
			return ""
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case float32:
		f := float64(t)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return ""
		}
		return strconv.FormatFloat(f, 'f', -1, 32)
	default:
		// fmt.Sprint on typed nil pointers returns "<nil>"; callers still guard
		// that sentinel so unresolved IDs do not become stored resource keys.
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func extendedJSONIDString(value string) string {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "{") {
		return value
	}
	var object map[string]any
	if err := json.Unmarshal([]byte(value), &object); err != nil {
		return value
	}
	if id := extendedJSONIDMapString(object); id != "" {
		return id
	}
	return value
}

func extendedJSONIDMapString(object map[string]any) string {
	for _, key := range []string{"$oid", "$numberLong", "$numberInt"} {
		if value, ok := object[key]; ok {
			return ResourceIDString(value)
		}
	}
	return ""
}
