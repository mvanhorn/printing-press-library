// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// The documented GET / example: row 3 spells "HELLO WORLD" (codes 8,5,12,12,15,
// 0,23,15,18,12,4) on a 6x22 Flagship board, with layout delivered as a
// JSON-encoded string. See docs.vestaboard.com/docs/read-write-api/endpoints.
const helloWorldLayout = `{"currentMessage":{"layout":"[[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],[0,0,0,0,0,8,5,12,12,15,0,23,15,18,12,4,0,0,0,0,0,0],[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]]","id":"abc-123"}}`

func TestDecodeLayoutStringEncoded(t *testing.T) {
	var resp currentMessageResponse
	if err := json.Unmarshal([]byte(helloWorldLayout), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	grid, err := decodeLayout(resp.CurrentMessage.Layout)
	if err != nil {
		t.Fatalf("decodeLayout: %v", err)
	}
	if len(grid) != 6 || len(grid[2]) != 22 {
		t.Fatalf("expected 6x22 grid, got %dx%d", len(grid), len(grid[0]))
	}
	rendered := renderGrid(grid)
	if !strings.Contains(rendered, "HELLO WORLD") {
		t.Fatalf("rendered grid missing HELLO WORLD:\n%s", rendered)
	}
}

func TestDecodeLayoutBareArray(t *testing.T) {
	// Some boards return layout as a bare 2D array rather than a string.
	grid, err := decodeLayout(json.RawMessage(`[[1,2,3]]`))
	if err != nil {
		t.Fatalf("decodeLayout bare array: %v", err)
	}
	if got := renderGrid(grid); !strings.Contains(got, "ABC") {
		t.Fatalf("expected ABC, got:\n%s", got)
	}
}

func TestGlyphForUnknownCode(t *testing.T) {
	if glyphForCode(255) != "?" {
		t.Fatalf("unknown code should render as ?")
	}
	if glyphForCode(63) != "r" {
		t.Fatalf("color chip 63 should render as r")
	}
}

func TestDecodeLayoutRejectsEmpty(t *testing.T) {
	// A JSON null layout unmarshals to a nil slice without error; it must be
	// rejected rather than rendered as a blank board.
	if _, err := decodeLayout(json.RawMessage(`null`)); err == nil {
		t.Fatalf("expected error for null layout, got nil")
	}
	if _, err := decodeLayout(json.RawMessage(`[]`)); err == nil {
		t.Fatalf("expected error for empty layout, got nil")
	}
}

func TestFilledGlyphDoesNotCollideWithPound(t *testing.T) {
	// Code 39 is Pound "#"; code 71 (Filled) must render as something else so
	// a preview containing both is unambiguous.
	if glyphForCode(39) != "#" {
		t.Fatalf("code 39 should be #")
	}
	if glyphForCode(71) == "#" {
		t.Fatalf("filled (71) must not collide with pound (#)")
	}
}
