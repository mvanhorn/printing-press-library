// Copyright 2026 Alex Bresler and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

// TestLooksLikeURLOrHost guards the `changes` argument validator. It must accept
// real domains and URLs and reject obviously-invalid tokens — including the
// dogfood sentinel __printing_press_invalid__ — so an invalid argument fails
// fast with a non-zero exit instead of becoming a doomed empty archive query.
func TestLooksLikeURLOrHost(t *testing.T) {
	valid := []string{
		"example.com",
		"sub.example.co.uk",
		"https://example.com/pricing",
		"http://www.example.org",
	}
	invalid := []string{
		"",
		"   ",
		"__printing_press_invalid__",
		"localhost",
		"not a host",
	}
	for _, s := range valid {
		if !looksLikeURLOrHost(s) {
			t.Errorf("expected %q to be accepted", s)
		}
	}
	for _, s := range invalid {
		if looksLikeURLOrHost(s) {
			t.Errorf("expected %q to be rejected", s)
		}
	}
}
