// Copyright 2026 giuliano-giacaglia. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestNormalizeNodeID(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"colon passthrough", "1234:5678", "1234:5678"},
		{"hyphen converted", "1234-5678", "1234:5678"},
		{"chain with mixed separators preserved", "I5666:180910;1:10515", "I5666:180910;1:10515"},
		{"chain with hyphens converted", "I5666-180910;1-10515", "I5666:180910;1:10515"},
		{"empty", "", ""},
		{"whitespace trimmed", "  1234-5678  ", "1234:5678"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeNodeID(tc.in)
			if got != tc.want {
				t.Errorf("normalizeNodeID(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeNodeIDList(t *testing.T) {
	got := normalizeNodeIDList([]string{"1234-5678", "I5666-180910;1-10515"})
	want := "1234:5678,I5666:180910;1:10515"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
