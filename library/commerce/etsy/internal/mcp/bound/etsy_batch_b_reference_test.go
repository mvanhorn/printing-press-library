// Test-only pre-Batch-B reference.
package bound

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func oldMirror(stdout []byte, req MirrorRequest) MirrorResult {
	if req.MaxBytes <= 0 {
		req.MaxBytes = Budget()
	}
	limit := mirrorLimit(req)
	original := req.TrueBytes
	if original == 0 {
		original = len(stdout)
	}
	finish := func(b []byte, event string) MirrorResult {
		if event != "" {
			fmt.Fprintf(mirrorEventWriter, "pp-mcp-bound event=%s method=%s original_bytes=%d max_bytes=%d\n", event, req.Tool, original, req.MaxBytes)
		}
		if len(b) > limit {
			b = minimalTruncation(req.MaxBytes, limit)
		}
		return MirrorResult{Text: string(b), Event: event}
	}
	preview := func() MirrorResult { return finish(mirrorPreview(stdout, req, original), "preview") }
	digest := mirrorDigest(stdout)
	fail := func(code, msg string) MirrorResult {
		return MirrorResult{Text: (&CursorError{code, msg}).Error(), IsError: true}
	}
	if req.Cursor != nil && req.Cursor.D != digest {
		causes := "store write or time-dependent fields"
		if req.SnapshotParam {
			causes = "new snapshot, store write, or time-dependent fields"
		}
		message := "the results changed since the first page (" + causes + "); re-run from the first page without `cursor`."
		if req.SnapshotParam {
			message += " To keep pages consistent, pass `snapshot` set to the page-1 `snapshot_id` (top-level, or `results.snapshot_id` with `agent: true`; if `select` dropped it, re-run once without `select` to read it)."
		}
		return fail("cursor_stale", message)
	}
	if req.Overflow {
		return preview()
	}
	if req.Cursor == nil && len(stdout) <= limit {
		return finish(stdout, "")
	}
	var items []json.RawMessage
	var obj map[string]json.RawMessage
	var wrapper map[string]json.RawMessage
	key := "items"
	trimmed := strings.TrimSpace(string(stdout))
	if strings.HasPrefix(trimmed, "[") && json.Unmarshal(stdout, &items) == nil {
		// Bare arrays may contain any JSON values.
	} else {
		if json.Unmarshal(stdout, &obj) != nil || obj == nil {
			return preview()
		}
		// Descend exactly once through the agent wrapper. Canonical metadata
		// belongs at the top level: absent top-level truncated means complete.
		if len(obj) == 2 && obj["meta"] != nil && obj["results"] != nil {
			var inner map[string]json.RawMessage
			if json.Unmarshal(obj["results"], &inner) == nil && inner != nil {
				wrapper, obj = obj, inner
			}
		}
		keys := make([]string, 0, len(obj))
		for k := range obj {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		best := 0
		key = ""
		for _, k := range keys {
			raw := obj[k]
			var a []json.RawMessage
			if json.Unmarshal(raw, &a) != nil || len(a) == 0 {
				continue
			}
			objects := true
			for _, v := range a {
				if !strings.HasPrefix(strings.TrimSpace(string(v)), "{") {
					objects = false
					break
				}
			}
			if objects && len(raw) > best {
				best = len(raw)
				key = k
				items = a
			}
		}
		if key == "" {
			return preview()
		}
	}
	n := len(items)
	o := 0
	if req.Cursor != nil {
		o = req.Cursor.O
		if o < 0 || o >= n {
			return fail("cursor_invalid", "cursor offset is out of range")
		}
	}
	base := map[string]any{}
	if wrapper != nil {
		for k, v := range wrapper {
			base[k] = v
		}
	} else {
		for k, v := range obj {
			base[k] = v
		}
	}
	// Reserve canonical keys even on the final page, so data cannot masquerade as metadata.
	for _, k := range []string{"truncated", "oversized", "max_bytes", "returned_count", "next_cursor", "item_bytes", "oversized_item_id", "oversized_preview", "count"} {
		if v, ok := base[k]; ok {
			if k == "count" {
				var count float64
				if json.Unmarshal(v.(json.RawMessage), &count) == nil && count == float64(n) {
					delete(base, k)
					continue
				}
			}
			base["_pp_"+k] = v
			delete(base, k)
		}
	}
	noteKey := "note"
	if _, ok := base["note"]; ok {
		noteKey = "_pp_note"
	}
	cursor := func(offset int) string {
		return EncodeMirrorCursor(MirrorCursor{T: req.Tool, A: req.ArgsHash, O: offset, D: digest})
	}
	build := func(k int, truncated bool, reserve bool, extra map[string]any) []byte {
		m := make(map[string]any, len(base)+10)
		for k, v := range base {
			m[k] = v
		}
		page := items[o : o+k]
		if len(page) == 0 {
			page = []json.RawMessage{}
		}
		if wrapper != nil {
			inner := make(map[string]any, len(obj))
			for k, v := range obj {
				inner[k] = v
			}
			inner[key] = page
			m["results"] = inner
		} else {
			m[key] = page
		}
		m["count"] = n
		m["returned_count"] = k
		m["truncated"] = truncated
		m["oversized"] = false
		m["max_bytes"] = req.MaxBytes
		if truncated {
			m[noteKey] = mirrorNote(req, req.SupportsCursor, "This output exceeded the budget.")
			if req.SupportsCursor {
				offset := o + k
				if reserve {
					offset = n
				}
				m["next_cursor"] = cursor(offset)
			}
		}
		for k, v := range extra {
			m[k] = v
		}
		b, err := json.Marshal(m)
		if err != nil {
			return nil
		}
		return b
	}
	remaining := n - o
	if remaining <= ItemCap {
		b := build(remaining, false, false, nil)
		if b != nil && len(b) <= limit {
			return finish(b, "")
		}
	}
	if b := build(0, true, true, nil); b == nil || len(b) > limit {
		return preview()
	}
	hi := remaining
	if hi > ItemCap {
		hi = ItemCap
	}
	lo := 0
	best := 0
	for lo <= hi {
		mid := lo + (hi-lo)/2
		b := build(mid, true, true, nil)
		if b != nil && len(b) <= limit {
			best = mid
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	if best > 0 {
		return finish(build(best, true, false, nil), "truncated")
	}
	if remaining == 0 {
		return preview()
	}
	item := items[o]
	id := ""
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(item, &fields)
	for _, k := range []string{"listing_id", "shop_id", "receipt_id", "transaction_id", "payment_id", "entry_id", "id", "name"} {
		if raw, ok := fields[k]; ok {
			var s string
			var num json.Number
			if json.Unmarshal(raw, &s) == nil {
				id = s
				break
			}
			if json.Unmarshal(raw, &num) == nil && string(raw) != "null" {
				id = num.String()
				break
			}
		}
	}
	b := fitFragment(item, limit, func(s string) []byte {
		extra := map[string]any{"oversized": true, "item_bytes": len(item), "oversized_item_id": id, "oversized_preview": s}
		out := build(0, true, false, extra)
		var m map[string]json.RawMessage
		_ = json.Unmarshal(out, &m)
		delete(m, "next_cursor")
		opening := "This item exceeded the budget."
		if o+1 == n {
			opening += " The result is otherwise complete."
		}
		m[noteKey], _ = json.Marshal(mirrorNote(req, req.SupportsCursor && o+1 < n, opening))
		if req.SupportsCursor && o+1 < n {
			m["next_cursor"], _ = json.Marshal(cursor(o + 1))
		}
		out, _ = json.Marshal(m)
		return out
	})
	if len(b) > limit {
		return preview()
	}
	return finish(b, "oversized")
}
