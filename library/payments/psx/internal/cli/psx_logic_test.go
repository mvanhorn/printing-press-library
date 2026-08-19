// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// psx_logic_test.go pins the pure-logic helpers behind the novel commands.
// Every case here corresponds to a defect found in review: each one returned
// wrong-but-plausible output rather than failing loudly.

package cli

import "testing"

// TestParseNum covers the portal's numeric renderings: thousands separators,
// percent suffixes, unit multipliers, and the "1.00 (103.09%)" movers form.
func TestParseNum(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"314.34", 314.34, true},
		{"4,560,170", 4560170, true},
		{"-0.19%", -0.19, true},
		{"482.5M", 482_500_000, true},
		{"1.2B", 1_200_000_000, true},
		{"7.0M", 7_000_000, true},
		{"1.00 (103.09%)", 1.00, true},
		{"-", 0, false},
		{"", 0, false},
		{"n/a", 0, false},
	}
	for _, c := range cases {
		got, ok := parseNum(c.in)
		if ok != c.ok {
			t.Errorf("parseNum(%q) ok = %v, want %v", c.in, ok, c.ok)
			continue
		}
		if ok && got != c.want {
			t.Errorf("parseNum(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestParsePSXDate pins multi-format date handling. A single-format parser
// silently dropped every history row when a surface changed rendering.
func TestParsePSXDate(t *testing.T) {
	cases := map[string]string{
		"Aug 19, 2026":           "2026-08-19",
		"April 29, 2026 3:56 PM": "2026-04-29",
		"2026-08-19":             "2026-08-19",
		"12/05/2026":             "2026-05-12",
		"":                       "",
		"not a date":             "",
	}
	for in, want := range cases {
		if got := parsePSXDate(in); got != want {
			t.Errorf("parsePSXDate(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestRangeStart pins book-closure range handling. PSX renders book closure as
// "12/05/2026 - 13/05/2026"; reading the whole cell as one date yielded no
// deadlines at all from payouts deadline.
func TestRangeStart(t *testing.T) {
	cases := map[string]string{
		"12/05/2026 - 13/05/2026": "12/05/2026",
		"12/05/2026":              "12/05/2026",
		"":                        "",
	}
	for in, want := range cases {
		if got := rangeStart(in); got != want {
			t.Errorf("rangeStart(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestExpandPayoutKind pins the trailing type code. An unknown code must pass
// through untouched rather than be guessed at.
func TestExpandPayoutKind(t *testing.T) {
	cases := map[string]string{
		"32.50%(iii) (D)": "32.50%(iii) dividend",
		"10%(i) (B)":      "10%(i) bonus",
		"5% (R)":          "5% right issue",
		"7.5%(ii) (Z)":    "7.5%(ii) (Z)",
		"":                "",
	}
	for in, want := range cases {
		if got := expandPayoutKind(in); got != want {
			t.Errorf("expandPayoutKind(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestDescribePayout pins readable prose over the raw key=value dump the
// actions digest previously emitted.
func TestDescribePayout(t *testing.T) {
	row := map[string]string{
		"details":           "32.50%(iii) (D)",
		"book_closure":      "12/05/2026 - 13/05/2026",
		"financial_results": "31/03/2026(IIIQ)",
	}
	got := describePayout(row)
	want := "32.50%(iii) dividend, book closure 12/05/2026 - 13/05/2026, for period 31/03/2026(IIIQ)"
	if got != want {
		t.Errorf("describePayout() = %q, want %q", got, want)
	}
	if fallback := describePayout(map[string]string{"symbol": "OGDC"}); fallback == "" {
		t.Error("describePayout with no known columns should fall back, not return empty")
	}
}

// TestPerformerIndex pins the mover ordinals. These index into the UNFILTERED
// table list; dropping empty tables first made --kind gainers return losers.
func TestPerformerIndex(t *testing.T) {
	for kind, want := range map[string]int{"active": 0, "": 0, "gainers": 1, "up": 1, "losers": 2, "down": 2} {
		got, err := performerIndex(kind)
		if err != nil {
			t.Errorf("performerIndex(%q) unexpected error: %v", kind, err)
			continue
		}
		if got != want {
			t.Errorf("performerIndex(%q) = %d, want %d", kind, got, want)
		}
	}
	if _, err := performerIndex("sideways"); err == nil {
		t.Error("performerIndex should reject an unknown kind")
	}
}

// TestNormalizeISODate pins calendar-date normalisation. Un-normalised dates
// sorted arbitrarily against the other feeds in the merged actions digest.
func TestNormalizeISODate(t *testing.T) {
	for in, want := range map[string]string{
		"2026-09-30":   "2026-09-30",
		"Aug 19, 2026": "2026-08-19",
		"":             "",
	} {
		if got := normalizeISODate(in); got != want {
			t.Errorf("normalizeISODate(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestPreviousDay pins the derived buy-by date.
func TestPreviousDay(t *testing.T) {
	if got := previousDay("2026-05-12"); got != "2026-05-11" {
		t.Errorf("previousDay = %q, want 2026-05-11", got)
	}
	// Month boundary.
	if got := previousDay("2026-05-01"); got != "2026-04-30" {
		t.Errorf("previousDay across month = %q, want 2026-04-30", got)
	}
	// Unparseable input passes through rather than inventing a date.
	if got := previousDay("garbage"); got != "garbage" {
		t.Errorf("previousDay(unparseable) = %q, want passthrough", got)
	}
}

// TestMedianAndMAD pins the robust statistics behind `unusual`. A mean would
// be dominated by the outlier the command is trying to detect.
func TestMedianAndMAD(t *testing.T) {
	if got := median([]float64{5, 1, 3}); got != 3 {
		t.Errorf("median odd = %v, want 3", got)
	}
	if got := median([]float64{4, 1, 3, 2}); got != 2.5 {
		t.Errorf("median even = %v, want 2.5", got)
	}
	if got := median(nil); got != 0 {
		t.Errorf("median empty = %v, want 0", got)
	}
	xs := []float64{10, 10, 10, 10, 1000}
	med := median(xs)
	if mad := medianAbsDev(xs, med); mad != 0 {
		t.Errorf("medianAbsDev with a single outlier = %v, want 0 (outlier must not move it)", mad)
	}
}

// TestResolveDriftColumn pins candidate resolution. A fixed key could never
// match "MARKET CAP. (B)" -> market_cap_b, so drift blamed missing history for
// data the user already had.
func TestResolveDriftColumn(t *testing.T) {
	rows := []snapshotRow{{Data: map[string]string{"market_cap_b": "482.5", "price": "24.17"}}}
	got, available := resolveDriftColumn([]string{"market_cap", "market_cap_b"}, rows)
	if got != "market_cap_b" {
		t.Errorf("resolveDriftColumn = %q, want market_cap_b", got)
	}
	if len(available) == 0 {
		t.Error("available columns should be reported for diagnostics")
	}
	if miss, _ := resolveDriftColumn([]string{"nonexistent"}, rows); miss != "" {
		t.Errorf("unmatched candidate should return empty, got %q", miss)
	}
}

// TestAbsf pins the ranking helper used by diff, rotation and basis.
func TestAbsf(t *testing.T) {
	if absf(-2.5) != 2.5 || absf(2.5) != 2.5 || absf(0) != 0 {
		t.Error("absf incorrect")
	}
}

// TestTruncateStrIsRuneSafe pins multi-byte handling; byte slicing split the
// em dash in calendar headlines into invalid UTF-8.
func TestTruncateStrIsRuneSafe(t *testing.T) {
	in := "AGM — International Industries Limited (Karachi)"
	got := truncateStr(in, 10)
	for _, r := range got {
		if r == '�' {
			t.Fatalf("truncateStr produced invalid UTF-8: %q", got)
		}
	}
	if len([]rune(got)) != 10 {
		t.Errorf("truncateStr rune length = %d, want 10", len([]rune(got)))
	}
}
