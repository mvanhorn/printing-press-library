// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package config

import "testing"

func TestParseDatacenter(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want string
	}{
		{"valid us6", "placeholder-key-us6", "us6"},
		{"valid us21", "placeholder-key-us21", "us21"},
		{"valid us1", "placeholder-key-us1", "us1"},
		{"no suffix", "deadbeefcafe", ""},
		{"trailing dash, empty dc", "deadbeef-", ""},
		{"too-long dc", "deadbeef-toolongname", ""},
		{"too-short dc", "deadbeef-u", ""},
		{"non-alphanumeric dc", "deadbeef-us@6", ""},
		{"uppercase dc (rejected for safety)", "deadbeef-US6", ""},
		{"empty input", "", ""},
		{"just a dash", "-", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseDatacenter(tt.key); got != tt.want {
				t.Errorf("ParseDatacenter(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}
