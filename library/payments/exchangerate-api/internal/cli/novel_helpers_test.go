// Tests for pure-logic helpers in hand-authored novel commands.
// Covers: splitAndUpper (matrix.go), parseDurationOrDate (history_cache.go),
// nextRefreshDate (quota_burn.go), tierRank + lastN (plan_check.go), and the
// cross-rate computation idiom from matrix.go.
package cli

// PATCH exchangerate-novel-helper-tests: table-driven tests for pure-logic helpers (splitAndUpper, parseDurationOrDate, nextRefreshDate, tierRank, lastN, cross-rate math).

import (
	"math"
	"testing"
	"time"
)

func TestSplitAndUpper(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"USD,EUR,GBP", []string{"USD", "EUR", "GBP"}},
		{"usd,eur,gbp", []string{"USD", "EUR", "GBP"}},
		{"  usd , eur ,gbp  ", []string{"USD", "EUR", "GBP"}},
		{"USD,,EUR", []string{"USD", "EUR"}},
		{",,,", []string{}},
		{"USD", []string{"USD"}},
		{"", []string{}},
	}
	for _, tc := range cases {
		got := splitAndUpper(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("splitAndUpper(%q): len=%d want=%d (%v vs %v)", tc.in, len(got), len(tc.want), got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("splitAndUpper(%q)[%d] = %q, want %q", tc.in, i, got[i], tc.want[i])
			}
		}
	}
}

func TestParseDurationOrDate(t *testing.T) {
	now := time.Now().UTC()

	// Duration shortcuts (`d`, `w`, `h`, `m`-as-minutes).
	shortcuts := []struct {
		in   string
		want time.Duration
	}{
		{"30d", 30 * 24 * time.Hour},
		{"7d", 7 * 24 * time.Hour},
		{"24h", 24 * time.Hour},
		{"15m", 15 * time.Minute},
		{"2w", 14 * 24 * time.Hour},
	}
	for _, tc := range shortcuts {
		got, err := parseDurationOrDate(tc.in)
		if err != nil {
			t.Errorf("parseDurationOrDate(%q): unexpected err: %v", tc.in, err)
			continue
		}
		delta := now.Sub(got)
		// Allow 5s clock skew between our `now` and the call's internal Now.
		if delta < tc.want-5*time.Second || delta > tc.want+5*time.Second {
			t.Errorf("parseDurationOrDate(%q): got delta=%v, want=%v", tc.in, delta, tc.want)
		}
	}

	// ISO date — start-of-day UTC. parseDurationOrDate is the lower-bound
	// variant used by --since callers (history-cache/drift/log), so the
	// named day's start is correct — `captured_at >= 2024-03-27 00:00:00`
	// includes the full named day.
	got, err := parseDurationOrDate("2024-03-27")
	if err != nil {
		t.Fatalf("parseDurationOrDate(\"2024-03-27\"): %v", err)
	}
	if got.Year() != 2024 || got.Month() != 3 || got.Day() != 27 || got.Hour() != 0 {
		t.Errorf("parseDurationOrDate(\"2024-03-27\"): got %v, want 2024-03-27 00:00:00 UTC", got)
	}

	// ISO date via the upper-bound variant — end-of-day UTC (start of next
	// day) so --as-of callers' `captured_at <= ?` upper bound includes the
	// named day. See PATCH exchangerate-as-of-date-inclusive.
	got, err = parseDurationOrDateUpperBound("2024-03-27")
	if err != nil {
		t.Fatalf("parseDurationOrDateUpperBound(\"2024-03-27\"): %v", err)
	}
	if got.Year() != 2024 || got.Month() != 3 || got.Day() != 28 || got.Hour() != 0 {
		t.Errorf("parseDurationOrDateUpperBound(\"2024-03-27\"): got %v, want 2024-03-28 00:00:00 UTC", got)
	}

	// Upper-bound variant must NOT shift duration shortcuts or RFC3339 —
	// those caller asked for a precise instant.
	gotUB, _ := parseDurationOrDateUpperBound("24h")
	gotLB, _ := parseDurationOrDate("24h")
	if !gotUB.Equal(gotLB) {
		t.Errorf("parseDurationOrDateUpperBound(\"24h\") should equal parseDurationOrDate(\"24h\"); got UB=%v LB=%v", gotUB, gotLB)
	}
	gotUB, _ = parseDurationOrDateUpperBound("2024-03-27T10:00:00Z")
	gotLB, _ = parseDurationOrDate("2024-03-27T10:00:00Z")
	if !gotUB.Equal(gotLB) {
		t.Errorf("parseDurationOrDateUpperBound(rfc3339) should equal parseDurationOrDate(rfc3339); got UB=%v LB=%v", gotUB, gotLB)
	}

	// RFC3339.
	got, err = parseDurationOrDate("2024-03-27T10:00:00Z")
	if err != nil {
		t.Fatalf("parseDurationOrDate(rfc3339): %v", err)
	}
	if got.Year() != 2024 || got.Hour() != 10 {
		t.Errorf("parseDurationOrDate(rfc3339): got %v", got)
	}

	// time.ParseDuration fallback (e.g., "1h30m").
	got, err = parseDurationOrDate("1h30m")
	if err != nil {
		t.Fatalf("parseDurationOrDate(\"1h30m\"): %v", err)
	}
	delta := now.Sub(got)
	want := 90 * time.Minute
	if delta < want-5*time.Second || delta > want+5*time.Second {
		t.Errorf("parseDurationOrDate(\"1h30m\"): got delta=%v, want=%v", delta, want)
	}

	// Garbage.
	if _, err := parseDurationOrDate("not-a-time"); err == nil {
		t.Errorf("parseDurationOrDate(\"not-a-time\"): expected err, got nil")
	}
}

