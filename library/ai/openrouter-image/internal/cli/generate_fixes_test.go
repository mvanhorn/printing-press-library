// Copyright 2026 neal-kyle and contributors. Licensed under Apache-2.0. See LICENSE.
// Regression tests for review fixes: SSE stream parsing (all documented
// event shapes), collision-resistant ledger ids, and multi-image output
// handling.

package cli

import (
	"strings"
	"testing"
)

func TestParseImagesResponsePlainJSON(t *testing.T) {
	body := []byte(`{"created":1723000000,"data":[{"b64_json":"aGVsbG8=","media_type":"image/png"}],"usage":{"cost":0.02,"total_tokens":10}}`)
	resp, err := parseImagesResponse(body, false)
	if err != nil {
		t.Fatalf("parseImagesResponse(plain) error = %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("want 1 image, got %d", len(resp.Data))
	}
	if resp.Data[0].B64JSON != "aGVsbG8=" {
		t.Fatalf("b64 mismatch: %q", resp.Data[0].B64JSON)
	}
	if resp.Usage == nil || resp.Usage.Cost != 0.02 {
		t.Fatalf("usage not parsed: %+v", resp.Usage)
	}
}

// TestParseImagesResponseSSETopLevel covers the documented
// image_generation.partial_image / image_generation.completed events, which
// carry b64_json at the top level of the SSE payload.
func TestParseImagesResponseSSETopLevel(t *testing.T) {
	body := []byte(strings.Join([]string{
		"event: message",
		`data: {"type":"image_generation.partial_image","partial_image_index":0,"b64_json":"aGVs"}`,
		"",
		"event: message",
		`data: {"type":"image_generation.partial_image","partial_image_index":0,"b64_json":"bG8="}`,
		"",
		"event: message",
		`data: {"type":"image_generation.partial_image","partial_image_index":1,"b64_json":"d29y","media_type":"image/webp"}`,
		"",
		"event: message",
		`data: {"type":"image_generation.completed","partial_image_index":1,"b64_json":"ZmluYWwx","media_type":"image/webp","created":1748372400,"usage":{"cost":0.03,"total_tokens":15}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n"))
	resp, err := parseImagesResponse(body, true)
	if err != nil {
		t.Fatalf("parseImagesResponse(sse) error = %v", err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("want 2 images, got %d", len(resp.Data))
	}
	// Fragments for index 0 must be concatenated in arrival order.
	if resp.Data[0].B64JSON != "aGVsbG8=" {
		t.Fatalf("fragment concat mismatch: %q", resp.Data[0].B64JSON)
	}
	// The completed event for index 1 supersedes its partial fragment.
	if resp.Data[1].B64JSON != "ZmluYWwx" {
		t.Fatalf("completed event did not supersede partial: %q", resp.Data[1].B64JSON)
	}
	if resp.Data[1].MediaType != "image/webp" {
		t.Fatalf("media type mismatch: %q", resp.Data[1].MediaType)
	}
	if resp.Usage == nil || resp.Usage.Cost != 0.03 {
		t.Fatalf("usage not merged from completed event: %+v", resp.Usage)
	}
	if resp.Created != 1748372400 {
		t.Fatalf("created not merged: %d", resp.Created)
	}
}

// TestParseImagesResponseSSEPartialImageB64 covers the Responses-style
// response.image_generation_call.partial_image event shape.
func TestParseImagesResponseSSEPartialImageB64(t *testing.T) {
	body := []byte(strings.Join([]string{
		`data: {"type":"response.image_generation_call.in_progress","item_id":"call-123","sequence_number":1}`,
		"",
		`data: {"type":"response.image_generation_call.partial_image","item_id":"call-123","output_index":0,"partial_image_index":0,"partial_image_b64":"aGVsbG8=","sequence_number":3}`,
		"",
		`data: {"type":"response.image_generation_call.completed","item_id":"call-123","output_index":0,"sequence_number":4}`,
		"",
		"data: [DONE]",
		"",
	}, "\n"))
	resp, err := parseImagesResponse(body, true)
	if err != nil {
		t.Fatalf("parseImagesResponse(sse) error = %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("want 1 image, got %d", len(resp.Data))
	}
	if resp.Data[0].B64JSON != "aGVsbG8=" {
		t.Fatalf("partial_image_b64 not captured: %q", resp.Data[0].B64JSON)
	}
}

// TestParseImagesResponseSSENestedObject covers the ImageStreamingResponse
// shape, which wraps the event payload in a nested data object.
func TestParseImagesResponseSSENestedObject(t *testing.T) {
	body := []byte(strings.Join([]string{
		`data: {"data":{"type":"image_generation.partial_image","partial_image_index":0,"b64_json":"aGVsbG8="}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n"))
	resp, err := parseImagesResponse(body, true)
	if err != nil {
		t.Fatalf("parseImagesResponse(sse) error = %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("want 1 image, got %d", len(resp.Data))
	}
	if resp.Data[0].B64JSON != "aGVsbG8=" {
		t.Fatalf("nested object b64 not captured: %q", resp.Data[0].B64JSON)
	}
}

// TestParseImagesResponseSSEError covers the image_generation stream error
// event.
func TestParseImagesResponseSSEError(t *testing.T) {
	body := []byte(`data: {"type":"error","error":{"code":"upstream_error","message":"The upstream provider returned an error","type":"provider_error"}}`)
	_, err := parseImagesResponse(body, true)
	if err == nil {
		t.Fatal("want error for stream error event, got nil")
	}
	if !strings.Contains(err.Error(), "upstream_error") {
		t.Fatalf("error does not surface code: %v", err)
	}
}

func TestParseImagesResponseSSENoData(t *testing.T) {
	body := []byte("data: [DONE]\n\n")
	if _, err := parseImagesResponse(body, true); err == nil {
		t.Fatal("want error for stream with no image data, got nil")
	}
}

func TestNewLedgerIDUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		id := newLedgerID("openai/gpt-image-1")
		if seen[id] {
			t.Fatalf("ledger id collided: %s", id)
		}
		seen[id] = true
		if !strings.HasPrefix(id, "gen-") {
			t.Fatalf("ledger id missing gen- prefix: %s", id)
		}
	}
}

func TestSsePayloads(t *testing.T) {
	body := []byte("event: message\ndata: {\"a\":1}\n\nevent: message\ndata: {\"b\":2}\n\ndata: [DONE]\n")
	payloads := ssePayloads(body)
	if len(payloads) != 3 {
		t.Fatalf("want 3 payloads, got %d: %v", len(payloads), payloads)
	}
	if payloads[0] != `{"a":1}` || payloads[2] != "[DONE]" {
		t.Fatalf("payload mismatch: %v", payloads)
	}
}
