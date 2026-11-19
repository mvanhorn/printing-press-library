// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package bound

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func refreshBudgetForTest() { MaxBytes = Budget() }
func canonicalJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
func canonicalRows(n, size int) []map[string]any {
	rows := make([]map[string]any, n)
	for i := range rows {
		rows[i] = map[string]any{"listing_id": 1000000001 + i, "name": "ceramic mug", "text": strings.Repeat("x", size)}
	}
	return rows
}
func canonicalMap(t *testing.T, r MirrorResult, budget int) map[string]json.RawMessage {
	t.Helper()
	if r.IsError || len(r.Text) > budget {
		t.Fatalf("result error=%v bytes=%d budget=%d: %s", r.IsError, len(r.Text), budget, r.Text)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(r.Text), &m); err != nil {
		t.Fatal(err)
	}
	return m
}
func canonicalInt(t *testing.T, m map[string]json.RawMessage, k string) int {
	t.Helper()
	var n int
	if err := json.Unmarshal(m[k], &n); err != nil {
		t.Fatalf("%s: %v", k, err)
	}
	return n
}
func canonicalString(t *testing.T, m map[string]json.RawMessage, k string) string {
	t.Helper()
	var s string
	if err := json.Unmarshal(m[k], &s); err != nil {
		t.Fatalf("%s: %v", k, err)
	}
	return s
}
func canonicalBool(t *testing.T, m map[string]json.RawMessage, k string) bool {
	t.Helper()
	var b bool
	if err := json.Unmarshal(m[k], &b); err != nil {
		t.Fatalf("%s: %v", k, err)
	}
	return b
}

