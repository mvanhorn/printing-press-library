// Copyright 2026 adrian-horning. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestNormalizeHandle(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"charlidamelio", "charlidamelio"},
		{"@charlidamelio", "charlidamelio"},
		{"  @charlidamelio  ", "charlidamelio"},
		{"", ""},
		{"@", ""},
		{"@@x", "@x"}, // only the first '@' is stripped
	}
	for _, c := range cases {
		if got := NormalizeHandle(c.in); got != c.want {
			t.Errorf("NormalizeHandle(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormalizeHashtag(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"fyp", "fyp"},
		{"#fyp", "fyp"},
		{"  #fyp  ", "fyp"},
		{"", ""},
		{"#", ""},
		{"##x", "#x"},
	}
	for _, c := range cases {
		if got := NormalizeHashtag(c.in); got != c.want {
			t.Errorf("NormalizeHashtag(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
