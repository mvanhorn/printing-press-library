// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"math"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/woocommerce/internal/store"
)

func mkOrder(id, status string, total float64, age time.Duration, now time.Time, customer int64, gateway string, items ...wooLineItem) wooOrder {
	t := now.Add(-age)
	return wooOrder{
		ID: id, Status: status, Total: total,
		CustomerID: customer, PaymentMethod: gateway,
		Created: t, CreatedOK: true, Modified: t, ModifiedOK: true,
		LineItems: items,
	}
}

func TestBuildTriageViewBuckets(t *testing.T) {
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	orders := []wooOrder{
		mkOrder("1", "processing", 10, 48*time.Hour, now, 1, "Stripe"), // stuck
		mkOrder("2", "processing", 10, 2*time.Hour, now, 1, "Stripe"),  // fresh, not stuck
		mkOrder("3", "on-hold", 20, 72*time.Hour, now, 2, "PayPal"),    // aging
		mkOrder("4", "failed", 30, 24*time.Hour, now, 3, "Stripe"),     // recent failure
		mkOrder("5", "failed", 30, 40*24*time.Hour, now, 3, "Stripe"),  // old failure, not recent
		mkOrder("6", "completed", 40, 5*time.Hour, now, 4, "Stripe"),
	}
	v := buildTriageView(orders, 24*time.Hour, 50, now)

	got := map[string]int{}
	for _, b := range v.Buckets {
		got[b.Bucket] = b.Count
	}
	if got["stuck_processing"] != 1 {
		t.Fatalf("stuck_processing = %d, want 1", got["stuck_processing"])
	}
	if got["aging_on_hold"] != 1 {
		t.Fatalf("aging_on_hold = %d, want 1", got["aging_on_hold"])
	}
	if got["failed_recent"] != 1 {
		t.Fatalf("failed_recent = %d, want 1 (the 40-day-old failure must be excluded)", got["failed_recent"])
	}
	if v.ScannedOrders != len(orders) {
		t.Fatalf("ScannedOrders = %d, want %d", v.ScannedOrders, len(orders))
	}
}

func TestBuildTriageViewEmptyIsHonest(t *testing.T) {
	v := buildTriageView(nil, 24*time.Hour, 10, time.Now().UTC())
	if v.Note == "" {
		t.Fatal("empty input must set an explanatory note, not silently return zeroes")
	}
	for _, b := range v.Buckets {
		if b.Orders == nil {
			t.Fatalf("bucket %q Orders is nil; must be an empty slice so JSON renders []", b.Bucket)
		}
	}
}

func TestBuildVelocityViewMath(t *testing.T) {
	now := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	sold := map[soldKey]int{{productID: "10"}: 60}
	stock := map[soldKey]int64{{productID: "10"}: 30}
	labels := map[soldKey]productLabel{{productID: "10"}: {Name: "Widget", SKU: "W-1"}}

	v := buildVelocityView(sold, stock, labels, 30*24*time.Hour, "30d", 12, 50, now)
	if len(v.Items) != 1 {
		t.Fatalf("len(Items) = %d, want 1", len(v.Items))
	}
	it := v.Items[0]
	if it.UnitsPerDay != 2 {
		t.Fatalf("UnitsPerDay = %v, want 2 (60 units / 30 days)", it.UnitsPerDay)
	}
	if it.DaysOfCover == nil || *it.DaysOfCover != 15 {
		t.Fatalf("DaysOfCover = %v, want 15 (30 in stock / 2 per day)", it.DaysOfCover)
	}
	if it.StockoutDate != "2026-08-05" {
		t.Fatalf("StockoutDate = %q, want 2026-08-05", it.StockoutDate)
	}
}

func TestBuildVelocityViewUnknownStockHasNullCover(t *testing.T) {
	now := time.Now().UTC()
	sold := map[soldKey]int{{productID: "10"}: 10}
	v := buildVelocityView(sold, map[soldKey]int64{}, map[soldKey]productLabel{}, 10*24*time.Hour, "10d", 3, 50, now)
	if v.Items[0].DaysOfCover != nil {
		t.Fatal("DaysOfCover must be null when stock is unknown, not a fabricated number")
	}
	if v.Items[0].StockoutDate != "" {
		t.Fatal("StockoutDate must be empty when cover is unknown")
	}
}

