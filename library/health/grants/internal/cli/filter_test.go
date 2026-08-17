package cli

import (
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/grants/internal/sources"
)

func TestParseMoney(t *testing.T) {
	cases := []struct {
		in   any
		want int64
	}{
		{"$1,500,000", 1500000},
		{"500000.00", 500000},
		{"  $250,000 ", 250000},
		{float64(750000), 750000},
		{nil, 0},
		{"none", 0},
		{"", 0},
		{true, 0},
	}
	for _, c := range cases {
		if got := sources.ParseMoney(c.in); got != c.want {
			t.Errorf("ParseMoney(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestClosingBefore(t *testing.T) {
	opps := []sources.Opportunity{
		{Number: "A", CloseDate: "08/01/2026"},
		{Number: "B", CloseDate: "12/31/2026"},
		{Number: "C", CloseDate: ""}, // no close date, so it is dropped
		{Number: "D", CloseDate: "07/15/2026"},
	}
	cutoff, _ := time.Parse("2006-01-02", "2026-09-01")
	got := ClosingBefore(opps, cutoff)
	if len(got) != 2 || got[0].Number != "A" || got[1].Number != "D" {
		t.Fatalf("ClosingBefore: got %+v, want [A D]", got)
	}
}

func TestEligibilityMatches(t *testing.T) {
	types := []string{"Public and State controlled institutions of higher education", "Small businesses"}
	if !EligibilityMatches(types, "small business") {
		t.Error("expected 'small business' to match")
	}
	if !EligibilityMatches(types, "") {
		t.Error("empty query must match everything")
	}
	if EligibilityMatches(types, "individuals") {
		t.Error("'individuals' must not match")
	}
}

func TestAwardCap(t *testing.T) {
	if got := (sources.OppDetails{AwardCeiling: 500000, EstimatedFunding: 9}).AwardCap(); got != 500000 {
		t.Errorf("ceiling wins: got %d, want 500000", got)
	}
	if got := (sources.OppDetails{AwardCeiling: 0, EstimatedFunding: 300000}).AwardCap(); got != 300000 {
		t.Errorf("fallback to estimated: got %d, want 300000", got)
	}
	if got := (sources.OppDetails{}).AwardCap(); got != 0 {
		t.Errorf("empty: got %d, want 0", got)
	}
}

func TestFormatMoney(t *testing.T) {
	cases := map[int64]string{0: "—", 999: "$999", 1000: "$1,000", 1234567: "$1,234,567"}
	for in, want := range cases {
		if got := FormatMoney(in); got != want {
			t.Errorf("FormatMoney(%d) = %s, want %s", in, got, want)
		}
	}
}
