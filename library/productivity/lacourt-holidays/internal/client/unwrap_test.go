// Copyright 2026 Bob Benjy and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import "testing"

func TestUnwrapJSONStringBody(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"double-encoded array", `"[{\"date\":\"01/01/2027\"}]"`, `[{"date":"01/01/2027"}]`},
		{"empty array string", `"[]"`, `[]`},
		{"plain array", `[1,2]`, `[1,2]`},
		{"plain string", `"hello"`, `"hello"`},
		{"object", `{"a":1}`, `{"a":1}`},
		{"string with invalid json", `"[not json"`, `"[not json"`},
	}
	for _, tc := range cases {
		if got := string(unwrapJSONStringBody([]byte(tc.in))); got != tc.want {
			t.Errorf("%s: got %s want %s", tc.name, got, tc.want)
		}
	}
}
