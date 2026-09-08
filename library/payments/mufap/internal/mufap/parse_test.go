// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package mufap

import "testing"

// Shaped exactly like a MUFAP daily tab=1 page: a real <thead>, two data rows,
// and a trailing sibling row with a different cell count that must be skipped.
const sampleDaily = `
<table id="t"><thead><tr>
<th>Sector</th><th>Category</th><th>Fund Name</th><th>Validity Date</th><th>NAV</th><th>30 Days</th>
</tr></thead><tbody>
<tr><td>Open-End Funds</td><td>Money Market (Annualized Return )</td><td>ABL Cash Fund</td><td>Sep 04, 2026</td><td>10.4569</td><td>10.55</td></tr>
<tr><td>Open-End Funds</td><td>Equity (Absolute Return )</td><td>ABL Stock&nbsp;Fund</td><td>Sep 04, 2026</td><td>1,236.2847</td><td>-2.31</td></tr>
<tr><td>colspan filler</td></tr>
</tbody></table>`

func TestParseTable(t *testing.T) {
	tbl, err := ParseTable(sampleDaily)
	if err != nil {
		t.Fatalf("ParseTable: %v", err)
	}
	if got, want := len(tbl.Headers), 6; got != want {
		t.Fatalf("headers = %d, want %d (%v)", got, want, tbl.Headers)
	}
	if got, want := len(tbl.Rows), 2; got != want {
		t.Fatalf("rows = %d, want %d — the 1-cell sibling row must be skipped", got, want)
	}
	if got, want := tbl.Rows[0]["Fund Name"], "ABL Cash Fund"; got != want {
		t.Errorf("row0 Fund Name = %q, want %q", got, want)
	}
	// &nbsp; must be decoded, not carried through as a literal entity.
	if got, want := tbl.Rows[1]["Fund Name"], "ABL Stock Fund"; got != want {
		t.Errorf("row1 Fund Name = %q, want %q (entity not decoded)", got, want)
	}
}

func TestParseTableNoThead(t *testing.T) {
	if _, err := ParseTable("<html><body>Just a moment...</body></html>"); err == nil {
		t.Fatal("expected an error for a challenge page with no <thead>, got nil")
	}
}

func TestParseNumber(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"10.4569", 10.4569, true},
		{"1,236.2847", 1236.2847, true},
		{"-2.31", -2.31, true},
		{"95.16%", 95.16, true},
		{"0.0000", 0, true},
		// MUFAP writes negatives in accounting notation and never with a
		// minus sign. Rejecting these dropped 81% of the equity universe.
		{"(4.97)", -4.97, true},
		{"(1,236.28)", -1236.28, true},
		{"(3.10)%", -3.10, true},
		{"(0.18)", -0.18, true},
		// MUFAP's several spellings of "this fund did not report". None may
		// become a zero: a non-reporting fund is not a fund holding zero.
		{"-", 0, false},
		{"N/A", 0, false},
		{"Not Published", 0, false},
		{"", 0, false},
		{"  ", 0, false},
		{"()", 0, false},
		{"(N/A)", 0, false},
	}
	for _, c := range cases {
		got, ok := ParseNumber(c.in)
		if ok != c.ok {
			t.Errorf("ParseNumber(%q) ok = %v, want %v", c.in, ok, c.ok)
			continue
		}
		if ok && got != c.want {
			t.Errorf("ParseNumber(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestNormalizeValidityDate(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"Sep 04, 2026", "2026-09-04", true},
		{"Jan 01, 2005", "2005-01-01", true},
		{"Dec 31, 1999", "1999-12-31", true},
		{"2026-09-04", "", false},
		{"Sept 4, 2026", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := NormalizeValidityDate(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("NormalizeValidityDate(%q) = (%q,%v), want (%q,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestMonthKey(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		// The allocation endpoint wants an unpadded M-YYYY; sending ISO or a
		// zero-padded month returns HTTP 500 or an empty table, never an error.
		{"2026-07-31", "7-2026", false},
		{"2026-07", "7-2026", false},
		{"2026-12", "12-2026", false},
		{"2026-13", "", true},
		{"2026-00", "", true},
		{"nonsense", "", true},
		{"2026", "", true},
	}
	for _, c := range cases {
		got, err := MonthKey(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("MonthKey(%q) err = %v, wantErr %v", c.in, err, c.wantErr)
			continue
		}
		if !c.wantErr && got != c.want {
			t.Errorf("MonthKey(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