func TestMCPOutputNeverExceedsBudget(t *testing.T) {
	oldBudget := MaxBytes
	t.Setenv("PP_MCP_MAX_BYTES", "2048")
	refreshBudgetForTest()
	t.Cleanup(func() { MaxBytes = oldBudget })
	old := mirrorEventWriter
	mirrorEventWriter = io.Discard
	t.Cleanup(func() { mirrorEventWriter = old })
	rows := canonicalRows(30, 160)
	list := canonicalJSON(rows)
	type testCase struct {
		name  string
		data  []byte
		req   MirrorRequest
		check func(*testing.T, MirrorResult, map[string]json.RawMessage)
	}
	raw := func(t *testing.T, r MirrorResult, _ map[string]json.RawMessage) {
		if r.Text != "  []\n" {
			t.Fatal("raw changed")
		}
	}
	agent := canonicalJSON(map[string]any{"meta": map[string]any{"source": "local"}, "results": map[string]any{"results": rows, "note": "payload", "count": 999}})
	agentCheck := func(t *testing.T, r MirrorResult, m map[string]json.RawMessage) {
		if !canonicalBool(t, m, "truncated") || canonicalInt(t, m, "count") != len(rows) {
			t.Fatal(r)
		}
		inner := map[string]json.RawMessage{}
		if json.Unmarshal(m["results"], &inner) != nil || canonicalString(t, inner, "note") != "payload" || canonicalInt(t, inner, "count") != 999 {
			t.Fatal(r)
		}
		if _, ok := inner["truncated"]; ok {
			t.Fatal("inner canonical metadata")
		}
		if canonicalString(t, m, "note") == "" {
			t.Fatal("missing top level guidance")
		}
	}
	cases := []testCase{
		{name: "agent_descent_pages_inner_array_with_canonical_fields_top_level", data: agent, check: agentCheck},
		{name: "agent_descent_truncated_page_reads_incomplete_by_top_level_rule", data: agent, check: func(t *testing.T, r MirrorResult, m map[string]json.RawMessage) {
			truncated, present := m["truncated"]
			if !present || string(truncated) != "true" {
				t.Fatal("top-level rule incorrectly reads complete", r)
			}
		}},
		{name: "agent_descent_cursor_union_deep_equal", data: agent, check: func(t *testing.T, r MirrorResult, m map[string]json.RawMessage) {
			var union []map[string]any
			for i := 0; i < 100; i++ {
				var inner map[string]json.RawMessage
				_ = json.Unmarshal(m["results"], &inner)
				var page []map[string]any
				_ = json.Unmarshal(inner["results"], &page)
				union = append(union, page...)
				if !canonicalBool(t, m, "truncated") {
					var want []map[string]any
					_ = json.Unmarshal(canonicalJSON(rows), &want)
					if !reflect.DeepEqual(union, want) {
						t.Fatal("union differs")
					}
					return
				}
				c, e := DecodeMirrorCursor(canonicalString(t, m, "next_cursor"), "market_demand", ArgsHash(nil))
				if e != nil {
					t.Fatal(e)
				}
				r = Mirror(agent, MirrorRequest{Tool: "market_demand", ArgsHash: ArgsHash(nil), SupportsCursor: true, Cursor: c})
				m = canonicalMap(t, r, Budget())
			}
			t.Fatal("paging did not finish")
		}},
		{name: "agent_wrapper_with_array_results_no_descent", data: canonicalJSON(map[string]any{"meta": map[string]any{"source": "local"}, "results": rows}), check: func(t *testing.T, r MirrorResult, m map[string]json.RawMessage) {
			var page []json.RawMessage
			if json.Unmarshal(m["results"], &page) != nil || len(page) == 0 {
				t.Fatal(r)
			}
		}},
		{name: "reserve_subtracted_but_max_bytes_reports_full_budget", data: list, req: MirrorRequest{Reserve: 500}, check: func(t *testing.T, r MirrorResult, m map[string]json.RawMessage) {
			if len(r.Text) > Budget()-500 || canonicalInt(t, m, "max_bytes") != Budget() {
				t.Fatal(r)
			}
		}},
		{name: "empty", data: []byte{}, check: func(t *testing.T, r MirrorResult, _ map[string]json.RawMessage) {
			if r.Text != "" {
				t.Fatal(r)
			}
		}},
		{name: "fits_raw_passthrough_without_cursor", data: []byte("  []\n"), check: raw},
		{name: "fits_envelope_with_cursor_final_page", data: canonicalJSON(rows[:2]), req: MirrorRequest{Cursor: &MirrorCursor{O: 1, D: mirrorDigest(canonicalJSON(rows[:2]))}}, check: func(t *testing.T, r MirrorResult, m map[string]json.RawMessage) {
			if canonicalBool(t, m, "truncated") || canonicalInt(t, m, "returned_count") != 1 {
				t.Fatal(r)
			}
			if _, ok := m["note"]; ok {
				t.Fatal("final note")
			}
			if _, ok := m["next_cursor"]; ok {
				t.Fatal("final cursor")
			}
		}},
		{name: "past_budget", data: list, check: func(t *testing.T, r MirrorResult, m map[string]json.RawMessage) {
			if r.Event != "truncated" || canonicalInt(t, m, "returned_count") == 0 {
				t.Fatal(r)
			}
		}},
		{name: "binary_search_maximality", data: []byte("[" + strings.TrimSuffix(strings.Repeat("{},", 900), ",") + "]"), check: func(t *testing.T, r MirrorResult, m map[string]json.RawMessage) {
			k := canonicalInt(t, m, "returned_count")
			if k <= 50 || k >= 900 {
				t.Fatal(k)
			}
			var page []json.RawMessage
			_ = json.Unmarshal(m["items"], &page)
			page = append(page, json.RawMessage(`{}`))
			m["items"] = canonicalJSON(page)
			m["returned_count"] = canonicalJSON(k + 1)
			if len(canonicalJSON(m)) <= Budget() {
				t.Fatal("one extra item fits")
			}
		}},
		{name: "two_top_level_arrays_byte_dominant_objects_over_scalars", data: canonicalJSON(map[string]any{"results": rows, "small": rows[:1], "notes": []string{strings.Repeat("x", 800)}}), check: func(t *testing.T, r MirrorResult, m map[string]json.RawMessage) {
			if canonicalInt(t, m, "count") != 30 || !bytes.Equal(m["small"], canonicalJSON(rows[:1])) {
				t.Fatal(r)
			}
		}},
		{name: "array_free_to_preview", data: canonicalJSON(map[string]any{"text": strings.Repeat("€", 3000)})},
		{name: "scalar_array_only_to_preview", data: canonicalJSON(map[string]any{"notes": []string{strings.Repeat("x", 3000)}})},
		{name: "siblings_overflow_to_preview", data: canonicalJSON(map[string]any{"results": rows, "sibling": strings.Repeat("x", 3000)})},
		{name: "single_oversized_item_full_field_set", data: canonicalJSON(canonicalRows(2, 5000)), check: func(t *testing.T, r MirrorResult, m map[string]json.RawMessage) {
			for _, k := range []string{"items", "count", "returned_count", "truncated", "oversized", "max_bytes", "item_bytes", "oversized_item_id", "oversized_preview", "next_cursor", "note"} {
				if _, ok := m[k]; !ok {
					t.Fatal(k)
				}
			}
			if !canonicalBool(t, m, "oversized") || canonicalString(t, m, "oversized_item_id") != "1000000001" || canonicalInt(t, m, "returned_count") != 0 {
				t.Fatal(r)
			}
			c, e := DecodeMirrorCursor(canonicalString(t, m, "next_cursor"), "market_demand", ArgsHash(nil))
			if e != nil || c.O != 1 {
				t.Fatal(c, e)
			}
		}},
		{name: "oversized_last_item_omits_next_cursor", data: canonicalJSON(canonicalRows(1, 5000)), check: func(t *testing.T, r MirrorResult, m map[string]json.RawMessage) {
			if !canonicalBool(t, m, "oversized") {
				t.Fatal(r)
			}
			if _, ok := m["next_cursor"]; ok {
				t.Fatal("last cursor")
			}
			if !strings.Contains(canonicalString(t, m, "note"), "This item exceeded the budget. The result is otherwise complete.") {
				t.Fatal(r)
			}
		}},
		{name: "sibling_object_verbatim_and_note_owned_moves_guidance_to_pp_note", data: canonicalJSON(map[string]any{"results": rows, "note": "linen apron", "sibling": map[string]any{"shop": "Juniper Ceramics", "shop_id": 9000001}}), check: func(t *testing.T, r MirrorResult, m map[string]json.RawMessage) {
			if canonicalString(t, m, "note") != "linen apron" || !strings.Contains(canonicalString(t, m, "_pp_note"), "same arguments") || !bytes.Equal(m["sibling"], canonicalJSON(map[string]any{"shop": "Juniper Ceramics", "shop_id": 9000001})) {
				t.Fatal(r)
			}
		}},
		{name: "count_matches_array_length_kept", data: canonicalJSON(map[string]any{"results": rows, "count": 30}), check: func(t *testing.T, r MirrorResult, m map[string]json.RawMessage) {
			if canonicalInt(t, m, "count") != 30 {
				t.Fatal(r)
			}
			if _, ok := m["_pp_count"]; ok {
				t.Fatal("moved matching count")
			}
		}},
		{name: "count_mismatch_relocated_to_pp_count", data: canonicalJSON(map[string]any{"results": rows, "count": 55}), check: func(t *testing.T, r MirrorResult, m map[string]json.RawMessage) {
			if canonicalInt(t, m, "count") != 30 || canonicalInt(t, m, "_pp_count") != 55 {
				t.Fatal(r)
			}
		}},
		{name: "canonical_field_collision_relocates_to_pp_key", data: canonicalJSON(map[string]any{"results": rows, "truncated": "wool scarf", "next_cursor": "linen apron", "max_bytes": 7, "oversized": 9, "returned_count": 11, "item_bytes": 13, "oversized_item_id": "ceramic mug", "oversized_preview": "linen apron"}), check: func(t *testing.T, r MirrorResult, m map[string]json.RawMessage) {
			for _, k := range []string{"truncated", "next_cursor", "max_bytes", "oversized", "returned_count", "item_bytes", "oversized_item_id", "oversized_preview"} {
				if _, ok := m["_pp_"+k]; !ok {
					t.Fatal(k)
				}
			}
		}},
		{name: "supports_cursor_false_truncated_page_note_cannot_be_paged", data: list, req: MirrorRequest{Narrow: "Narrow with the `top` parameter.", NoRerun: ""}, check: func(t *testing.T, r MirrorResult, m map[string]json.RawMessage) {
			if _, ok := m["next_cursor"]; ok {
				t.Fatal("unexpected cursor")
			}
			if canonicalString(t, m, "note") != "This output exceeded the budget. The rest cannot be paged with a cursor. Narrow with the `top` parameter." {
				t.Fatal(r)
			}
		}},
		{name: "no_rerun_note_verbatim", data: list, req: MirrorRequest{NoRerun: "Do not re-run; read stored results."}, check: func(t *testing.T, r MirrorResult, m map[string]json.RawMessage) {
			if canonicalString(t, m, "note") != "Do not re-run; read stored results." {
				t.Fatal(r)
			}
		}},
		{name: "cursor_union_pages_concatenate_deep_equal_in_order", data: list, check: func(t *testing.T, r MirrorResult, m map[string]json.RawMessage) {
			var union []map[string]any
			for i := 0; i < 40; i++ {
				var page []map[string]any
				_ = json.Unmarshal(m["items"], &page)
				union = append(union, page...)
				if !canonicalBool(t, m, "truncated") {
					break
				}
				c, e := DecodeMirrorCursor(canonicalString(t, m, "next_cursor"), "market_demand", ArgsHash(nil))
				if e != nil {
					t.Fatal(e)
				}
				r = Mirror(list, MirrorRequest{Tool: "market_demand", ArgsHash: ArgsHash(nil), SupportsCursor: true, Cursor: c})
				m = canonicalMap(t, r, Budget())
			}
			var want []map[string]any
			_ = json.Unmarshal(list, &want)
			if !reflect.DeepEqual(union, want) {
				t.Fatal("page union differs")
			}
		}},
		{name: "bare_array_gives_items", data: list, check: func(t *testing.T, r MirrorResult, m map[string]json.RawMessage) {
			if _, ok := m["items"]; !ok {
				t.Fatal(r)
			}
			if _, ok := m["results"]; ok {
				t.Fatal(r)
			}
		}},
		{name: "object_wrapped_array_keeps_results", data: canonicalJSON(map[string]any{"results": rows}), check: func(t *testing.T, r MirrorResult, m map[string]json.RawMessage) {
			if _, ok := m["results"]; !ok {
				t.Fatal(r)
			}
			if _, ok := m["items"]; ok {
				t.Fatal(r)
			}
		}},
		{name: "budget_floor_clamp", data: list, check: func(t *testing.T, r MirrorResult, m map[string]json.RawMessage) {
			if Budget() != 2048 || canonicalInt(t, m, "max_bytes") != 2048 {
				t.Fatal(r)
			}
		}},
	}
	middle := canonicalRows(3, 20)
	middle[1]["title"] = strings.Repeat("a", 5000)
	middleData := canonicalJSON(middle)
	middleCursor := &MirrorCursor{K: "m", V: 2, T: "market_demand", A: ArgsHash(nil), D: mirrorDigest(middleData), O: 1}
	cases = append(cases,
		testCase{name: "oversized_middle_item_cursor_offset_one_next_is_two", data: middleData, req: MirrorRequest{Cursor: middleCursor}, check: func(t *testing.T, r MirrorResult, m map[string]json.RawMessage) {
			c, err := DecodeMirrorCursor(canonicalString(t, m, "next_cursor"), "market_demand", ArgsHash(nil))
			if err != nil || c == nil || c.O != 2 {
				t.Fatal(c, err)
			}
			note := canonicalString(t, m, "note")
			if !strings.Contains(note, "Pass next_cursor back as cursor") || strings.Contains(note, "otherwise complete") {
				t.Fatal(note)
			}
		}},
		testCase{name: "oversized_middle_item_union_paging", data: middleData, check: func(t *testing.T, r MirrorResult, m map[string]json.RawMessage) {
			var union []map[string]any
			var oversized []string
			for page := 0; page < 10; page++ {
				m = canonicalMap(t, r, Budget())
				var items []map[string]any
				if err := json.Unmarshal(m["items"], &items); err != nil {
					t.Fatal(err)
				}
				union = append(union, items...)
				if canonicalBool(t, m, "oversized") {
					oversized = append(oversized, canonicalString(t, m, "oversized_item_id"))
				}
				if _, ok := m["next_cursor"]; !ok {
					if !bytes.Equal(canonicalJSON(union), canonicalJSON([]map[string]any{middle[0], middle[2]})) || !reflect.DeepEqual(oversized, []string{"1000000002"}) {
						t.Fatal(union, oversized)
					}
					return
				}
				c, err := DecodeMirrorCursor(canonicalString(t, m, "next_cursor"), "market_demand", ArgsHash(nil))
				if err != nil {
					t.Fatal(err)
				}
				r = Mirror(middleData, MirrorRequest{Tool: "market_demand", ArgsHash: ArgsHash(nil), SupportsCursor: true, Cursor: c})
			}
			t.Fatal("paging did not finish")
		}},
	)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "budget_floor_clamp" {
				t.Setenv("PP_MCP_MAX_BYTES", "1")
				refreshBudgetForTest()
			}
			r := tc.req
			r.Tool = "market_demand"
			r.ArgsHash = ArgsHash(nil)
			if tc.name != "supports_cursor_false_truncated_page_note_cannot_be_paged" && tc.name != "no_rerun_note_verbatim" {
				r.SupportsCursor = true
			}
			result := Mirror(tc.data, r)
			if len(result.Text) > Budget() {
				t.Fatalf("%d bytes", len(result.Text))
			}
			var m map[string]json.RawMessage
			if result.Text != "" && (result.Text != string(tc.data) || r.Cursor != nil) {
				m = canonicalMap(t, result, Budget())
			}
			if result.Event == "preview" && !strings.HasPrefix(canonicalString(t, m, "note"), "This output exceeded the budget and cannot be paged.") {
				t.Fatal("incorrect preview guidance", result)
			}
			if result.Event == "preview" || tc.name == "oversized_last_item_omits_next_cursor" {
				if strings.Contains(canonicalString(t, m, "note"), "next_cursor") {
					t.Fatal("unusable cursor guidance", result)
				}
			}
			noteKey := "note"
			if _, ok := m["_pp_note"]; ok {
				noteKey = "_pp_note"
			}
			if result.Event == "truncated" && r.SupportsCursor && !strings.Contains(canonicalString(t, m, noteKey), "Pass next_cursor back as cursor together with the same arguments.") {
				t.Fatal("missing cursor guidance", result)
			}
			if tc.check != nil {
				tc.check(t, result, m)
			} else {
				if result.Event != "preview" || !canonicalBool(t, m, "truncated") {
					t.Fatal(result)
				}
				if _, ok := m["preview"]; !ok {
					t.Fatal(result)
				}
			}
		})
	}
}

