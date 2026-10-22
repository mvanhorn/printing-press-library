// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
)

// largeStringLimit caps any single string returned from a write. FedEx
// returns labels and trade documents as base64 blobs that are useless (and
// expensive) in an agent's context; ask for labelResponseOptions=URL_ONLY to
// get a printable label URL instead.
const largeStringLimit = 2048

// summarizeWriteResponse keeps the fields an agent needs after a label,
// cancellation, or pickup write (tracking numbers, charges, confirmation
// codes, label URLs) and drops recipient details and encoded documents. Other
// writes return the FedEx response with encoded documents omitted.
func summarizeWriteResponse(tool string, value any) any {
	var keys []string
	switch tool {
	case "shipments_create", "ship_bulk":
		keys = []string{"mastertrackingnumber", "trackingnumber", "shipmentid", "servicetype", "netcharge", "totalnetcharge", "totalnetfedexcharge", "currency", "url", "jobid"}
	case "pickups_create":
		keys = []string{"pickupconfirmationcode", "confirmationnumber", "scheduleddate", "location"}
	case "shipments_cancel", "pickups_cancel":
		keys = []string{"cancelled", "canceled", "successful", "success", "cancelledshipment", "message"}
	default:
		return omitLargeStrings(value)
	}
	allowed := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		allowed[key] = struct{}{}
	}
	result := map[string]any{}
	collectAllowedFields(value, allowed, result)
	if len(result) == 0 {
		result["response_received"] = true
	}
	return result
}

func omitLargeStrings(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, child := range typed {
			out[key] = omitLargeStrings(child)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, child := range typed {
			out[i] = omitLargeStrings(child)
		}
		return out
	case string:
		if len(typed) > largeStringLimit {
			return fmt.Sprintf("[omitted %d-byte encoded value]", len(typed))
		}
		return typed
	default:
		return value
	}
}

func collectAllowedFields(value any, allowed map[string]struct{}, result map[string]any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if _, ok := allowed[strings.ToLower(key)]; ok && isSafeScalar(child) {
				appendResult(result, key, child)
				continue
			}
			collectAllowedFields(child, allowed, result)
		}
	case []any:
		for _, child := range typed {
			collectAllowedFields(child, allowed, result)
		}
	}
}

func isSafeScalar(value any) bool {
	switch typed := value.(type) {
	case string:
		return len(typed) <= largeStringLimit
	case bool, float64, json.Number:
		return true
	default:
		return false
	}
}

func appendResult(result map[string]any, key string, value any) {
	if existing, ok := result[key]; ok {
		if values, isSlice := existing.([]any); isSlice {
			result[key] = append(values, value)
		} else {
			result[key] = []any{existing, value}
		}
		return
	}
	result[key] = value
}
