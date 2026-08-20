// Copyright 2026 Brian Wishan and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-orders/internal/parser"
)

// PATCH(greptile-rollup-currency) coverage: rollupSpend must propagate the
// per-order Currency that parseOrderCard extracted (e.g. "GBP" on
// amazon.co.uk) instead of always reporting "USD".
func TestRollupSpend_PropagatesPerOrderCurrency(t *testing.T) {
	orders := []parser.OrderSummary{
		{OrderID: "111-1111111-1111111", PlacedDate: "2026-04-15", Total: 19.99, Currency: "GBP"},
		{OrderID: "222-2222222-2222222", PlacedDate: "2026-04-20", Total: 5.50, Currency: "GBP"},
		{OrderID: "333-3333333-3333333", PlacedDate: "2026-05-01", Total: 100.00, Currency: "EUR"},
	}

	buckets := rollupSpend(orders, "month")

	got := map[string]string{}
	for _, b := range buckets {
		got[b.Key] = b.Currency
	}

	if got["2026-04"] != "GBP" {
		t.Errorf("April bucket currency = %q, want %q", got["2026-04"], "GBP")
	}
	if got["2026-05"] != "EUR" {
		t.Errorf("May bucket currency = %q, want %q", got["2026-05"], "EUR")
	}
}

// Empty Currency on the first order in a bucket should fall back to "USD"
// so existing US-locale callers don't see "" in JSON output.
func TestRollupSpend_DefaultsToUSDWhenCurrencyEmpty(t *testing.T) {
	orders := []parser.OrderSummary{
		{OrderID: "111-1111111-1111111", PlacedDate: "2026-04-15", Total: 19.99, Currency: ""},
	}

	buckets := rollupSpend(orders, "month")

	if len(buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(buckets))
	}
	if buckets[0].Currency != "USD" {
		t.Errorf("bucket currency = %q, want %q (default fallback)", buckets[0].Currency, "USD")
	}
}
