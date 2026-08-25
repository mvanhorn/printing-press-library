// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Helpers for merging README-style flat filter flags into the nested
// filterOptions/paginationOptions body Mobbin's search endpoints expect. The
// generator emits only the opaque --filter-options/--pagination-options JSON
// flags; these back the ergonomic flat flags (--platform, --screen-patterns,
// ...) documented in the narrative, README, and SKILL.

// bodyObjectField returns the map at body[key], accepting an existing object,
// a JSON string (the --*-options escape hatch), or nil (fresh object).
func bodyObjectField(body map[string]any, key string) (map[string]any, error) {
	switch v := body[key].(type) {
	case nil:
		return map[string]any{}, nil
	case map[string]any:
		return v, nil
	case string:
		if v == "" {
			return map[string]any{}, nil
		}
		var parsed map[string]any
		if err := json.Unmarshal([]byte(v), &parsed); err != nil {
			return nil, fmt.Errorf("parsing --%s JSON: %w", strings.ReplaceAll(key, "Options", "-options"), err)
		}
		// JSON `null` unmarshals to a nil map; callers write keys into the
		// result, so return a fresh map to avoid an assignment-to-nil-map panic.
		if parsed == nil {
			return map[string]any{}, nil
		}
		return parsed, nil
	default:
		return nil, fmt.Errorf("unexpected type for %s", key)
	}
}

// setNilDefaults sets any missing keys to JSON null. Mobbin validates the full
// filter schema, so unset fields must be present as null rather than absent.
func setNilDefaults(body map[string]any, keys ...string) {
	for _, key := range keys {
		if _, ok := body[key]; !ok {
			body[key] = nil
		}
	}
}

// setDefault sets key to value only when it is absent.
func setDefault(body map[string]any, key string, value any) {
	if _, ok := body[key]; !ok {
		body[key] = value
	}
}

type searchEnvelope struct {
	Value *struct {
		Data            json.RawMessage `json:"data"`
		HasNextPage     bool            `json:"hasNextPage"`
		TotalCount      int             `json:"totalCount"`
		SearchRequestID string          `json:"searchRequestId"`
	} `json:"value"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// parseSearchEnvelope unpacks the migrated Mobbin content-search response
// ({"value":{"data":[...],"hasNextPage":...,"totalCount":...}}) and re-wraps
// value.data under a top-level "data" key (so `--select data.id` keeps working)
// alongside total_count / has_next_page. Mobbin returns HTTP 200 with an
// {"error":{...}} body or an absent "value" for unauthenticated sessions;
// both become a non-success error (exit 3) so agents don't get a false success.
// Callers MUST skip this under --dry-run / verify, where the body is synthetic.
func parseSearchEnvelope(data json.RawMessage) (json.RawMessage, error) {
	var env searchEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		// Not the migrated shape (or not JSON); pass through unchanged.
		return data, nil
	}
	if env.Error != nil {
		msg := env.Error.Message
		if msg == "" {
			msg = "request failed"
		}
		return nil, &cliError{code: 3, err: fmt.Errorf("Mobbin API error: %s (run `mobbin-pp-cli auth login --chrome` if your session is unauthenticated)", msg)}
	}
	if env.Value == nil {
		return nil, &cliError{code: 3, err: fmt.Errorf("Mobbin API returned no results; your session may be unauthenticated — run `mobbin-pp-cli auth login --chrome`")}
	}
	arr := env.Value.Data
	if len(arr) == 0 {
		arr = json.RawMessage("[]")
	}
	payload := map[string]any{
		"data":          arr,
		"total_count":   env.Value.TotalCount,
		"has_next_page": env.Value.HasNextPage,
	}
	if env.Value.SearchRequestID != "" {
		payload["search_request_id"] = env.Value.SearchRequestID
	}
	return json.Marshal(payload)
}
