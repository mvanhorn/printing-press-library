// Copyright 2026 Dickie and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Tests for the hand-authored analytics helpers.

package cli

import (
	"testing"
	"time"
)

func TestAdjustSettlement(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		holidays map[string]bool
		want     string
	}{
		{"weekday no change", "2026-06-17", nil, "2026-06-17"}, // Wed
		{"saturday rolls to monday", "2026-06-20", nil, "2026-06-22"},
		{"sunday rolls to monday", "2026-06-21", nil, "2026-06-22"},
		{"monday holiday rolls to tuesday", "2026-06-22", map[string]bool{"2026-06-22": true}, "2026-06-23"},
		{"saturday past monday holiday to tuesday", "2026-06-20", map[string]bool{"2026-06-22": true}, "2026-06-23"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in, _ := time.Parse("2006-01-02", tt.in)
			got := adjustSettlement(in, tt.holidays).Format("2006-01-02")
			if got != tt.want {
				t.Fatalf("adjustSettlement(%s) = %s, want %s", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseStoredDate(t *testing.T) {
	tests := []struct {
		in string
		ok bool
	}{
		{"2026-06-20", true},
		{"2026-06-20T15:04:05Z", true},
		{"2026-06-20 15:04:05", true},
		{"", false},
		{"not-a-date", false},
	}
	for _, tt := range tests {
		_, ok := parseStoredDate(tt.in)
		if ok != tt.ok {
			t.Fatalf("parseStoredDate(%q) ok=%v, want %v", tt.in, ok, tt.ok)
		}
	}
}

func TestParseShareLimit(t *testing.T) {
	tests := []struct {
		in     string
		want   float64
		hasVal bool
	}{
		{"10%", 0.10, true},
		{"10", 0.10, true},
		{"0.1", 0.10, true},
		{"", 0, false},
		{"  25 % ", 0.25, true},
	}
	for _, tt := range tests {
		got, has := parseShareLimit(tt.in)
		if has != tt.hasVal {
			t.Fatalf("parseShareLimit(%q) has=%v, want %v", tt.in, has, tt.hasVal)
		}
		if has && (got < tt.want-1e-9 || got > tt.want+1e-9) {
			t.Fatalf("parseShareLimit(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestTokenURLFromBase(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"https://api.treasuryspring.com/api/v1", "https://api.treasuryspring.com/oauth/token"},
		{"https://api.sandbox.treasuryspring.com/api/v1", "https://api.sandbox.treasuryspring.com/oauth/token"},
		{"", "https://api.treasuryspring.com/oauth/token"},
	}
	for _, tt := range tests {
		if got := tokenURLFromBase(tt.in); got != tt.want {
			t.Fatalf("tokenURLFromBase(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
}
