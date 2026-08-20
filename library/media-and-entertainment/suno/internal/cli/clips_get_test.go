// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
package cli

import "testing"

func TestResolveClipIDs(t *testing.T) {
	cases := []struct {
		name    string
		flagIds string
		args    []string
		want    string
	}{
		{"flag wins", "a,b", []string{"x"}, "a,b"},
		{"positional single", "", []string{"x"}, "x"},
		{"positional multiple", "", []string{"x", "y"}, "x,y"},
		{"neither", "", nil, ""},
	}
	for _, tc := range cases {
		if got := resolveClipIDs(tc.flagIds, tc.args); got != tc.want {
			t.Errorf("%s: resolveClipIDs(%q,%v)=%q want %q", tc.name, tc.flagIds, tc.args, got, tc.want)
		}
	}
}
