// Copyright 2026 Nimrod Astarhan and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
)

func TestInjectAccountIDIntoArgs(t *testing.T) {
	const acct = "12345678"
	cases := []struct {
		name  string
		args  []string
		want  []string
	}{
		{
			name: "runs list with no account_id",
			args: []string{"dbt-cloud-pp-cli", "runs", "list"},
			want: []string{"dbt-cloud-pp-cli", "runs", "list", acct},
		},
		{
			name: "runs list with account_id already present",
			args: []string{"dbt-cloud-pp-cli", "runs", "list", acct},
			want: []string{"dbt-cloud-pp-cli", "runs", "list", acct},
		},
		{
			name: "runs list with flag first",
			args: []string{"dbt-cloud-pp-cli", "runs", "list", "--limit", "5"},
			want: []string{"dbt-cloud-pp-cli", "runs", "list", acct, "--limit", "5"},
		},
		{
			name: "jobs list with no account_id",
			args: []string{"dbt-cloud-pp-cli", "jobs", "list"},
			want: []string{"dbt-cloud-pp-cli", "jobs", "list", acct},
		},
		{
			name: "unknown command not affected",
			args: []string{"dbt-cloud-pp-cli", "doctor"},
			want: []string{"dbt-cloud-pp-cli", "doctor"},
		},
		{
			name: "monitor command not affected",
			args: []string{"dbt-cloud-pp-cli", "monitor", "9999"},
			want: []string{"dbt-cloud-pp-cli", "monitor", "9999"},
		},
		{
			name: "short args not affected",
			args: []string{"dbt-cloud-pp-cli"},
			want: []string{"dbt-cloud-pp-cli"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := injectAccountIDIntoArgs(tc.args, acct)
			if len(got) != len(tc.want) {
				t.Fatalf("injectAccountIDIntoArgs() = %v (len %d), want %v (len %d)",
					got, len(got), tc.want, len(tc.want))
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("index %d: got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestIsAllDigits(t *testing.T) {
	cases := []struct{ s string; want bool }{
		{"12345", true},
		{"0", true},
		{"", false},
		{"123abc", false},
		{"abc", false},
		{"-1", false},
	}
	for _, tc := range cases {
		if got := isAllDigits(tc.s); got != tc.want {
			t.Errorf("isAllDigits(%q) = %v, want %v", tc.s, got, tc.want)
		}
	}
}
