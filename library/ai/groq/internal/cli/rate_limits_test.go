// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
)

// TestNovelRateLimitsHelpWires smoke-tests that the rate-limits command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelRateLimitsHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"rate-limits", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("rate-limits --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "rate-limits"} {
		if !strings.Contains(help, want) {
			t.Fatalf("rate-limits --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestHeaderInt(t *testing.T) {
	h := http.Header{}
	h.Set("X-RateLimit-Limit-Requests", "1000")
	h.Set("X-RateLimit-Remaining-Tokens", "abc")
	if got := headerInt(h, "X-RateLimit-Limit-Requests"); got != 1000 {
		t.Fatalf("limit requests = %d, want 1000", got)
	}
	if got := headerInt(h, "X-RateLimit-Remaining-Tokens"); got != 0 {
		t.Fatalf("non-numeric header = %d, want 0", got)
	}
	if got := headerInt(h, "Missing-Header"); got != 0 {
		t.Fatalf("missing header = %d, want 0", got)
	}
}

func TestOrDash(t *testing.T) {
	if orDash("") != "-" {
		t.Fatal("orDash('') should be '-'")
	}
	if orDash("1m") != "1m" {
		t.Fatal("orDash('1m') should be '1m'")
	}
}