func TestNextRefreshDate(t *testing.T) {
	// Today is the 17th, refresh day is 17 → next refresh is the 17th of next month.
	now := time.Date(2026, time.May, 17, 12, 0, 0, 0, time.UTC)
	got := nextRefreshDate(now, 17)
	want := time.Date(2026, time.June, 17, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("nextRefreshDate(May 17, day=17): got %v, want %v", got, want)
	}

	// Refresh day is the 20th, today is the 17th → next refresh is the 20th of THIS month.
	got = nextRefreshDate(now, 20)
	want = time.Date(2026, time.May, 20, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("nextRefreshDate(May 17, day=20): got %v, want %v", got, want)
	}

	// Refresh day is the 5th, today is the 17th → next refresh is the 5th of next month.
	got = nextRefreshDate(now, 5)
	want = time.Date(2026, time.June, 5, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("nextRefreshDate(May 17, day=5): got %v, want %v", got, want)
	}
}

func TestTierRank(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"unknown", 0},
		{"Free", 1},
		{"Pro", 2},
		{"Business", 3},
		{"Volume", 4},
	}
	for _, tc := range cases {
		if got := tierRank(tc.in); got != tc.want {
			t.Errorf("tierRank(%q): got %d, want %d", tc.in, got, tc.want)
		}
	}
	// Ranking ordering: Free < Pro < Business < Volume
	if tierRank("Free") >= tierRank("Pro") || tierRank("Pro") >= tierRank("Business") || tierRank("Business") >= tierRank("Volume") {
		t.Error("tier rank ordering broken")
	}
}

func TestLastN(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"abcdef", 4, "cdef"},
		{"abc", 4, "abc"},
		{"", 4, ""},
		{"fake0000000000000fake37ac", 4, "37ac"},
		{"x", 1, "x"},
	}
	for _, tc := range cases {
		if got := lastN(tc.in, tc.n); got != tc.want {
			t.Errorf("lastN(%q, %d): got %q, want %q", tc.in, tc.n, got, tc.want)
		}
	}
}

// TestCrossRateComputation verifies the math in matrix.go inline. Cross-rate
// derivation from a base must be self-consistent: matrix[X][X] == 1, and
// matrix[A][B] * matrix[B][A] should round-trip to 1 within float error.
func TestCrossRateComputation(t *testing.T) {
	// Simulate /latest USD with three target rates.
	rates := map[string]float64{
		"USD": 1.0, // base self-rate (added in matrix.go after unmarshal)
		"EUR": 0.8597,
		"GBP": 0.7495,
		"JPY": 158.6088,
	}
	codes := []string{"USD", "EUR", "GBP", "JPY"}
	matrix := make(map[string]map[string]float64, len(codes))
	for _, from := range codes {
		row := make(map[string]float64, len(codes))
		for _, to := range codes {
			row[to] = rates[to] / rates[from]
		}
		matrix[from] = row
	}

	// Self-rates always 1.
	for _, c := range codes {
		if matrix[c][c] != 1.0 {
			t.Errorf("matrix[%s][%s] = %f, want 1.0", c, c, matrix[c][c])
		}
	}

	// Round-trip: matrix[A][B] * matrix[B][A] ≈ 1.
	for _, a := range codes {
		for _, b := range codes {
			rt := matrix[a][b] * matrix[b][a]
			if math.Abs(rt-1.0) > 1e-9 {
				t.Errorf("round-trip matrix[%s][%s] * matrix[%s][%s] = %f, want 1.0", a, b, b, a, rt)
			}
		}
	}

	// Cross-rate consistency: matrix[A][C] should equal matrix[A][B] * matrix[B][C].
	// USD -> JPY should equal USD -> EUR * EUR -> JPY.
	expected := matrix["USD"]["EUR"] * matrix["EUR"]["JPY"]
	got := matrix["USD"]["JPY"]
	if math.Abs(got-expected) > 1e-9 {
		t.Errorf("cross-rate consistency: USD->JPY = %f, USD->EUR * EUR->JPY = %f", got, expected)
	}
}
