// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"testing"
)

// raws builds a []json.RawMessage from literal JSON fragments; each `{"n":1}` is 7 bytes.
func raws(parts ...string) []json.RawMessage {
	out := make([]json.RawMessage, len(parts))
	for i, p := range parts {
		out[i] = json.RawMessage(p)
	}
	return out
}

func TestPaginateByBytes_AllFitOnePage(t *testing.T) {
	got := PaginateByBytes(raws(`{"a":1}`, `{"b":2}`, `{"c":3}`), 0, 0, 1000)
	if got.Total != 3 || got.Returned != 3 || got.Offset != 0 || got.NextOffset != nil || got.Truncated {
		t.Fatalf("got %+v", got)
	}
}

func TestPaginateByBytes_SpansPages(t *testing.T) {
	// 5 items x 7 bytes; budget 16 fits 2 (7,14); 3rd (21) overflows.
	got := PaginateByBytes(raws(`{"a":1}`, `{"b":2}`, `{"c":3}`, `{"d":4}`, `{"e":5}`), 0, 0, 16)
	if got.Returned != 2 || got.NextOffset == nil || *got.NextOffset != 2 {
		t.Fatalf("got %+v", got)
	}
}

func TestPaginateByBytes_ExactBudgetBoundary(t *testing.T) {
	// budget 14 fits exactly 2 (7+7=14, not > 14).
	got := PaginateByBytes(raws(`{"a":1}`, `{"b":2}`, `{"c":3}`), 0, 0, 14)
	if got.Returned != 2 || got.NextOffset == nil || *got.NextOffset != 2 {
		t.Fatalf("got %+v", got)
	}
}

func TestPaginateByBytes_OversizedSingleItem(t *testing.T) {
	// First item (17 bytes) exceeds budget 10 -> returned alone, truncated, next points past it.
	got := PaginateByBytes(raws(`{"big":"xxxxxxx"}`, `{"a":1}`), 0, 0, 10)
	if got.Returned != 1 || !got.Truncated || got.NextOffset == nil || *got.NextOffset != 1 {
		t.Fatalf("got %+v", got)
	}
}

func TestPaginateByBytes_OffsetMidList(t *testing.T) {
	got := PaginateByBytes(raws(`{"a":1}`, `{"b":2}`, `{"c":3}`, `{"d":4}`, `{"e":5}`), 2, 0, 1000)
	if got.Offset != 2 || got.Returned != 3 || got.NextOffset != nil {
		t.Fatalf("got %+v", got)
	}
}

func TestPaginateByBytes_OffsetPastEnd(t *testing.T) {
	got := PaginateByBytes(raws(`{"a":1}`, `{"b":2}`), 10, 0, 1000)
	if got.Total != 2 || got.Returned != 0 || got.NextOffset != nil {
		t.Fatalf("got %+v", got)
	}
}

func TestPaginateByBytes_LimitCapsCount(t *testing.T) {
	got := PaginateByBytes(raws(`{"a":1}`, `{"b":2}`, `{"c":3}`, `{"d":4}`), 0, 2, 1000)
	if got.Returned != 2 || got.NextOffset == nil || *got.NextOffset != 2 {
		t.Fatalf("got %+v", got)
	}
}

func TestListFromResponse_BareArray(t *testing.T) {
	items, ok := ListFromResponse(json.RawMessage(`[{"a":1},{"b":2}]`), "")
	if !ok || len(items) != 2 {
		t.Fatalf("ok=%v items=%d", ok, len(items))
	}
}

func TestListFromResponse_EnvelopeAutoDetect(t *testing.T) {
	items, ok := ListFromResponse(json.RawMessage(`{"groups":[{"a":1}]}`), "")
	if !ok || len(items) != 1 {
		t.Fatalf("ok=%v items=%d", ok, len(items))
	}
}

func TestListFromResponse_ExplicitField(t *testing.T) {
	items, ok := ListFromResponse(json.RawMessage(`{"groups":[{"a":1},{"b":2}],"meta":1}`), "groups")
	if !ok || len(items) != 2 {
		t.Fatalf("ok=%v items=%d", ok, len(items))
	}
}

func TestListFromResponse_MultipleArraysAmbiguous(t *testing.T) {
	_, ok := ListFromResponse(json.RawMessage(`{"a":[1],"b":[2]}`), "")
	if ok {
		t.Fatalf("expected ok=false for ambiguous multi-array")
	}
}

func TestListFromResponse_SingleObjectPassthrough(t *testing.T) {
	_, ok := ListFromResponse(json.RawMessage(`{"id":5,"name":"x"}`), "")
	if ok {
		t.Fatalf("expected ok=false for non-list object")
	}
}

func TestListFromResponse_NonJSON(t *testing.T) {
	_, ok := ListFromResponse(json.RawMessage(`not json`), "")
	if ok {
		t.Fatalf("expected ok=false for non-JSON")
	}
}

func TestPaginateBody_PaginatesEnvelopeList(t *testing.T) {
	body := json.RawMessage(`{"groups":[{"a":1},{"b":2},{"c":3},{"d":4},{"e":5}]}`)
	out, paginated := PaginateBody(body, 0, 0, 16, "")
	if !paginated {
		t.Fatalf("expected paginated=true")
	}
	var got PageResult
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Total != 5 || got.Returned != 2 || got.NextOffset == nil || *got.NextOffset != 2 {
		t.Fatalf("got %+v", got)
	}
}

func TestPaginateBody_PassthroughForNonList(t *testing.T) {
	out, paginated := PaginateBody(json.RawMessage(`{"id":5}`), 0, 0, 16, "")
	if paginated || out != nil {
		t.Fatalf("expected passthrough: paginated=%v out=%s", paginated, out)
	}
}
