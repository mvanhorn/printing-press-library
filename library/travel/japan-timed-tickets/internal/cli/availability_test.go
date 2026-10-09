// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-timed-tickets/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/travel/japan-timed-tickets/internal/tickets"
)

// TestNovelAvailabilityHelpWires smoke-tests that the availability command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelAvailabilityHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"availability", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("availability --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "availability"} {
		if !strings.Contains(help, want) {
			t.Fatalf("availability --help missing %q in output:\n%s", want, help)
		}
	}
}

// TestSlotStartRange checks that the human SLOTS range spans the earliest and
// latest start across products, not the first and last list items.
func TestSlotStartRange(t *testing.T) {
	first, last, ok := slotStartRange([]string{"08:00", "09:30", "20:00", "08:00"})
	if !ok || first != "08:00" || last != "20:00" {
		t.Fatalf("slotStartRange = %q, %q, %v", first, last, ok)
	}
	first, last, ok = slotStartRange([]string{"9:00", "10:30"})
	if !ok || first != "9:00" || last != "10:30" {
		t.Fatalf("unpadded hours: %q, %q, %v", first, last, ok)
	}
	if _, _, ok := slotStartRange([]string{"", "n/a"}); ok {
		t.Fatalf("no parseable start should report ok=false")
	}
}

// TestSlotStockNote checks that the slot totals are summed per product, so a
// store calendar label such as "few" is shown next to the real slot stock.
func TestSlotStockNote(t *testing.T) {
	n := func(v int) *int { return &v }
	slots := []tickets.Slot{
		{Product: "Entrance Pass", Stock: n(59), Capacity: n(145)},
		{Product: "Entrance Pass", Stock: n(226), Capacity: n(275)},
		{Product: "Premium Pass", Stock: n(3), Capacity: n(10)},
		{Product: "Entrance Pass"}, // unknown stock is not counted
	}
	got := slotStockNote(slots)
	want := "; slot read: Entrance Pass 285 of 420 places open in 2 slots, Premium Pass 3 of 10 places open in 1 slot"
	if got != want {
		t.Fatalf("slotStockNote = %q, want %q", got, want)
	}
	if got := slotStockNote([]tickets.Slot{{Product: "X", Stock: n(5)}}); got != "; slot read: X 5 places open in 1 slot" {
		t.Fatalf("stock without capacity: %q", got)
	}
	soldOut := []tickets.Slot{
		{Product: "Entrance Pass", Stock: n(0), Capacity: n(145)},
		{Product: "Entrance Pass", Stock: n(40), Capacity: n(145)},
	}
	if got := slotStockNote(soldOut); got != "; slot read: Entrance Pass 40 of 290 places open in 2 slots (1 sold out)" {
		t.Fatalf("sold-out slot count: %q", got)
	}
	if got := slotStockNote(nil); got != "" {
		t.Fatalf("no slots: %q", got)
	}
}
