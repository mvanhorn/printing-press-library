// Copyright 2026 Zayd and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

// TestEscapePathSegment covers the security hardening that routes user-supplied
// path-segment values through url.PathEscape (replacePathParam). It must
// neutralize segment-breaking characters while preserving the literal commas
// Squarespace's CSV id path params (variantIdCsvs, profileIdCsvs,
// productIdCsvs, documentIds) depend on.
func TestEscapePathSegment(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain id", "abc123", "abc123"},
		{"slash is escaped", "a/b", "a%2Fb"},
		{"query and fragment escaped", "x?y#z", "x%3Fy%23z"},
		{"space escaped", "a b", "a%20b"},
		{"percent escaped", "a%2e", "a%252e"},
		{"csv commas preserved, parts escaped", "id 1,id/2,id3", "id%201,id%2F2,id3"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := escapePathSegment(tc.in); got != tc.want {
				t.Errorf("escapePathSegment(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestReplacePathParamEscapes confirms the splice helper escapes the injected
// value so an id cannot break out of its URL path segment.
func TestReplacePathParamEscapes(t *testing.T) {
	t.Parallel()
	got := replacePathParam("/v2/commerce/products/{productId}", "productId", "evil/../../admin")
	want := "/v2/commerce/products/evil%2F..%2F..%2Fadmin"
	if got != want {
		t.Errorf("replacePathParam = %q, want %q", got, want)
	}
}