// TestRevenueDecompositionIdentity is the load-bearing invariant: the
// order-count and basket-size contributions must sum exactly to the revenue
// delta, or the decomposition is lying about causes.
func TestRevenueDecompositionIdentity(t *testing.T) {
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	window := 7 * 24 * time.Hour
	orders := []wooOrder{
		mkOrder("c1", "completed", 100, 1*24*time.Hour, now, 1, "Stripe"),
		mkOrder("c2", "completed", 50, 2*24*time.Hour, now, 2, "Stripe"),
		mkOrder("c3", "processing", 30, 3*24*time.Hour, now, 3, "Stripe"),
		mkOrder("p1", "completed", 200, 9*24*time.Hour, now, 1, "Stripe"),
		mkOrder("p2", "completed", 120, 10*24*time.Hour, now, 4, "Stripe"),
	}
	v := buildRevenueExplainView(orders, map[string]productLabel{}, window, now)

	var countC, basketC float64
	for _, c := range v.Contributions {
		switch c.Factor {
		case "order_count":
			countC = c.Amount
		case "basket_size":
			basketC = c.Amount
		}
	}
	if diff := math.Abs((countC + basketC) - v.Delta.Revenue); diff > 0.02 {
		t.Fatalf("order_count(%v) + basket_size(%v) = %v, want revenue delta %v (diff %v)",
			countC, basketC, countC+basketC, v.Delta.Revenue, diff)
	}
	if v.Current.Orders != 3 || v.Prior.Orders != 2 {
		t.Fatalf("period order counts = (%d, %d), want (3, 2)", v.Current.Orders, v.Prior.Orders)
	}
}

func TestRevenueExplainExcludesNonSettledStatuses(t *testing.T) {
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	orders := []wooOrder{
		mkOrder("a", "completed", 100, 1*24*time.Hour, now, 1, "Stripe"),
		mkOrder("b", "cancelled", 999, 1*24*time.Hour, now, 2, "Stripe"),
		mkOrder("c", "failed", 999, 1*24*time.Hour, now, 3, "Stripe"),
	}
	v := buildRevenueExplainView(orders, map[string]productLabel{}, 7*24*time.Hour, now)
	if v.Current.Revenue != 100 {
		t.Fatalf("Revenue = %v, want 100 (cancelled and failed orders must not count)", v.Current.Revenue)
	}
}

func TestBuildRefundRateView(t *testing.T) {
	sold := map[string]int{"10": 100, "20": 5}
	refunded := map[string]int{"10": 9}
	value := map[string]float64{"10": 404.10}
	labels := map[string]productLabel{"10": {Name: "Jacket", SKU: "J-1"}}

	v := buildRefundRateView(sold, refunded, value, labels, "90d", "line_item", 300, 50, 1)
	if len(v.Items) != 2 {
		t.Fatalf("len(Items) = %d, want 2", len(v.Items))
	}
	if v.Items[0].ProductID != "10" || v.Items[0].RefundRate != 0.09 {
		t.Fatalf("top item = %+v, want product 10 at rate 0.09", v.Items[0])
	}
	if v.Items[0].Name != "Jacket" {
		t.Fatalf("Name = %q, want Jacket", v.Items[0].Name)
	}
}

func TestBuildRefundRateViewMinUnitsFilter(t *testing.T) {
	sold := map[string]int{"10": 100, "20": 2}
	v := buildRefundRateView(sold, map[string]int{}, map[string]float64{}, map[string]productLabel{}, "90d", "line_item", 10, 50, 10)
	if len(v.Items) != 1 {
		t.Fatalf("len(Items) = %d, want 1 (product 20 has fewer than --min-units)", len(v.Items))
	}
	if v.Note == "" {
		t.Fatal("expected a note when no refunds were recorded")
	}
}