func TestMirrorCursorErrors(t *testing.T) {
	data := canonicalJSON(canonicalRows(2, 50))
	hash := ArgsHash([]string{"--top", "2"})
	valid := MirrorCursor{K: "m", V: 2, T: "market_demand", A: hash, D: mirrorDigest(data)}
	t.Run("valid_cursor_at_cap", func(t *testing.T) {
		c := valid
		// Pick the largest valid encoding that fits the cap.
		for len(EncodeMirrorCursor(c)) <= 1024 {
			c.D += "a"
		}
		c.D = c.D[:len(c.D)-1]
		raw := EncodeMirrorCursor(c)
		if len(raw) != 1024 {
			t.Fatal("control not at cap", len(raw))
		}
		decoded, err := DecodeMirrorCursor(raw, c.T, hash)
		if err != nil || decoded == nil || *decoded != c {
			t.Fatal(decoded, err)
		}
	})
	cases := []struct{ name, raw, code string }{
		{"malformed", "!", "cursor_invalid"}, {"oversize_valid_cursor", EncodeMirrorCursor(MirrorCursor{K: "m", V: 2, T: valid.T, A: hash, D: strings.Repeat("a", 1100)}), "cursor_invalid"},
	}
	for _, v := range []struct {
		name   string
		change func(*MirrorCursor)
		code   string
	}{
		{"wrong_k", func(c *MirrorCursor) { c.K = "e" }, "cursor_invalid"},
		{"wrong_v", func(c *MirrorCursor) { c.V = 3 }, "cursor_invalid"},
		{"typed_v1", func(c *MirrorCursor) { c.V = 1 }, "cursor_invalid"},
		{"wrong_tool_before_args", func(c *MirrorCursor) { c.T = "market_shops"; c.A = "wrong" }, "cursor_wrong_tool"},
		{"wrong_args", func(c *MirrorCursor) { c.A = "wrong" }, "cursor_args_mismatch"},
	} {
		c := valid
		v.change(&c)
		cases = append(cases, struct{ name, raw, code string }{v.name, base64.RawURLEncoding.EncodeToString(canonicalJSON(c)), v.code})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, e := DecodeMirrorCursor(tc.raw, valid.T, hash)
			if e == nil || e.Code != tc.code {
				t.Fatal(e)
			}
		})
	}
	for _, o := range []int{-1, 2} {
		t.Run("offset_"+strconv.Itoa(o+1), func(t *testing.T) {
			c := valid
			c.O = o
			r := Mirror(data, MirrorRequest{Cursor: &c})
			if !r.IsError || !strings.HasPrefix(r.Text, "cursor_invalid:") {
				t.Fatal(r)
			}
		})
	}
	t.Run("stale_digest_checked_before_offset", func(t *testing.T) {
		c := valid
		c.O = -1
		c.D = "wrong"
		r := Mirror(data, MirrorRequest{Cursor: &c})
		want := "cursor_stale: the results changed since the first page (store write or time-dependent fields); re-run from the first page without `cursor`."
		if !r.IsError || r.Text != want {
			t.Fatal(r)
		}
	})
	t.Run("stale_digest_with_valid_offset", func(t *testing.T) {
		c := valid
		c.D = "wrong"
		if r := Mirror(data, MirrorRequest{Cursor: &c}); !r.IsError || !strings.HasPrefix(r.Text, "cursor_stale:") {
			t.Fatal(r)
		}
	})
	if c, e := DecodeMirrorCursor("", valid.T, hash); c != nil || e != nil {
		t.Fatal(c, e)
	}
	c2, e := DecodeMirrorCursor(EncodeMirrorCursor(valid), valid.T, hash)
	if e != nil || !reflect.DeepEqual(c2, &valid) {
		t.Fatal(c2, e)
	}
	if ArgsHash([]string{"ab", "c"}) == ArgsHash([]string{"a", "bc"}) || ArgsHash(nil) == ArgsHash([]string{""}) || len(hash) != 16 {
		t.Fatal("ambiguous hash")
	}
}

