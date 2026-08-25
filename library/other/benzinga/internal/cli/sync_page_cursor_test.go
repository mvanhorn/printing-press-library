// Copyright 2026 waveriderai and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestNextPageIntCursorZeroIndexed(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{in: "", want: "1"},
		{in: "0", want: "1"},
		{in: "1", want: "2"},
		{in: " 2 ", want: "3"},
		{in: "not-a-page", want: "1"},
		{in: "-1", want: "1"},
	}
	for _, c := range cases {
		if got := nextPageIntCursor(c.in); got != c.want {
			t.Errorf("nextPageIntCursor(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
