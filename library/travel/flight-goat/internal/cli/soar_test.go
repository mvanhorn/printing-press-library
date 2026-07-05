// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestSoarPricePreservesCents(t *testing.T) {
	cases := []struct {
		currency string
		price    float64
		want     string
	}{
		{"USD", 94.98, "USD 94.98"},
		{"USD", 309.4, "USD 309.40"},
		{"usd", 144, "USD 144.00"},
		{"", 87.4, "USD 87.40"},
	}
	for _, c := range cases {
		if got := soarPrice(c.currency, c.price); got != c.want {
			t.Errorf("soarPrice(%q, %v) = %q, want %q", c.currency, c.price, got, c.want)
		}
	}
}
