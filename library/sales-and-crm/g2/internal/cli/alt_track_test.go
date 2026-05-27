// Copyright 2026 rahul-bansal. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
)

func TestCsvEscape(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"no special chars passthrough", "Acme Corp", "Acme Corp"},
		{"comma wraps in quotes", "Acme, Inc", `"Acme, Inc"`},
		{"quote gets doubled", `He said "hi"`, `"He said ""hi"""`},
		{"newline wraps in quotes", "line1\nline2", "\"line1\nline2\""},
		{"empty stays empty", "", ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := csvEscape(tc.in); got != tc.want {
				t.Fatalf("csvEscape(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNewAltTrackCmdHelp(t *testing.T) {
	flags := &rootFlags{}
	cmd := newAltTrackCmd(flags)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("alt-track --help failed: %v", err)
	}
}

func TestNewAltTrackCmdDryRun(t *testing.T) {
	// dryRun is a persistent root flag; flip it directly on rootFlags and
	// invoke with a positional arg so the leaf's RunE reaches the dryRunOK
	// short-circuit path (rather than tripping the "product required" gate).
	flags := &rootFlags{dryRun: true}
	cmd := newAltTrackCmd(flags)
	cmd.SetArgs([]string{"slack"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("alt-track dry-run failed: %v", err)
	}
}
