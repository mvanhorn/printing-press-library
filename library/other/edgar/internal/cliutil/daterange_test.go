// Copyright 2026 magoo242 and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH(amend-efts-date-range): hand-added test — see .printing-press-patches/.

package cliutil

import "testing"

func TestSplitDateRange(t *testing.T) {
	cases := []struct {
		in, wantStart, wantEnd string
	}{
		{"2024-01-01,2026-05-13", "2024-01-01", "2026-05-13"},
		{"2026-07-20,2026-07-21", "2026-07-20", "2026-07-21"},
		{" 2024-01-01 , 2026-05-13 ", "2024-01-01", "2026-05-13"}, // whitespace trimmed
		{"2024-01-01,", "2024-01-01", ""},                         // open-ended end
		{",2026-05-13", "", "2026-05-13"},                         // open-ended start
		{"2024-01-01", "2024-01-01", ""},                          // no comma: start-only
		{"", "", ""},                                              // empty
		{"   ", "", ""},                                           // whitespace-only
	}
	for _, c := range cases {
		start, end := SplitDateRange(c.in)
		if start != c.wantStart || end != c.wantEnd {
			t.Errorf("SplitDateRange(%q) = (%q, %q), want (%q, %q)", c.in, start, end, c.wantStart, c.wantEnd)
		}
	}
}
