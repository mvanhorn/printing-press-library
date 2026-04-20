// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestNormalizePersonInput(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"williamhgates", "williamhgates"},
		{"https://www.linkedin.com/in/williamhgates", "williamhgates"},
		{"https://www.linkedin.com/in/williamhgates/", "williamhgates"},
		{"http://linkedin.com/in/alonsovelasco", "alonsovelasco"},
		{"http://linkedin.com/in/alonsovelasco/", "alonsovelasco"},
		{"/in/alonsovelasco/", "alonsovelasco"},
		{"  jenniferbonuso  ", "jenniferbonuso"},
	}
	for _, tc := range cases {
		if got := normalizePersonInput(tc.in); got != tc.want {
			t.Errorf("normalizePersonInput(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeSections(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{}, ""},
		{[]string{"experience"}, "experience"},
		{[]string{"experience", "education"}, "experience,education"},
	}
	for _, tc := range cases {
		if got := normalizeSections(tc.in); got != tc.want {
			t.Errorf("normalizeSections(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
