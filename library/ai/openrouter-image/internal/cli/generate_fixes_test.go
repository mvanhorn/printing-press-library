// Copyright 2026 neal-kyle and contributors. Licensed under Apache-2.0. See LICENSE.
// Regression tests for review fixes: SSE stream parsing, collision-resistant
// ledger ids, and multi-image output handling.

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

func TestParseImagesResponseSSE(t *testing.T) {
	// Simulated text/event-stream: two fragment events for index 0 plus a
	// usage event, terminated by [DONE].
	body := []byte(strings.Join([]string{
		"event: message",
		`data: {"data":[{"index":0,"b64_json":"aGVs"}]}`,
		"",
		"event: message",
		`data: {"data":[{"index":0,"b64_json":"bG8="}]}`,
		"",
		"event: message",
		`data: {"data":[{"index":1,"b64_json":"d29ybGQ=","media_type":"image/webp"}],"usage":{"cost":0.03,"total_tokens":15}}`,
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
	if resp.Data[1].B64JSON != "d29ybGQ=" {
		t.Fatalf("second image mismatch: %q", resp.Data[1].B64JSON)
	}
	if resp.Data[1].MediaType != "image/webp" {
		t.Fatalf("media type mismatch: %q", resp.Data[1].MediaType)
	}
	if resp.Usage == nil || resp.Usage.Cost != 0.03 {
		t.Fatalf("usage not merged from final event: %+v", resp.Usage)
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
