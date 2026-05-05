// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// coverage_test.go: PreRunE validation tests for the coverage command.
// Full integration tests (cookie/bearer fan-out) live in source_selection_test.go;
// this file is scoped to the U3 --location flag's argument-validation contract.

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newCoverageRootCmd builds a stripped-down root that mounts only the
// coverage subcommand. Mirrors newAPIHpnRootCmd's pattern in
// api_hpn_test.go so PreRunE validation can be exercised without
// dragging in unrelated init.
func newCoverageRootCmd(t *testing.T, flags *rootFlags) *cobra.Command {
	t.Helper()
	root := &cobra.Command{Use: "contact-goat-pp-cli", SilenceUsage: true, SilenceErrors: true}
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.AddCommand(newCoverageCmd(flags))
	return root
}

// TestCoverage_RejectsBothPositionalAndLocation: cobra accepts the
// invocation, PreRunE rejects with a usage error.
func TestCoverage_RejectsBothPositionalAndLocation(t *testing.T) {
	flags := &rootFlags{dryRun: true, yes: true}
	root := newCoverageRootCmd(t, flags)
	_, _, err := runCmd(t, root, []string{"coverage", "Stripe", "--location", "San Francisco"})
	if err == nil {
		t.Fatal("want usage error when both positional and --location are set")
	}
	if !strings.Contains(err.Error(), "OR --location") {
		t.Errorf("error should mention positional-or-location exclusivity, got: %v", err)
	}
}

// TestCoverage_RejectsNeitherPositionalNorLocation: zero positional
// args AND no --location is a usage error.
func TestCoverage_RejectsNeitherPositionalNorLocation(t *testing.T) {
	flags := &rootFlags{dryRun: true, yes: true}
	root := newCoverageRootCmd(t, flags)
	_, _, err := runCmd(t, root, []string{"coverage"})
	if err == nil {
		t.Fatal("want usage error when neither positional nor --location is set")
	}
	if !strings.Contains(err.Error(), "company positional or --location") {
		t.Errorf("error should mention positional-or-location requirement, got: %v", err)
	}
}

// TestCoverage_RejectsLocationWithCookieSource: --location mode requires
// bearer-only; explicit --source hp is a usage error.
func TestCoverage_RejectsLocationWithCookieSource(t *testing.T) {
	flags := &rootFlags{dryRun: true, yes: true}
	root := newCoverageRootCmd(t, flags)
	_, _, err := runCmd(t, root, []string{"coverage", "--location", "San Francisco", "--source", "hp"})
	if err == nil {
		t.Fatal("want usage error for --location --source hp")
	}
	if !strings.Contains(err.Error(), "--source hp not supported") {
		t.Errorf("error should explain cookie surface limitation, got: %v", err)
	}
}

// TestCoverage_RejectsLocationWithLISource: --location mode rejects
// --source li explicitly (LinkedIn has no city-search).
func TestCoverage_RejectsLocationWithLISource(t *testing.T) {
	flags := &rootFlags{dryRun: true, yes: true}
	root := newCoverageRootCmd(t, flags)
	_, _, err := runCmd(t, root, []string{"coverage", "--location", "San Francisco", "--source", "li"})
	if err == nil {
		t.Fatal("want usage error for --location --source li")
	}
	if !strings.Contains(err.Error(), "--source li not supported") {
		t.Errorf("error should explain LinkedIn limitation, got: %v", err)
	}
}

// TestCoverage_HelpRendersWithLocationFlag: smoke test that --help
// surfaces the new flag without panic. Locks SKILL.md / verifier
// expectations: --location must be a registered flag on the cobra
// command.
func TestCoverage_HelpRendersWithLocationFlag(t *testing.T) {
	flags := &rootFlags{}
	root := newCoverageRootCmd(t, flags)
	out, _, err := runCmd(t, root, []string{"coverage", "--help"})
	if err != nil {
		t.Fatalf("--help returned error: %v", err)
	}
	if !strings.Contains(out, "--location") {
		t.Errorf("help should document --location, got:\n%s", out)
	}
	if !strings.Contains(out, "positional OR --location") {
		t.Errorf("help should describe scope choice, got:\n%s", out)
	}
}
