// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/internal/cliutil/testenv"
)

// TestNovelDatesHelpWires smoke-tests that the dates command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelDatesHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"dates", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dates --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "dates"} {
		if !strings.Contains(help, want) {
			t.Fatalf("dates --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestDatesNoteShowsHoursReason(t *testing.T) {
	open := "09:00"
	cases := []struct {
		name string
		row  dateRow
		want string
	}{
		{"same reason once", dateRow{TicketsUnknownReason: "month not published", Hours: hoursView{UnknownReason: "month not published"}}, "month not published"},
		{"both reasons", dateRow{TicketsUnknownReason: "store behind waiting room", Hours: hoursView{UnknownReason: "beyond schedule"}}, "hours: beyond schedule; tickets: store behind waiting room"},
		{"hours only", dateRow{Hours: hoursView{UnknownReason: "beyond schedule"}}, "hours: beyond schedule"},
		{"hours known", dateRow{TicketsUnknownReason: "store behind waiting room", Hours: hoursView{Open: &open, UnknownReason: "x"}}, "store behind waiting room"},
		{"sale rule", dateRow{TicketsUnknownReason: "not on sale", SaleOpensAt: &saleRule{At: "2026-10-01T14:00+09:00"}, Hours: hoursView{UnknownReason: "not on sale"}}, "sale opens 2026-10-01T14:00+09:00 (rule)"},
	}
	for _, tc := range cases {
		if got := datesNote(tc.row); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}
