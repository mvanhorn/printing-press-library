// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package envelope

import (
	"encoding/json"
	"os"
	"strconv"
)

const defaultMaxBytes = 32000

const defaultTruncationNote = "MCP response exceeded the tool result budget. Narrow the request with limit, offset, filters, search/sql, or a command-mirror tool with --agent/--compact/--select."

// Options controls canonical MCP result envelope rendering.
type Options struct {
	Count          int
	Offset         int
	ItemCap        int
	MaxBytes       int
	Note           string
	SupportsCursor bool
}

// MaxBytes is the shared byte budget for all MCP typed and mirrored surfaces.
func MaxBytes() int {
	if v, err := strconv.Atoi(os.Getenv("PP_MCP_MAX_BYTES")); err == nil && v > 0 {
		return v
	}
	return defaultMaxBytes
}

// List renders items into the canonical MCP pagination envelope:
// {results,count,returned_count,truncated,next_offset,max_bytes,note}.
func List(items []json.RawMessage, opts Options) []byte {
	if items == nil {
		items = []json.RawMessage{}
	}
	if opts.MaxBytes <= 0 {
		opts.MaxBytes = MaxBytes()
	}
	if opts.Count < 0 {
		opts.Count = len(items)
	}
	if opts.Offset < 0 {
		opts.Offset = 0
	}
	if opts.Offset > opts.Count {
		opts.Offset = opts.Count
	}
	if opts.Note == "" {
		opts.Note = defaultTruncationNote
	}
	limit := len(items)
	if opts.ItemCap > 0 && limit > opts.ItemCap {
		limit = opts.ItemCap
	}
	return fitArray(items, limit, opts.MaxBytes, "results", func(page []json.RawMessage) map[string]any { return build(page, opts) })
}

// ObjectArray renders an object response after replacing one top-level array
// field with a budgeted page and attaching pagination metadata. Sibling fields
// from the original object are preserved.
func ObjectArray(obj map[string]json.RawMessage, arrayField string, items []json.RawMessage, opts Options) []byte {
	if items == nil {
		items = []json.RawMessage{}
	}
	if opts.MaxBytes <= 0 {
		opts.MaxBytes = MaxBytes()
	}
	if opts.Count < 0 {
		opts.Count = len(items)
	}
	if opts.Offset < 0 {
		opts.Offset = 0
	}
	if opts.Offset > opts.Count {
		opts.Offset = opts.Count
	}
	if opts.Note == "" {
		opts.Note = defaultTruncationNote
	}
	limit := len(items)
	if opts.ItemCap > 0 && limit > opts.ItemCap {
		limit = opts.ItemCap
	}
	return fitArray(items, limit, opts.MaxBytes, arrayField, func(page []json.RawMessage) map[string]any { return buildObject(obj, arrayField, page, opts) })
}

// Preview renders a bounded over-budget preview for responses that cannot be
// reduced as list-shaped JSON.
func Preview(data []byte, maxBytes int, note string) []byte {
	if maxBytes <= 0 {
		maxBytes = MaxBytes()
	}
	if note == "" {
		note = "MCP response exceeded the tool result budget and was not a recognized list envelope. Narrow the request with filters, search/sql, or a command-mirror tool with --agent/--compact/--select."
	}
	previewLimit := 4000
	if maxBytes > 0 && previewLimit > maxBytes/2 {
		previewLimit = maxBytes / 2
	}
	if previewLimit < 0 {
		previewLimit = 0
	}
	previewBytes := data
	if len(previewBytes) > previewLimit {
		previewBytes = previewBytes[:previewLimit]
	}
	for {
		out, _ := json.Marshal(map[string]any{
			"truncated":      true,
			"original_bytes": len(data),
			"max_bytes":      maxBytes,
			"preview":        string(previewBytes),
			"note":           note,
		})
		if len(out) <= maxBytes || len(previewBytes) == 0 {
			return out
		}
		previewBytes = previewBytes[:len(previewBytes)/2]
	}
}

func build(results []json.RawMessage, opts Options) map[string]any {
	returned := len(results)
	truncated := opts.Offset+returned < opts.Count
	out := map[string]any{
		"results":        results,
		"count":          opts.Count,
		"returned_count": returned,
		"truncated":      truncated,
		"max_bytes":      opts.MaxBytes,
	}
	if truncated {
		if opts.SupportsCursor {
			out["next_offset"] = opts.Offset + returned
		}
		out["note"] = opts.Note
	}
	return out
}

func buildObject(obj map[string]json.RawMessage, arrayField string, results []json.RawMessage, opts Options) map[string]any {
	out := make(map[string]any, len(obj)+6)
	for key, raw := range obj {
		out[key] = raw
	}
	if results == nil {
		results = []json.RawMessage{}
	}
	returned := len(results)
	truncated := opts.Offset+returned < opts.Count
	out[arrayField] = results
	if _, exists := obj["count"]; !exists {
		out["count"] = opts.Count
	}
	out["returned_count"] = returned
	out["truncated"] = truncated
	out["max_bytes"] = opts.MaxBytes
	if truncated {
		if opts.SupportsCursor {
			out["next_offset"] = opts.Offset + returned
		}
		out["note"] = opts.Note
	}
	return out
}

// fitArray measures each encoded item once; only metadata is marshaled during
// the binary search. The winning page is serialized once at the end.
func fitArray(items []json.RawMessage, limit, budget int, field string, build func([]json.RawMessage) map[string]any) []byte {
	sizes := make([]int, limit+1)
	for i := 0; i < limit; i++ {
		raw, err := json.Marshal(items[i])
		if err != nil {
			return nil
		}
		sizes[i+1] = sizes[i] + len(raw)
	}
	fits := func(n int) bool {
		obj := build(items[:n])
		obj[field] = []json.RawMessage{}
		meta, err := json.Marshal(obj)
		commas := 0
		if n > 0 {
			commas = n - 1
		}
		return err == nil && len(meta)+sizes[n]+commas <= budget
	}
	// Completing a list removes the truncation note, so check that endpoint first.
	if fits(limit) {
		out, _ := json.Marshal(build(items[:limit]))
		return out
	}
	lo, hi := 0, limit
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if fits(mid) {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	out, _ := json.Marshal(build(items[:lo]))
	return out
}
