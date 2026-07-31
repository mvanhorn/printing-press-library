// Copyright 2026 Adrian Horning and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for the amend-2026-07-29 compact-payload patch: --agent/--compact must
// not strip the array that IS the response, so the comments commands stop
// reporting success with no data.

package cli

import (
	"encoding/json"
	"testing"
)

// decodeCompact runs a raw response through compactFields and returns the
// resulting object.
func decodeCompact(t *testing.T, raw string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(compactFields(json.RawMessage(raw)), &out); err != nil {
		t.Fatalf("compactFields returned undecodable JSON: %v", err)
	}
	return out
}

// A comments response is scalar envelope fields plus one array. Stripping the
// array leaves a paid call reporting success with nothing in it.
func TestCompactKeepsCommentsWhenItIsThePayload(t *testing.T) {
	got := decodeCompact(t, `{
		"success": true,
		"credits_charged": 1,
		"cursor": "AQHSkY5v",
		"comments": [
			{"id": "18099528092261230", "text": "first", "child_comment_count": 1},
			{"id": "17884442442656251", "text": "second", "child_comment_count": 0}
		]
	}`)

	items, ok := got["comments"].([]any)
	if !ok {
		t.Fatalf("comments dropped from a comments-only response; got keys %v", keysOf(got))
	}
	if len(items) != 2 {
		t.Errorf("comments = %d items, want 2", len(items))
	}
	if got["success"] != true {
		t.Errorf("envelope field success lost: %v", got["success"])
	}
}

// An empty result set must stay an empty array: "zero comments" and "no
// comments key" are different answers to an agent.
func TestCompactKeepsEmptyCommentsArray(t *testing.T) {
	got := decodeCompact(t, `{"success": true, "comments": []}`)

	items, ok := got["comments"].([]any)
	if !ok {
		t.Fatalf("empty comments array dropped; got keys %v", keysOf(got))
	}
	if len(items) != 0 {
		t.Errorf("comments = %d items, want 0", len(items))
	}
}

// A scalar array beside the payload is envelope garnish, not a competing
// payload: only compactObjectArrayValue-shaped arrays (non-empty, objects)
// may count against the blocked key. Regression pin for the case where
// {"tags":["a"],"comments":[...]} stripped the comments.
func TestCompactKeepsCommentsDespiteScalarArray(t *testing.T) {
	got := decodeCompact(t, `{
		"success": true,
		"tags": ["a", "b"],
		"comments": [{"id": "1", "text": "kept"}]
	}`)

	items, ok := got["comments"].([]any)
	if !ok {
		t.Fatalf("comments dropped because a scalar array was counted as payload; got keys %v", keysOf(got))
	}
	if len(items) != 1 {
		t.Errorf("comments = %d items, want 1", len(items))
	}
}

// An empty unblocked array must not count as a competing payload either:
// {"related":[],"comments":[...]} keeps the comments.
func TestCompactKeepsCommentsDespiteEmptySidecarArray(t *testing.T) {
	got := decodeCompact(t, `{
		"success": true,
		"related": [],
		"comments": [{"id": "1", "text": "kept"}]
	}`)

	items, ok := got["comments"].([]any)
	if !ok {
		t.Fatalf("comments dropped because an empty array was counted as payload; got keys %v", keysOf(got))
	}
	if len(items) != 1 {
		t.Errorf("comments = %d items, want 1", len(items))
	}
}

// The original intent still holds: comments riding along on a post object are
// a sidecar next to that object's own payload array, and stay stripped.
func TestCompactStripsCommentsSidecar(t *testing.T) {
	got := decodeCompact(t, `{
		"id": "DadJkPxMYsI",
		"images": [{"url": "https://example.test/a.jpg"}],
		"comments": [{"id": "1", "text": "sidecar"}]
	}`)

	if _, present := got["comments"]; present {
		t.Errorf("sidecar comments survived alongside a payload array; got keys %v", keysOf(got))
	}
	if _, present := got["images"]; !present {
		t.Errorf("payload array images was dropped; got keys %v", keysOf(got))
	}
}

// Non-array blocked keys are unaffected by the payload rule.
func TestCompactStillStripsDescription(t *testing.T) {
	got := decodeCompact(t, `{"id": "x", "description": "long prose", "comments": [{"id": "1"}]}`)

	if _, present := got["description"]; present {
		t.Errorf("description survived compaction; got keys %v", keysOf(got))
	}
	if _, present := got["comments"]; !present {
		t.Errorf("comments dropped although it is the only array; got keys %v", keysOf(got))
	}
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
