package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-orders/internal/parser"
)

func TestDetectRecurringPurchases(t *testing.T) {
	orders := []parser.OrderSummary{
		// AAA "Coffee" — bought on a steady ~30-day cadence (regular).
		{PlacedDate: "2026-01-01", ASINs: []string{"AAA"}, ItemTitles: []string{"Coffee"}},
		{PlacedDate: "2026-01-31", ASINs: []string{"AAA"}, ItemTitles: []string{"Coffee"}},
		{PlacedDate: "2026-03-02", ASINs: []string{"AAA"}, ItemTitles: []string{"Coffee"}},
		{PlacedDate: "2026-04-01", ASINs: []string{"AAA"}, ItemTitles: []string{"Coffee"}},
		// BBB "Cable" — bought twice, far apart (repeat but not a regular cadence).
		{PlacedDate: "2026-01-01", ASINs: []string{"BBB"}, ItemTitles: []string{"Cable"}},
		{PlacedDate: "2026-06-01", ASINs: []string{"BBB"}, ItemTitles: []string{"Cable"}},
		// CCC — one-off, must be excluded at min-occurrences 2.
		{PlacedDate: "2026-02-15", ASINs: []string{"CCC"}, ItemTitles: []string{"Charger"}},
		// Unparseable date — must be skipped, not panic.
		{PlacedDate: "", ASINs: []string{"AAA"}, ItemTitles: []string{"Coffee"}},
	}

	got := detectRecurringPurchases(orders, 2)

	if len(got) != 2 {
		t.Fatalf("got %d recurring items, want 2: %+v", len(got), got)
	}

	// Regular cadence sorts first.
	coffee := got[0]
	if coffee.ASIN != "AAA" {
		t.Fatalf("first item = %q, want AAA (regular cadence sorts first)", coffee.ASIN)
	}
	if coffee.Occurrences != 4 {
		t.Errorf("Coffee occurrences = %d, want 4", coffee.Occurrences)
	}
	if coffee.AvgIntervalDays != 30 {
		t.Errorf("Coffee avgIntervalDays = %d, want 30", coffee.AvgIntervalDays)
	}
	if !coffee.Regular {
		t.Errorf("Coffee Regular = false, want true")
	}
	if coffee.FirstOrdered != "2026-01-01" || coffee.LastOrdered != "2026-04-01" {
		t.Errorf("Coffee range = %s..%s, want 2026-01-01..2026-04-01", coffee.FirstOrdered, coffee.LastOrdered)
	}
	if coffee.NextExpected != "2026-05-01" {
		t.Errorf("Coffee nextExpected = %q, want 2026-05-01", coffee.NextExpected)
	}

	cable := got[1]
	if cable.ASIN != "BBB" {
		t.Errorf("second item = %q, want BBB", cable.ASIN)
	}
	if cable.Regular {
		t.Errorf("Cable Regular = true, want false (single, wide interval)")
	}
}

func TestDetectRecurringPurchasesRespectsMinOccurrences(t *testing.T) {
	orders := []parser.OrderSummary{
		{PlacedDate: "2026-01-01", ASINs: []string{"AAA"}, ItemTitles: []string{"Coffee"}},
		{PlacedDate: "2026-02-01", ASINs: []string{"AAA"}, ItemTitles: []string{"Coffee"}},
	}
	if got := detectRecurringPurchases(orders, 3); len(got) != 0 {
		t.Errorf("with min-occurrences 3 and 2 purchases, got %d items, want 0", len(got))
	}
}
