// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
package mcp

import "testing"

func TestFormatMCPParamValue(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"large integer id (was 1.925035e+06)", float64(1925035), "1925035"},
		{"another big id", float64(4346907350), "4346907350"},
		{"decimal amount", float64(28.48), "28.48"},
		{"zero", float64(0), "0"},
		{"string passthrough", "2024-01-01", "2024-01-01"},
		{"int", 42, "42"},
	}
	for _, c := range cases {
		if got := formatMCPParamValue(c.in); got != c.want {
			t.Errorf("%s: formatMCPParamValue(%v) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}
