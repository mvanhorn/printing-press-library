// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package bound

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

const (
	DefaultMaxBytes  = 32000
	BudgetFloor      = 2048
	ItemCap          = 10000
	MirrorCaptureMax = 16 << 20
)

var budgetClampOnce sync.Once
var budgetCeilingOnce sync.Once
var budgetInvalidOnce sync.Once
var budgetNonpositiveOnce sync.Once
var mirrorEventWriter io.Writer = os.Stderr

func Budget() int {
	raw := strings.TrimSpace(os.Getenv("PP_MCP_MAX_BYTES"))
	if raw == "" {
		return DefaultMaxBytes
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	// All-digit positive overflow is a ceiling request, even beyond int64.
	if err != nil {
		if len(strings.TrimPrefix(raw, "+")) > 0 && strings.IndexFunc(strings.TrimPrefix(raw, "+"), func(r rune) bool { return r < '0' || r > '9' }) < 0 {
			budgetCeilingOnce.Do(func() { fmt.Fprintln(os.Stderr, "PP_MCP_MAX_BYTES clamped to maximum 16777216") })
			return MirrorCaptureMax
		}
		budgetInvalidOnce.Do(func() { fmt.Fprintln(os.Stderr, "PP_MCP_MAX_BYTES invalid; using default 32000") })
		return DefaultMaxBytes
	}
	if v <= 0 {
		budgetNonpositiveOnce.Do(func() { fmt.Fprintln(os.Stderr, "PP_MCP_MAX_BYTES must be positive; using default 32000") })
		return DefaultMaxBytes
	}
	if v < BudgetFloor {
		budgetClampOnce.Do(func() { fmt.Fprintln(os.Stderr, "PP_MCP_MAX_BYTES clamped to minimum 2048") })
		return BudgetFloor
	}
	if v > MirrorCaptureMax {
		budgetCeilingOnce.Do(func() { fmt.Fprintln(os.Stderr, "PP_MCP_MAX_BYTES clamped to maximum 16777216") })
		return MirrorCaptureMax
	}
	return int(v)
}

type MirrorRequest struct {
	Tool           string
	ArgsHash       string
	Cursor         *MirrorCursor
	SupportsCursor bool
	SnapshotParam  bool
	Narrow         string
	NoRerun        string
	MaxBytes       int
	Reserve        int // Bytes reserved for another content block; max_bytes still reports MaxBytes.
	TrueBytes      int
	Overflow       bool
}
type MirrorResult struct {
	Text    string
	IsError bool
	Event   string
}
type MirrorCursor struct {
	K string `json:"k"`
	V int    `json:"v"`
	T string `json:"t"`
	A string `json:"a"`
	O int    `json:"o"`
	D string `json:"d"`
}
type CursorError struct{ Code, Msg string }

func (e *CursorError) Error() string { return e.Code + ": " + e.Msg }
func mirrorDigest(b []byte) string   { h := sha256.Sum256(b); return fmt.Sprintf("%x", h[:8]) }
func ArgsHash(argv []string) string {
	var b strings.Builder
	for _, a := range argv {
		fmt.Fprintf(&b, "%d:", len(a))
		b.WriteString(a)
	}
	return mirrorDigest([]byte(b.String()))
}
func EncodeMirrorCursor(c MirrorCursor) string {
	c.K = "m"
	c.V = 2
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}
func DecodeMirrorCursor(raw, tool, argsHash string) (*MirrorCursor, *CursorError) {
	if raw == "" {
		return nil, nil
	}
	invalid := func() (*MirrorCursor, *CursorError) {
		return nil, &CursorError{"cursor_invalid", "invalid mirror cursor"}
	}
	if len(raw) > 1024 {
		return invalid()
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return invalid()
	}
	var c MirrorCursor
	if json.Unmarshal(b, &c) != nil || c.K != "m" || c.V != 2 {
		return invalid()
	}
	if c.T != tool {
		return nil, &CursorError{"cursor_wrong_tool", "cursor belongs to a different tool"}
	}
	if c.A != argsHash {
		return nil, &CursorError{"cursor_args_mismatch", "resend the same arguments you used for the first page, plus cursor"}
	}
	return &c, nil
}
func mirrorNote(r MirrorRequest, cursorEmitted bool, opening string) string {
	if !r.SupportsCursor && r.NoRerun != "" {
		return r.NoRerun
	}
	s := opening
	if cursorEmitted {
		s += " Pass next_cursor back as cursor together with the same arguments."
	}
	if !r.SupportsCursor && !strings.Contains(opening, "cannot be paged") && !strings.Contains(opening, "otherwise complete") {
		s += " The rest cannot be paged with a cursor."
	}
	if r.Narrow != "" {
		s += " " + r.Narrow
	}
	return s
}

// fitFragment sizes the final JSON, including escaping and all metadata.
func fitFragment(data []byte, limit int, build func(string) []byte) []byte {
	lo, hi := 0, len(data)
	best := build("")
	for lo <= hi {
		mid := lo + (hi-lo)/2
		end := mid
		for back := 0; end > 0 && end < len(data) && !utf8.RuneStart(data[end]) && back < 3; back++ {
			end--
		}
		b := build(string(data[:end]))
		if len(b) <= limit {
			best = b
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return best
}
func mirrorPreview(data []byte, r MirrorRequest, original int) []byte {
	return fitFragment(data, mirrorLimit(r), func(s string) []byte {
		b, _ := json.Marshal(map[string]any{"truncated": true, "oversized": false, "max_bytes": r.MaxBytes, "original_bytes": original, "preview": s, "note": mirrorNote(r, false, "This output exceeded the budget and cannot be paged.")})
		return b
	})
}
func mirrorLimit(r MirrorRequest) int {
	reserve := r.Reserve
	if reserve < 0 {
		reserve = 0
	}
	if reserve >= r.MaxBytes {
		return 0
	}
	return r.MaxBytes - reserve
}

// minimalTruncation preserves an incomplete signal even when metadata cannot fit.
func minimalTruncation(budget, limit int) []byte {
	b := []byte(fmt.Sprintf(`{"truncated":true,"max_bytes":%d}`, budget))
	if len(b) <= limit {
		return b
	}
	return nil
}
func TextWithin(s string, budget, reserve int) string {
	limit := mirrorLimit(MirrorRequest{MaxBytes: budget, Reserve: reserve})
	if len(s) <= limit {
		return s
	}
	b := mirrorPreview([]byte(s), MirrorRequest{MaxBytes: budget, Reserve: reserve}, len(s))
	// A reserved remainder can be smaller than even the empty envelope.
	if len(b) > limit {
		return string(minimalTruncation(budget, limit))
	}
	return string(b)
}

func Mirror(stdout []byte, req MirrorRequest) MirrorResult {
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
	trimmed := bytes.TrimSpace(stdout)
	if bytes.HasPrefix(trimmed, []byte("[")) && json.Unmarshal(stdout, &items) == nil {
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
				if !bytes.HasPrefix(bytes.TrimSpace(v), []byte("{")) {
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
	// Size only the budget-sized prefix. RawMessage marshaling compacts and
	// HTML-escapes, so raw input lengths are not reliable estimates.
	fullFrame := len(build(0, false, false, nil))
	pageFrame := len(build(0, true, true, nil))
	if pageFrame > limit && fullFrame > limit {
		return preview()
	}
	best, allFit := mirrorPageSize(items[o:], fullFrame, pageFrame, limit)
	if allFit {
		b := build(remaining, false, false, nil)
		if b != nil && len(b) <= limit {
			return finish(b, "")
		}
	}
	if pageFrame > limit {
		return preview()
	}
	// Verify the estimate once; corrections may only reduce the page.
	for best > 0 {
		if b := build(best, true, true, nil); b != nil && len(b) <= limit {
			break
		}
		best--
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

// mirrorPageSize only visits the budget-sized prefix, independently of input parsing.
func mirrorPageSize(items []json.RawMessage, fullFrame, pageFrame, limit int) (int, bool) {
	remaining := len(items)
	best, payload := 0, 0
	allFit := remaining <= ItemCap
	var compact, escaped bytes.Buffer
	for k := 1; k <= remaining && k <= ItemCap; k++ {
		compact.Reset()
		escaped.Reset()
		_ = json.Compact(&compact, items[k-1])
		json.HTMLEscape(&escaped, compact.Bytes())
		payload += escaped.Len()
		if k > 1 {
			payload++
		}
		digits := 0
		for v := k; v >= 10; v /= 10 {
			digits++
		}
		if pageFrame+payload+digits <= limit {
			best = k
		}
		if fullFrame+payload+digits > limit {
			allFit = false
		}
		if pageFrame+payload+digits > limit && !allFit {
			break
		}
	}
	return best, allFit
}
