// Copyright 2026 david and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/gusto/internal/cliutil/testenv"
)

// TestNovelPayrollsCompareHelpWires smoke-tests that the payrolls compare command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelPayrollsCompareHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"payrolls", "compare", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("payrolls compare --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "compare"} {
		if !strings.Contains(help, want) {
			t.Fatalf("payrolls compare --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestSignedMoney(t *testing.T) {
	for in, want := range map[float64]string{-352.91: "-$352.91", 12.5: "+$12.50", 0: "+$0.00"} {
		if got := signedMoney(in); got != want {
			t.Fatalf("signedMoney(%v) = %q, want %q", in, got, want)
		}
	}
}
