// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored tests for the SRAM AXS novel-feature helpers.

package cli

import (
	"encoding/json"
	"testing"
)

func TestGstr(t *testing.T) {
	m := map[string]any{
		"serial":  "ABC123",
		"empty":   "",
		"count":   float64(7),
		"flag":    true,
		"nullval": nil,
	}
	cases := []struct {
		name string
		keys []string
		want string
	}{
		{"first key hit", []string{"serial"}, "ABC123"},
		{"fallback past empty", []string{"empty", "serial"}, "ABC123"},
		{"number coerced", []string{"count"}, "7"},
		{"bool coerced", []string{"flag"}, "true"},
		{"null then hit", []string{"nullval", "serial"}, "ABC123"},
		{"no hit", []string{"missing"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := gstr(m, tc.keys...); got != tc.want {
				t.Fatalf("gstr(%v) = %q, want %q", tc.keys, got, tc.want)
			}
		})
	}
}

func TestDecodeList(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{"bare array", `[{"id":"1"},{"id":"2"}]`, 2},
		{"results envelope", `{"results":[{"id":"1"}]}`, 1},
		{"data envelope", `{"data":[{"id":"1"},{"id":"2"},{"id":"3"}]}`, 3},
		{"empty array", `[]`, 0},
		{"object no list", `{"detail":"nope"}`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := decodeList(json.RawMessage(tc.in))
			if len(got) != tc.want {
				t.Fatalf("decodeList(%s) len = %d, want %d", tc.in, len(got), tc.want)
			}
		})
	}
}

func TestTrimFloat(t *testing.T) {
	cases := map[float64]string{
		7:    "7",
		7.5:  "7.5",
		0:    "0",
		-3:   "-3",
		1.25: "1.25",
	}
	for in, want := range cases {
		if got := trimFloat(in); got != want {
			t.Fatalf("trimFloat(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestParseAXSTime(t *testing.T) {
	cases := []struct {
		in     string
		wantOK bool
	}{
		{"2026-06-14T17:00:00Z", true},
		{"2026-06-14T17:00:00", true},
		{"2026-06-14", true},
		{"not-a-time", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			_, ok := parseAXSTime(tc.in)
			if ok != tc.wantOK {
				t.Fatalf("parseAXSTime(%q) ok = %v, want %v", tc.in, ok, tc.wantOK)
			}
		})
	}
}
