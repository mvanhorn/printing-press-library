// Copyright 2026 richardadonnell and contributors. Licensed under Apache-2.0. See LICENSE.

package autotempest

import "testing"

func TestNormalizeSlug(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"F-150", "f150"},
		{"CR-V", "crv"},
		{"Alfa Romeo", "alfaromeo"},
		{"civic", "civic"},
		{"model 3", "model3"},
		{"", ""},
		{"  Honda  ", "honda"},
		{"Mercedes-Benz", "mercedesbenz"},
	}
	for _, c := range cases {
		if got := NormalizeSlug(c.in); got != c.want {
			t.Errorf("NormalizeSlug(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
