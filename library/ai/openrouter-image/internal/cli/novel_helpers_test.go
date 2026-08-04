// Copyright 2026 neal-kyle and contributors. Licensed under Apache-2.0. See LICENSE.
// Behavior tests for the novel-feature helpers: pricing extraction, unit
// normalization, CSV batch parsing, and filename sanitization.

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCheapestOutputUnit(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantCost float64
		wantProv string
		wantUnit string
	}{
		{
			name:     "single endpoint per-image",
			body:     `{"id":"openai/gpt-image-1","endpoints":[{"provider_name":"OpenAI","provider_slug":"openai","pricing":[{"billable":"output_image","unit":"image","cost_usd":0.02}]}]}`,
			wantCost: 0.02, wantProv: "openai", wantUnit: "image",
		},
		{
			name: "multiple providers picks cheapest",
			body: `{"id":"x/y","endpoints":[
				{"provider_slug":"a","pricing":[{"billable":"output_image","unit":"image","cost_usd":0.05}]},
				{"provider_slug":"b","pricing":[{"billable":"output_image","unit":"image","cost_usd":0.01}]}]}`,
			wantCost: 0.01, wantProv: "b", wantUnit: "image",
		},
		{
			name:     "per-token unit preserved",
			body:     `{"id":"openai/gpt-image-1","endpoints":[{"provider_slug":"openai","pricing":[{"billable":"output_image","unit":"token","cost_usd":0.00004}]}]}`,
			wantCost: 0.00004, wantProv: "openai", wantUnit: "token",
		},
		{
			name:     "input-only pricing ignored",
			body:     `{"id":"x/y","endpoints":[{"provider_slug":"a","pricing":[{"billable":"input_reference","unit":"image","cost_usd":0.5}]}]}`,
			wantCost: 0, wantProv: "", wantUnit: "",
		},
		{
			name:     "empty endpoints",
			body:     `{"id":"x/y","endpoints":[]}`,
			wantCost: 0, wantProv: "", wantUnit: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCost, gotProv, gotUnit := cheapestOutputUnit(json.RawMessage(tt.body))
			if gotCost != tt.wantCost {
				t.Errorf("cost = %v, want %v", gotCost, tt.wantCost)
			}
			if gotProv != tt.wantProv {
				t.Errorf("provider = %q, want %q", gotProv, tt.wantProv)
			}
			if gotUnit != tt.wantUnit {
				t.Errorf("unit = %q, want %q", gotUnit, tt.wantUnit)
			}
		})
	}
}

func TestParseBatchCSV(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "batch.csv")
	content := `prompt,model,n,resolution,quality,output
"a red panda",openai/gpt-image-1,2,2K,high,panda.png
"a cat",google/gemini-2.5-flash-image,1,,,cat.png
,,,,
"skip empty model",,
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := parseBatchCSV(path)
	if err != nil {
		t.Fatalf("parseBatchCSV error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	if rows[0].Model != "openai/gpt-image-1" || rows[0].N != 2 || rows[0].Resolution != "2K" || rows[0].Quality != "high" {
		t.Errorf("row 0 = %+v", rows[0])
	}
	if rows[1].Model != "google/gemini-2.5-flash-image" || rows[1].N != 1 {
		t.Errorf("row 1 = %+v", rows[1])
	}
}

func TestParseBatchCSVMissingColumns(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.csv")
	if err := os.WriteFile(path, []byte("prompt,quality\nhi,low\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := parseBatchCSV(path); err == nil {
		t.Fatal("expected error for missing model column")
	}
}

func TestSafeName(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"openai/gpt-image-1", "openai-gpt-image-1"},
		{"bytedance-seed/seedream-4.5", "bytedance-seed-seedream-4.5"},
		{"a b c", "a-b-c"},
		{"weird!name", "weird-name"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := safeName(tt.in); got != tt.want {
			t.Errorf("safeName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestExtFromMediaType(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"image/jpeg", ".jpg"},
		{"image/png", ".png"},
		{"image/webp", ".webp"},
		{"image/svg+xml", ".svg"},
		{"image/gif", ".gif"},
		{"", ".png"},
	}
	for _, tt := range tests {
		if got := extFromMediaType(tt.in); got != tt.want {
			t.Errorf("extFromMediaType(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestNormalizeAny(t *testing.T) {
	raw := json.RawMessage(`{"a":1,"b":[true,null]}`)
	got := normalizeAny(raw)
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("normalizeAny returned %T, want map", got)
	}
	if m["a"].(float64) != 1 {
		t.Errorf("a = %v, want 1", m["a"])
	}
}
