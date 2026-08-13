// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
	"time"
)

func TestParseDesignTime(t *testing.T) {
	// The API returns microsecond precision; RFC3339 alone cannot parse it.
	cases := map[string]bool{
		"2026-06-15T06:09:15.000000Z": true, // real API shape (microseconds)
		"2026-06-15T06:09:15Z":        true, // plain RFC3339
		"2026-06-15T06:09:15.5Z":      true, // tenths
		"not-a-time":                  false,
		"":                            false,
	}
	for in, wantOK := range cases {
		if _, ok := parseDesignTime(in); ok != wantOK {
			t.Errorf("parseDesignTime(%q) ok=%v, want %v", in, ok, wantOK)
		}
	}
	// Microsecond timestamp must compare correctly against a cutoff (the bug:
	// a parse failure here made --since silently export everything).
	old, ok := parseDesignTime("2020-01-01T00:00:00.000000Z")
	if !ok || !old.Before(time.Now()) {
		t.Errorf("microsecond timestamp did not parse/compare correctly: ok=%v", ok)
	}
}

func TestDesignImageName(t *testing.T) {
	d := Design{ID: 57628105, UUID: "abc-uuid"}
	tests := []struct {
		name     string
		template string
		i        int
		total    int
		want     string
	}{
		{"default single", "", 0, 1, "57628105"},
		{"default multi disambiguates", "", 0, 2, "57628105-1"},
		{"default multi second", "", 1, 2, "57628105-2"},
		{"n template single", "{n}", 0, 1, "1"},
		{"n template multi no double index", "{n}", 0, 3, "1"},
		{"n template multi second no double index", "{n}", 1, 3, "2"},
		{"id-n template multi", "{id}-{n}", 2, 3, "57628105-3"},
		{"non-n template multi still disambiguates", "{id}", 1, 2, "57628105-2"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := designImageName(tc.template, d, tc.i, tc.total); got != tc.want {
				t.Errorf("designImageName(%q, d, %d, %d) = %q, want %q", tc.template, tc.i, tc.total, got, tc.want)
			}
		})
	}
}
