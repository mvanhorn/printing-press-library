// Copyright 2026 Joseph Alvin Castillo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestResolveProjectID(t *testing.T) {
	cases := []struct {
		name      string
		flag      string
		env       string
		want      string
		wantError bool
	}{
		{name: "flag wins", flag: "proj_flag", env: "proj_env", want: "proj_flag"},
		{name: "env fallback", flag: "", env: "proj_env", want: "proj_env"},
		{name: "flag trimmed", flag: "  proj_flag  ", env: "", want: "proj_flag"},
		{name: "neither set", flag: "", env: "", wantError: true},
		{name: "whitespace-only flag falls back to env", flag: "   ", env: "proj_env", want: "proj_env"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("REVENUECAT_PROJECT_ID", tc.env)
			got, err := resolveProjectID(tc.flag)
			if tc.wantError {
				if err == nil {
					t.Fatalf("expected error, got project=%q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("resolveProjectID(%q) = %q, want %q", tc.flag, got, tc.want)
			}
		})
	}
}