func TestMirrorEventsAndOverflow(t *testing.T) {
	var log bytes.Buffer
	old := mirrorEventWriter
	mirrorEventWriter = &log
	t.Cleanup(func() { mirrorEventWriter = old })
	for _, tc := range []struct {
		name      string
		data      []byte
		overflow  bool
		trueBytes int
	}{
		{"truncated", canonicalJSON(canonicalRows(50, 100)), false, 0},
		{"oversized", canonicalJSON(canonicalRows(1, 5000)), false, 0},
		{"preview", []byte(strings.Repeat("€", 2000)), true, MirrorCaptureMax + 100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			log.Reset()
			original := tc.trueBytes
			if original == 0 {
				original = len(tc.data)
			}
			r := Mirror(tc.data, MirrorRequest{Tool: "market_demand", MaxBytes: 2048, Overflow: tc.overflow, TrueBytes: tc.trueBytes})
			canonicalMap(t, r, 2048)
			want := fmt.Sprintf("pp-mcp-bound event=%s method=market_demand original_bytes=%d max_bytes=2048\n", tc.name, original)
			if log.String() != want || r.Event != tc.name {
				t.Fatal(log.String(), r.Event)
			}
			if tc.overflow {
				m := canonicalMap(t, r, 2048)
				if canonicalInt(t, m, "original_bytes") != original {
					t.Fatal(r)
				}
			}
		})
	}
}

