// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/passive-indices/internal/niftyindices"
)

// TestNovelCompareHelpWires smoke-tests that the compare command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelCompareHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"compare", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("compare --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "compare"} {
		if !strings.Contains(help, want) {
			t.Fatalf("compare --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestCompareNormalizesAndValidatesBenchmarkIdentity(t *testing.T) {
	if err := validateBenchmarkIdentity("1150", "Nifty 50 TRI", "nifty 50"); err != nil {
		t.Fatalf("equivalent benchmark rejected: %v", err)
	}
	if err := validateBenchmarkIdentity("1150", "Nifty 50 TRI", "NIFTY BANK"); err == nil {
		t.Fatal("mismatched benchmark was accepted")
	}

	quotes := []niftyindices.LiveQuote{{IndexName: "NIFTY BANK", Last: 42}}
	got := findLiveQuote(quotes, "nifty bank")
	if got == nil || got.Last != 42 {
		t.Fatalf("case-insensitive live quote lookup = %#v, want NIFTY BANK", got)
	}
}

func TestCompareReportsConstituentFailure(t *testing.T) {
	out := map[string]any{}
	failures := addConstituentResult(out, nil, nil, errors.New("upstream unavailable"), 10)
	if len(failures) != 1 || failures[0]["source"] != "index_constituents" || failures[0]["error"] == "" {
		t.Fatalf("constituent failures = %#v, want labeled failure", failures)
	}
	if _, ok := out["index_constituents_sample"]; ok {
		t.Fatal("failed constituent fetch should not add a sample")
	}
}
