// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// MUFAP serves a rendered page for ANY (Month, Year) it is asked for, so a
// mistyped parameter comes back as an ordinary-looking empty table rather than
// an error. Validating before the request is the only way a caller learns they
// mistyped instead of reading a fabricated empty month.
func TestNetsalesValidatePeriod(t *testing.T) {
	ok := []struct {
		month, year string
		wantM       int
		wantY       int
	}{
		{"5", "2026", 5, 2026},
		{"05", "2026", 5, 2026}, // accepted and normalized; the wire format is unpadded
		{"1", "2018", 1, 2018},
		{"12", "2026", 12, 2026},
	}
	for _, tc := range ok {
		m, y, err := netsalesValidatePeriod(tc.month, tc.year)
		if err != nil {
			t.Errorf("netsalesValidatePeriod(%q,%q) errored: %v", tc.month, tc.year, err)
			continue
		}
		if m != tc.wantM || y != tc.wantY {
			t.Errorf("netsalesValidatePeriod(%q,%q) = %d,%d; want %d,%d", tc.month, tc.year, m, y, tc.wantM, tc.wantY)
		}
	}

	bad := [][2]string{
		{"0", "2026"}, {"13", "2026"}, {"-1", "2026"}, {"", "2026"}, {"may", "2026"},
		{"5", "26"}, {"5", ""}, {"5", "1999"}, {"5", "2101"}, {"5", "twenty"},
	}
	for _, tc := range bad {
		if _, _, err := netsalesValidatePeriod(tc[0], tc[1]); err == nil {
			t.Errorf("netsalesValidatePeriod(%q,%q) should have failed", tc[0], tc[1])
		}
	}
}

// The zero-padding help text must not claim a rejection the code does not make.
func TestNetsalesMonthFlagHelpMatchesBehaviour(t *testing.T) {
	cmd := newNetsalesMonthlyParsedCmd(&rootFlags{})
	f := cmd.Flags().Lookup("month")
	if f == nil {
		t.Fatal("--month flag missing")
	}
	if strings.Contains(f.Usage, "NOT zero-padded") {
		t.Errorf("help says zero-padded input is rejected, but netsalesValidatePeriod accepts and normalizes it: %q", f.Usage)
	}
	if _, _, err := netsalesValidatePeriod("05", "2026"); err != nil {
		t.Errorf("behaviour changed: %q is now rejected, so the help text needs updating too", "05")
	}
}

// Backoff must actually grow, or a bounded retry against an intermittent
// challenge is just four rapid failures.
func TestNetsalesBackoffGrows(t *testing.T) {
	prev := netsalesBackoff(1)
	if prev <= 0 {
		t.Fatalf("first backoff must be positive, got %v", prev)
	}
	for i := 2; i < netsalesChallengeAttempts; i++ {
		cur := netsalesBackoff(i)
		if cur <= prev {
			t.Errorf("backoff(%d)=%v is not greater than backoff(%d)=%v", i, cur, i-1, prev)
		}
		prev = cur
	}
}

// The generated leaves return {} at exit 0 on these HTML table pages. The novel
// hook must replace them, and must leave exactly one command per name.
func TestNetsalesNovelHookReplacesGeneratedLeaves(t *testing.T) {
	root := &cobra.Command{Use: "mufap-pp-cli"}
	flags := &rootFlags{}
	root.AddCommand(newNetsalesCmd(flags))

	var parent *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "netsales" {
			parent = c
		}
	}
	if parent == nil {
		t.Fatal("generated netsales parent not found")
	}
	// Before: the generated leaves are present.
	if len(parent.Commands()) != 2 {
		t.Fatalf("expected 2 generated leaves, got %d", len(parent.Commands()))
	}

	// Run every registered novel hook, as root construction does.
	for _, hook := range novelCommandHooks {
		hook(root, flags)
	}

	counts := map[string]int{}
	for _, c := range parent.Commands() {
		counts[c.Name()]++
	}
	for _, name := range []string{"monthly", "investor"} {
		if counts[name] != 1 {
			t.Errorf("netsales %s appears %d times after the hook; want exactly 1", name, counts[name])
		}
	}

	// The replacements must be the parsing ones: the generated leaves carry a
	// pp:endpoint annotation, ours carry pp:data-source=live and declare typed
	// exit codes.
	for _, c := range parent.Commands() {
		if c.Name() != "monthly" && c.Name() != "investor" {
			continue
		}
		if c.Annotations["pp:data-source"] != "live" {
			t.Errorf("netsales %s is still the generated leaf (annotations %v)", c.Name(), c.Annotations)
		}
		if c.Annotations["pp:typed-exit-codes"] == "" {
			t.Errorf("netsales %s should declare typed exit codes", c.Name())
		}
		if c.Flags().Lookup("month") == nil || c.Flags().Lookup("year") == nil {
			t.Errorf("netsales %s lost its --month/--year flags", c.Name())
		}
	}
}

// A missing figure must never render as 0: the whole point of the pointer
// fields is that a non-reporting category is not a category reporting zero.
func TestNetsalesCellRendersMissingAsDash(t *testing.T) {
	if got := netsalesCell(nil); got != "-" {
		t.Errorf("netsalesCell(nil) = %q, want %q", got, "-")
	}
	zero := 0.0
	if got := netsalesCell(&zero); got != "0" {
		t.Errorf("netsalesCell(0) = %q, want %q -- a real zero must be distinguishable from missing", got, "0")
	}
	neg := -12.0
	if got := netsalesCell(&neg); got != "-12" {
		t.Errorf("netsalesCell(-12) = %q, want %q", got, "-12")
	}
}