func TestBudgetValuesAndClampWarning(t *testing.T) {
	for _, row := range []struct {
		value    string
		warnings int
	}{{"", 0}, {"abc", 1}, {"0", 1}, {"-1", 1}} {
		t.Run("default_"+row.value, func(t *testing.T) {
			budgetInvalidOnce = sync.Once{}
			budgetNonpositiveOnce = sync.Once{}
			old := os.Stderr
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			os.Stderr = w
			defer func() { os.Stderr = old; r.Close(); w.Close() }()
			t.Setenv("PP_MCP_MAX_BYTES", row.value)
			a, b := Budget(), Budget()
			w.Close()
			os.Stderr = old
			out, _ := io.ReadAll(r)
			if a != 32000 || b != 32000 || strings.Count(string(out), "PP_MCP_MAX_BYTES") != row.warnings {
				t.Fatal(a, b, string(out))
			}
		})
	}
	t.Setenv("PP_MCP_MAX_BYTES", " 4000 \n")
	if Budget() != 4000 {
		t.Fatal(Budget())
	}
	budgetClampOnce = sync.Once{}
	t.Cleanup(func() { budgetClampOnce = sync.Once{} })
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	t.Setenv("PP_MCP_MAX_BYTES", "1")
	a, b := Budget(), Budget()
	_ = w.Close()
	os.Stderr = old
	out, _ := io.ReadAll(r)
	_ = r.Close()
	if a != 2048 || b != 2048 || strings.Count(string(out), "clamped") != 1 {
		t.Fatal(a, b, string(out))
	}
}
func TestTypedBudgetSubprocess(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestTypedBudgetHelper$")
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "PP_MCP_MAX_BYTES=") && !strings.HasPrefix(e, "GO_WANT_HELPER_PROCESS=") {
			cmd.Env = append(cmd.Env, e)
		}
	}
	cmd.Env = append(cmd.Env, "PP_MCP_MAX_BYTES=4000", "GO_WANT_HELPER_PROCESS=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	t.Setenv("PP_MCP_MAX_BYTES", "")
	if err := os.Unsetenv("PP_MCP_MAX_BYTES"); err != nil {
		t.Fatal(err)
	}
	if Budget() != 32000 {
		t.Fatal(Budget())
	}
}
func TestTypedBudgetHelper(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	if len(EndpointResponse("GET", canonicalJSON(canonicalRows(100, 160)))) > 4000 {
		t.Fatal("typed GET response exceeded environment budget")
	}
	if MaxBytes != 4000 || len(Text(strings.Repeat("x", 10000))) > 4000 {
		t.Fatal("typed budget not initialized from environment")
	}
}
func TestTextWithin(t *testing.T) {
	for _, limit := range []int{0, 1, 2, 100, 2048} {
		s := TextWithin(strings.Repeat("€", 5000), limit, 0)
		if len(s) > limit {
			t.Fatal(limit, len(s))
		}
		if limit >= 2048 && !json.Valid([]byte(s)) {
			t.Fatal(s)
		}
	}
	if TextWithin("linen apron", 100, 0) != "linen apron" {
		t.Fatal("changed fitting text")
	}
}

