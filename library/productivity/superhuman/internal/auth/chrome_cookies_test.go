// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package auth

import (
	"sort"
	"testing"
)

// TestChromeListAccountsOnAccountsHost exercises every interesting
// path through the googleId-shaped cookie filter: csrf skip, non-digit
// skip, short-digit skip, and the happy path of a 21-digit Google user
// ID. Pure function — no I/O, no real Chrome required.
func TestChromeListAccountsOnAccountsHost(t *testing.T) {
	cases := []struct {
		name    string
		cookies map[string]string
		want    []string
	}{
		{
			name:    "empty input",
			cookies: map[string]string{},
			want:    nil,
		},
		{
			name: "skips csrf",
			cookies: map[string]string{
				"csrf": "any-value",
			},
			want: nil,
		},
		{
			name: "skips non-digit names",
			cookies: map[string]string{
				"csrf":             "x",
				"sess123":          "x",
				"_GA":              "x",
				"some-tracking-id": "x",
			},
			want: nil,
		},
		{
			name: "skips short digit-only names (not a real googleId)",
			cookies: map[string]string{
				"123":             "x", // too short
				"1234567890":      "x", // 10 digits, too short
				"123456789012345": "x", // 15 digits, still too short
			},
			want: nil,
		},
		{
			name: "captures 21-digit google ids",
			cookies: map[string]string{
				"csrf":                  "x",
				"123456789012345678901": "session-one",
				"987654321098765432109": "session-two",
			},
			want: []string{"123456789012345678901", "987654321098765432109"},
		},
		{
			name: "captures 16-digit boundary digit names",
			cookies: map[string]string{
				"csrf":             "x",
				"1234567890123456": "x",
			},
			want: []string{"1234567890123456"},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := ListAccountsOnAccountsHost(tc.cookies)
			sort.Strings(got)
			wantSorted := append([]string(nil), tc.want...)
			sort.Strings(wantSorted)
			if len(got) != len(wantSorted) {
				t.Fatalf("len mismatch: got %v, want %v", got, wantSorted)
			}
			for i := range got {
				if got[i] != wantSorted[i] {
					t.Fatalf("index %d: got %q, want %q (full got=%v want=%v)", i, got[i], wantSorted[i], got, wantSorted)
				}
			}
		})
	}
}

// TestChromeIsPureDigits asserts the small helper used by the cookie
// filter. Negative cases catch sign chars and embedded non-digits.
func TestChromeIsPureDigits(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"0", true},
		{"1234567890", true},
		{"-12345", false},
		{"+12345", false},
		{"12a45", false},
		{"12 45", false},
		{"abc", false},
	}
	for _, tc := range cases {
		if got := isPureDigits(tc.in); got != tc.want {
			t.Fatalf("isPureDigits(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