func TestBuildLtvViewCohorts(t *testing.T) {
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	mar := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	orders := []wooOrder{
		{ID: "1", Status: "completed", Total: 100, CustomerID: 1, Created: mar, CreatedOK: true},
		{ID: "2", Status: "completed", Total: 50, CustomerID: 1, Created: mar.AddDate(0, 1, 0), CreatedOK: true},
		{ID: "3", Status: "completed", Total: 80, CustomerID: 2, Created: mar, CreatedOK: true},
		{ID: "4", Status: "completed", Total: 20, CustomerID: 0, Created: now, CreatedOK: true},
	}
	v := buildLtvView(orders, map[string]customerProfile{}, "month", 50)

	var march *ltvCohort
	for i := range v.Cohorts {
		if v.Cohorts[i].Cohort == "2026-03" {
			march = &v.Cohorts[i]
		}
	}
	if march == nil {
		t.Fatalf("missing 2026-03 cohort, got %+v", v.Cohorts)
	}
	if march.Customers != 2 || march.RepeatCustomers != 1 {
		t.Fatalf("march cohort = %+v, want 2 customers / 1 repeat", *march)
	}
	if march.RepeatRate != 0.5 {
		t.Fatalf("RepeatRate = %v, want 0.5", march.RepeatRate)
	}
	// The guest order in this fixture carries no billing email, so it cannot be
	// attributed to a person and must be excluded from the aggregates rather
	// than collapsed into a synthetic "guest" customer.
	if v.UnidentifiedGuestOrders != 1 {
		t.Fatalf("unidentified_guest_orders = %d, want 1", v.UnidentifiedGuestOrders)
	}
	for _, c := range v.Customers {
		if c.CustomerID == "guest" {
			t.Fatalf("unattributable guest orders must not become a synthetic customer, got %+v", c)
		}
	}
	if v.Totals.Customers != 2 {
		t.Fatalf("totals.customers = %d, want 2 real customers", v.Totals.Customers)
	}
}

func TestBuildCatalogDiffView(t *testing.T) {
	old := []store.CatalogSnapshotRow{
		{ProductID: "1", Name: "A", Price: 50, RegularPrice: 50, OnSale: false, InStock: true},
		{ProductID: "2", Name: "B", Price: 10, InStock: true},
	}
	nw := []store.CatalogSnapshotRow{
		{ProductID: "1", Name: "A", Price: 40, RegularPrice: 50, SalePrice: 40, OnSale: true, InStock: false},
		{ProductID: "3", Name: "C", Price: 99, InStock: true},
	}
	v := buildCatalogDiffView("https://s", "2026-06-01T00:00:00Z", "2026-07-01T00:00:00Z", old, nw, 100)

	if v.Summary.Added != 1 || v.Summary.Removed != 1 {
		t.Fatalf("added/removed = %d/%d, want 1/1", v.Summary.Added, v.Summary.Removed)
	}
	if v.Summary.PriceChanges != 1 || v.PriceChanges[0].Change != -10 {
		t.Fatalf("price change = %+v, want a single -10 move", v.PriceChanges)
	}
	if v.Summary.SaleStarted != 1 {
		t.Fatalf("SaleStarted = %d, want 1", v.Summary.SaleStarted)
	}
	if v.Summary.StockFlips != 1 || v.StockFlips[0].ToInStock {
		t.Fatalf("stock flip = %+v, want one flip to out-of-stock", v.StockFlips)
	}
	if v.SpanDays != 30 {
		t.Fatalf("SpanDays = %v, want 30", v.SpanDays)
	}
}

func TestBuildCatalogDiffViewEmptySlicesNotNil(t *testing.T) {
	v := buildCatalogDiffView("https://s", "", "", nil, nil, 10)
	if v.Added == nil || v.Removed == nil || v.PriceChanges == nil ||
		v.SaleStarted == nil || v.SaleEnded == nil || v.StockFlips == nil {
		t.Fatal("diff slices must be initialized so JSON renders [] rather than null")
	}
}