func TestMirrorReserveDegeneratesSafely(t *testing.T) {
	for _, reserve := range []int{2008, 2046, 2047, 2048, 3000} {
		r := Mirror(canonicalJSON(canonicalRows(30, 160)), MirrorRequest{MaxBytes: 2048, Reserve: reserve})
		limit := 2048 - reserve
		if limit < 0 {
			limit = 0
		}
		if len(r.Text) > limit || r.Text == "{}" {
			t.Fatal(reserve, len(r.Text))
		}
		if limit == 40 && !strings.Contains(r.Text, `"truncated":true`) {
			t.Fatal(r.Text)
		}
		if limit < 30 && r.Text != "" {
			t.Fatal(r.Text)
		}
	}
}
func TestTypedSmallGETShapeUnchanged(t *testing.T) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(EndpointResponse("GET", canonicalJSON(canonicalRows(2, 10)))), &m); err != nil {
		t.Fatal(err)
	}
	if canonicalInt(t, m, "count") != 2 {
		t.Fatal(m)
	}
	if _, ok := m["items"]; !ok {
		t.Fatal(m)
	}
	if _, ok := m["results"]; ok {
		t.Fatal(m)
	}
}

// Regression mutations: remove the ceiling/default branches, restore prefix-wide
// UTF-8 validation, restore empty-object clamps, or prefer name over receipt_id.
func TestHardeningBudget(t *testing.T) {
	for _, row := range []struct {
		name, value string
		want        int
		warnings    int
	}{
		{"huge", "999999999999999999999999999999", MirrorCaptureMax, 1},
		{"MaxInt64", "9223372036854775807", MirrorCaptureMax, 1},
		{"non_numeric", "60k", DefaultMaxBytes, 1},
		{"bare_plus", "+", DefaultMaxBytes, 1},
		{"leading_plus", "+20000", 20000, 0},
		{"plus_overflow", "+999999999999999999999999999999", MirrorCaptureMax, 1},
		{"ceiling_boundary", "16777216", MirrorCaptureMax, 0},
		{"above_ceiling", "16777217", MirrorCaptureMax, 1},
		{"zero", "0", 32000, 1},
		{"negative", "-5", 32000, 1},
	} {
		t.Run(row.name, func(t *testing.T) {
			budgetCeilingOnce = sync.Once{}
			budgetInvalidOnce = sync.Once{}
			budgetNonpositiveOnce = sync.Once{}
			old := os.Stderr
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			os.Stderr = w
			defer func() { os.Stderr = old; r.Close(); w.Close() }()
			t.Setenv("PP_MCP_MAX_BYTES", row.value)
			a, b := Budget(), Budget()
			w.Close()
			os.Stderr = old
			out, _ := io.ReadAll(r)
			if (row.name == "zero" || row.name == "negative") && string(out) != "PP_MCP_MAX_BYTES must be positive; using default 32000\n" {
				t.Fatalf("warning=%q", out)
			}
			if row.name == "plus_overflow" && string(out) != "PP_MCP_MAX_BYTES clamped to maximum 16777216\n" {
				t.Fatalf("warning=%q", out)
			}
			if a != row.want || b != row.want || strings.Count(string(out), "PP_MCP_MAX_BYTES") != row.warnings {
				t.Fatalf("%d %d %q", a, b, out)
			}
		})
	}
}
func TestHardeningInvalidUTF8Fragment(t *testing.T) {
	data := bytes.Repeat([]byte("a"), 2<<20)
	data[len(data)/4] = 0xff
	start := time.Now()
	out := mirrorPreview(data, MirrorRequest{MaxBytes: 2048}, len(data))
	if len(out) > 2048 || time.Since(start) >= time.Second {
		t.Fatalf("bytes=%d elapsed=%s", len(out), time.Since(start))
	}
}
func TestHardeningMinimalClamp(t *testing.T) {
	for _, limit := range []int{2, 40} {
		for _, out := range []string{
			TextWithin(strings.Repeat("a", 1000), limit, 0),
			Mirror(bytes.Repeat([]byte("a"), 1000), MirrorRequest{MaxBytes: 2048, Reserve: 2048 - limit}).Text,
		} {
			if len(out) > limit || out == "{}" {
				t.Fatal(out)
			}
			if limit == 40 && !strings.Contains(out, `"truncated":true`) {
				t.Fatal(out)
			}
			if limit == 2 && out != "" {
				t.Fatal(out)
			}
		}
	}
}
func TestHardeningReceiptOversizedID(t *testing.T) {
	for _, row := range []struct {
		name string
		ids  map[string]any
	}{
		{"receipt_id", map[string]any{"receipt_id": 1000000001}},
		{"transaction_id", map[string]any{"transaction_id": 1000000001}},
		{"payment_id", map[string]any{"payment_id": 1000000001}},
		{"entry_id", map[string]any{"entry_id": 1000000001}},
		{"listing_before_transaction", map[string]any{"listing_id": 1000000001, "transaction_id": 1000000002}},
	} {
		t.Run(row.name, func(t *testing.T) {
			row.ids["name"] = "Synthetic Buyer"
			row.ids["text"] = strings.Repeat("a", 10000)
			out := Mirror(canonicalJSON([]map[string]any{row.ids}), MirrorRequest{MaxBytes: 2048})
			var m map[string]any
			if json.Unmarshal([]byte(out.Text), &m) != nil || m["oversized_item_id"] != "1000000001" {
				t.Fatal(m)
			}
		})
	}
}

