// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newRenderTestCmd returns a cobra.Command whose stdout is captured into the
// returned buffer, so renderIssuersFind's output gating can be asserted.
func newRenderTestCmd() (*cobra.Command, *bytes.Buffer) {
	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "find"}
	cmd.SetOut(&buf)
	cmd.SetErr(&bytes.Buffer{})
	return cmd, &buf
}

// TestRenderIssuersFindQuiet locks in the Greptile P1 fix: --quiet must
// suppress all stdout regardless of the other format flags, since the CSV,
// auto-JSON (flags.asJSON), and human-table branches each emit to stdout
// unconditionally. The exit code (nil error) communicates the result.
func TestRenderIssuersFindQuiet(t *testing.T) {
	matches := []issuerRecord{
		{Slug: "united-states", Label: "United States", Parent: ""},
		{Slug: "canada", Label: "Canada", Parent: ""},
	}

	cases := []struct {
		name  string
		flags *rootFlags
	}{
		{"plain quiet", &rootFlags{quiet: true}},
		{"json quiet", &rootFlags{quiet: true, asJSON: true}},
		{"csv quiet", &rootFlags{quiet: true, csv: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd, buf := newRenderTestCmd()
			if err := renderIssuersFind(cmd, tc.flags, matches); err != nil {
				t.Fatalf("renderIssuersFind returned error: %v", err)
			}
			if got := buf.String(); got != "" {
				t.Errorf("--quiet must suppress all stdout, got %q", got)
			}
		})
	}
}

// TestRenderIssuersFindEmitsWhenNotQuiet guards against an over-broad fix that
// silences output unconditionally: with no quiet flag the human table must
// still render the matched slugs.
func TestRenderIssuersFindEmitsWhenNotQuiet(t *testing.T) {
	matches := []issuerRecord{{Slug: "united-states", Label: "United States"}}
	cmd, buf := newRenderTestCmd()
	if err := renderIssuersFind(cmd, &rootFlags{}, matches); err != nil {
		t.Fatalf("renderIssuersFind returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "united-states") {
		t.Errorf("non-quiet render must emit matched slug, got %q", buf.String())
	}
}
