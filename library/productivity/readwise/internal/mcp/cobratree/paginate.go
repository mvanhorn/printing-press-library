// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// LOCAL PATCH (not generated): byte-budget pagination for mirrored shell-out
// output. Ported from resident-advisor. Upstream candidate for the Printing Press
// cobratree generator. Do not assume regen will preserve this file.

package cobratree

import (
	"encoding/json"

	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/mcp/envelope"
)

func paginateMirroredOutput(out string, offset, maxBytes int) string {
	if maxBytes <= 0 {
		return out
	}
	var items []json.RawMessage
	if err := json.Unmarshal([]byte(out), &items); err == nil {
		// Under budget and no paging requested -> return the bare array unchanged.
		if len(out) <= maxBytes && offset <= 0 {
			return out
		}
		return string(pageMirrorItems(items, offset, maxBytes))
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &obj); err != nil {
		if len(out) > maxBytes {
			return string(envelope.Preview([]byte(out), maxBytes, "Mirrored command response exceeded the tool result budget and could not be parsed as complete JSON (the capture limit may have been reached). Re-run with narrower flags, --compact, or --select."))
		}
		return out
	}
	if len(out) <= maxBytes && offset <= 0 {
		return out
	}
	// Agent wrappers retain meta and all outer siblings while paging results.
	if raw, wrapped := obj["results"]; wrapped && obj["meta"] != nil {
		siblings := make(map[string]json.RawMessage, len(obj))
		for k, v := range obj {
			if k != "results" {
				siblings[k] = v
			}
		}
		overhead, _ := json.Marshal(siblings)
		inner := paginateMirroredOutput(string(raw), offset, maxBytes-len(overhead)-12)
		obj["results"] = json.RawMessage(inner)
		out, _ := json.Marshal(obj)
		return string(out)
	}
	field, fieldItems := largestArrayField(obj)
	if field == "" {
		if len(out) > maxBytes {
			return string(envelope.Preview([]byte(out), maxBytes, "Mirrored command response exceeded the tool result budget and did not include a top-level array field to paginate. Re-run with narrower flags, --compact, or --select."))
		}
		return out
	}
	return string(pageMirrorObjectItems(obj, field, fieldItems, offset, maxBytes))
}

func pageMirrorItems(items []json.RawMessage, offset, maxBytes int) []byte {
	if offset < 0 {
		offset = 0
	}
	if offset > len(items) {
		offset = len(items)
	}
	return envelope.List(items[offset:], envelope.Options{
		Count:          len(items),
		Offset:         offset,
		MaxBytes:       maxBytes,
		Note:           "Mirrored command response exceeded the tool result budget. Pass next_offset back as offset to resume, or re-run with narrower flags, --compact, or --select.",
		SupportsCursor: true,
	})
}

func pageMirrorObjectItems(obj map[string]json.RawMessage, field string, items []json.RawMessage, offset, maxBytes int) []byte {
	if offset < 0 {
		offset = 0
	}
	if offset > len(items) {
		offset = len(items)
	}
	return envelope.ObjectArray(obj, field, items[offset:], envelope.Options{
		Count:          len(items),
		Offset:         offset,
		MaxBytes:       maxBytes,
		Note:           "Mirrored command response exceeded the tool result budget. Pass next_offset back as offset to resume, or re-run with narrower flags, --compact, or --select.",
		SupportsCursor: true,
	})
}

func largestArrayField(obj map[string]json.RawMessage) (string, []json.RawMessage) {
	var bestField string
	var bestItems []json.RawMessage
	for field, raw := range obj {
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			continue
		}
		if len(items) > len(bestItems) {
			bestField = field
			bestItems = items
		}
	}
	return bestField, bestItems
}