func TestBuildCatalogAuditViewRules(t *testing.T) {
	products := []auditProduct{
		{ID: "1", Status: "publish", SKU: "", Price: "10", StockStatus: "instock", HasImages: true, HasCategories: true, Description: "d"},
		{ID: "2", Status: "publish", SKU: "S", Price: "0", StockStatus: "outofstock", HasImages: false, HasCategories: false},
		{ID: "3", Status: "publish", SKU: "V", Price: "20", StockStatus: "instock", HasImages: true, HasCategories: true, Description: "d", Type: "variable"},
	}
	v := buildCatalogAuditView(products, map[string]int{}, []string{"90"}, 1, "", 50)

	counts := map[string]int{}
	for _, r := range v.Rules {
		counts[r.Rule] = r.Count
	}
	if counts["missing_sku"] != 1 {
		t.Fatalf("missing_sku = %d, want 1", counts["missing_sku"])
	}
	if counts["zero_price"] != 1 {
		t.Fatalf("zero_price = %d, want 1", counts["zero_price"])
	}
	if counts["out_of_stock_published"] != 1 {
		t.Fatalf("out_of_stock_published = %d, want 1", counts["out_of_stock_published"])
	}
	if counts["variable_without_variations"] != 1 {
		t.Fatalf("variable_without_variations = %d, want 1", counts["variable_without_variations"])
	}
	if counts["variation_missing_price"] != 1 {
		t.Fatalf("variation_missing_price = %d, want 1", counts["variation_missing_price"])
	}
	if v.ScannedProducts != 3 {
		t.Fatalf("ScannedProducts = %d, want 3", v.ScannedProducts)
	}
	// Every rule must appear even at zero so the output shape is stable.
	if len(v.Rules) != len(auditRuleDescriptions) {
		t.Fatalf("len(Rules) = %d, want %d (zero-count rules must still be listed)", len(v.Rules), len(auditRuleDescriptions))
	}
}

func TestParseMoneyAndTime(t *testing.T) {
	if got := parseMoney("49.00"); got != 49 {
		t.Fatalf("parseMoney(\"49.00\") = %v, want 49", got)
	}
	if got := parseMoney(""); got != 0 {
		t.Fatalf("parseMoney(\"\") = %v, want 0", got)
	}
	if got := parseMoney("not-a-number"); got != 0 {
		t.Fatalf("parseMoney(garbage) = %v, want 0", got)
	}
	for _, in := range []string{"2026-07-21T06:11:11", "2026-07-21T06:11:11Z", "2026-07-21"} {
		if _, ok := parseWooTime(in); !ok {
			t.Fatalf("parseWooTime(%q) failed; WooCommerce emits all of these shapes", in)
		}
	}
	if _, ok := parseWooTime(""); ok {
		t.Fatal("parseWooTime(\"\") must report failure")
	}
}

// --- regression tests for defects found in pre-publish review ---

// A $50 refund on a $500 single-line order of 10 units must report ~1 unit
// refunded, not all 10. The bug scaled units by the line's share of the order
// (1.0) instead of by the refunded fraction (0.1).
func TestRefundProportionalUnitsScaleByRefundFraction(t *testing.T) {
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	o := mkOrder("100", "completed", 500, 24*time.Hour, now, 1, "Stripe",
		wooLineItem{ProductID: 7, Quantity: 10, Total: "500.00"})
	o.Refunds = []wooOrderRefundStub{{Total: "-50.00"}}

	units, value := proportionalRefundAttribution([]wooOrder{o}, map[string]bool{"100": true})
	if got := units["7"]; got != 1 {
		t.Fatalf("units refunded = %d, want 1 (a 10%% refund of 10 units); the bug reported 10", got)
	}
	if got := round2(value["7"]); got != 50 {
		t.Fatalf("refunded value = %v, want 50", got)
	}
}