func TestStaleGuidanceScope(t *testing.T) {
	for _, row := range []struct {
		name     string
		snapshot bool
	}{{"without_snapshot", false}, {"with_snapshot", true}} {
		t.Run(row.name, func(t *testing.T) {
			r := Mirror([]byte(`[]`), MirrorRequest{SnapshotParam: row.snapshot, Cursor: &MirrorCursor{D: "changed"}})
			if !r.IsError || !strings.Contains(r.Text, "without `cursor`") || strings.Contains(r.Text, "snapshot") != row.snapshot {
				t.Fatal(r)
			}
			if row.snapshot && (!strings.Contains(r.Text, "`results.snapshot_id`") || !strings.Contains(r.Text, "without `select`")) {
				t.Fatal(r)
			}
		})
	}
}
func TestTextWithinReserveReportsFullBudget(t *testing.T) {
	for _, reserve := range []int{50, 2008} {
		out := TextWithin(strings.Repeat("a", 5000), 2048, reserve)
		var m map[string]any
		if len(out)+reserve > 2048 || json.Unmarshal([]byte(out), &m) != nil || m["max_bytes"] != float64(2048) {
			t.Fatal(out)
		}
	}
}

func TestNoCursorOversizedGuidance(t *testing.T) {
	data := canonicalJSON([]map[string]any{{"listing_id": 1000000001, "title": strings.Repeat("linen apron", 1000)}, {"listing_id": 1000000002, "title": "ceramic mug"}})
	r := Mirror(data, MirrorRequest{MaxBytes: 2048, Narrow: "Narrow with `select` parameters."})
	var m map[string]any
	if json.Unmarshal([]byte(r.Text), &m) != nil || m["oversized"] != true || m["next_cursor"] != nil {
		t.Fatal(r)
	}
	note, _ := m["note"].(string)
	if !strings.Contains(note, "The rest cannot be paged with a cursor.") || strings.Contains(note, "next_cursor") {
		t.Fatal(note)
	}
}
