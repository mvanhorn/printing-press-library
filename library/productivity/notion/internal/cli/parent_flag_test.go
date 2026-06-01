// Copyright 2026 Nikica Jokic and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"reflect"
	"testing"
)

func TestParseParentFlag(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    any
		wantErr bool
	}{
		{
			name:  "data_source_id shorthand",
			input: "data_source_id:11111111-2222-3333-4444-555555555555",
			want:  map[string]any{"type": "data_source_id", "data_source_id": "11111111-2222-3333-4444-555555555555"},
		},
		{
			name:  "database_id shorthand",
			input: "database_id:abc123",
			want:  map[string]any{"type": "database_id", "database_id": "abc123"},
		},
		{
			name:  "page_id shorthand",
			input: "page_id:abc123",
			want:  map[string]any{"type": "page_id", "page_id": "abc123"},
		},
		{
			name:  "block_id shorthand",
			input: "block_id:abc123",
			want:  map[string]any{"type": "block_id", "block_id": "abc123"},
		},
		{
			name:  "workspace shorthand",
			input: "workspace:true",
			want:  map[string]any{"type": "workspace", "workspace": true},
		},
		{
			name:  "raw JSON object still works",
			input: `{"data_source_id":"abc123"}`,
			want:  map[string]any{"data_source_id": "abc123"},
		},
		{
			name:  "raw JSON with type discriminator still works",
			input: `{"type":"page_id","page_id":"abc123"}`,
			want:  map[string]any{"type": "page_id", "page_id": "abc123"},
		},
		{
			name:  "shorthand tolerates surrounding whitespace",
			input: "  page_id : abc123 ",
			want:  map[string]any{"type": "page_id", "page_id": "abc123"},
		},
		{
			name:    "shorthand with empty value errors",
			input:   "page_id:",
			wantErr: true,
		},
		{
			name:    "workspace with non-true value errors",
			input:   "workspace:false",
			wantErr: true,
		},
		{
			name:    "unknown key errors with guidance",
			input:   "nonsense:value",
			wantErr: true,
		},
		{
			name:    "malformed JSON object errors",
			input:   `{"data_source_id":}`,
			wantErr: true,
		},
		{
			name:    "raw JSON array errors",
			input:   `[{"data_source_id":"abc123"}]`,
			wantErr: true,
		},
		{
			name:    "bare value with no separator errors",
			input:   "abc123",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseParentFlag(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseParentFlag(%q) = %v, want error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseParentFlag(%q) unexpected error: %v", tt.input, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseParentFlag(%q) = %#v, want %#v", tt.input, got, tt.want)
			}
		})
	}
}