// Guests are distinct people. Collapsing them into one synthetic customer made
// a guest-checkout store report customers=1 with the whole store's revenue.
func TestLtvKeepsGuestsDistinctByEmail(t *testing.T) {
	base := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	mk := func(id string, total float64, email string, day int) wooOrder {
		return wooOrder{ID: id, Status: "completed", Total: total, CustomerID: 0,
			Created: base.AddDate(0, 0, day), CreatedOK: true, BillingEmail: email}
	}
	orders := []wooOrder{
		mk("1", 100, "a@example.com", 0),
		mk("2", 200, "b@example.com", 1),
		mk("3", 300, "c@example.com", 2),
		mk("4", 400, "", 3), // unattributable
	}
	v := buildLtvView(orders, map[string]customerProfile{}, "month", 50)

	if v.Totals.Customers != 3 {
		t.Fatalf("totals.customers = %d, want 3 distinct guests (bug collapsed them to 1)", v.Totals.Customers)
	}
	if v.Totals.RepeatRate != 0 {
		t.Fatalf("repeat_rate = %v, want 0; collapsing guests fabricated repeat purchases", v.Totals.RepeatRate)
	}
	if v.UnidentifiedGuestOrders != 1 {
		t.Fatalf("unidentified_guest_orders = %d, want 1", v.UnidentifiedGuestOrders)
	}
	if v.Note == "" {
		t.Fatal("excluding an unattributable order must be disclosed in note")
	}
	if v.Totals.AvgLTV != 200 {
		t.Fatalf("avg_ltv = %v, want 200 ((100+200+300)/3); the $400 unattributable order must be excluded", v.Totals.AvgLTV)
	}
}

// Diffing a page-capped snapshot against a fuller one used to report every
// unseen product as "added". Membership must be withheld instead.
func TestCatalogDiffWithholdsMembershipOnTruncatedSnapshot(t *testing.T) {
	old := []store.CatalogSnapshotRow{
		{ProductID: "1", Name: "A", Price: 50, InStock: true, Truncated: true},
	}
	nw := []store.CatalogSnapshotRow{
		{ProductID: "1", Name: "A", Price: 40, InStock: true},
		{ProductID: "2", Name: "B", Price: 10, InStock: true},
	}
	v := buildCatalogDiffView("https://s", "2026-06-01T00:00:00Z", "2026-07-01T00:00:00Z", old, nw, 100)

	if v.MembershipComparable {
		t.Fatal("MembershipComparable must be false when a snapshot is truncated")
	}
	if len(v.Added) != 0 || len(v.Removed) != 0 {
		t.Fatalf("added/removed must be withheld on a truncated snapshot, got %d/%d", len(v.Added), len(v.Removed))
	}
	if v.Warning == "" {
		t.Fatal("a withheld comparison must carry an explicit warning")
	}
	// Price movement for products in BOTH snapshots is still valid.
	if len(v.PriceChanges) != 1 || v.PriceChanges[0].Change != -10 {
		t.Fatalf("price changes must survive truncation, got %+v", v.PriceChanges)
	}
}

func TestCatalogDiffComparableWhenBothComplete(t *testing.T) {
	old := []store.CatalogSnapshotRow{{ProductID: "1", Name: "A", Price: 50, InStock: true}}
	nw := []store.CatalogSnapshotRow{{ProductID: "2", Name: "B", Price: 10, InStock: true}}
	v := buildCatalogDiffView("https://s", "2026-06-01T00:00:00Z", "2026-07-01T00:00:00Z", old, nw, 100)
	if !v.MembershipComparable || v.Warning != "" {
		t.Fatal("two complete snapshots must compare membership normally")
	}
	if len(v.Added) != 1 || len(v.Removed) != 1 {
		t.Fatalf("added/removed = %d/%d, want 1/1", len(v.Added), len(v.Removed))
	}
}

// Converting a huge days-of-cover float to time.Duration overflows int64 and
// yielded stockout dates in 1677 (amd64) or 2318 (arm64).
func TestVelocityDoesNotProjectAbsurdStockoutDates(t *testing.T) {
	now := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	sold := map[soldKey]int{{productID: "1"}: 1}
	stock := map[soldKey]int64{{productID: "1"}: 3600} // ~108,000 days of cover
	v := buildVelocityView(sold, stock, map[soldKey]productLabel{}, 30*24*time.Hour, "30d", 1, 10, now)

	it := v.Items[0]
	if it.DaysOfCover == nil || *it.DaysOfCover < 100000 {
		t.Fatalf("DaysOfCover = %v, want the real (huge) value reported", it.DaysOfCover)
	}
	if it.StockoutDate != "" {
		t.Fatalf("StockoutDate = %q, want empty; an absurd projection must be omitted, not overflowed", it.StockoutDate)
	}
}
