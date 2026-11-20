// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
)

// TestNovelInventoryAuditHelpWires smoke-tests that the inventory audit command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelInventoryAuditHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"inventory", "audit", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("inventory audit --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "audit"} {
		if !strings.Contains(help, want) {
			t.Fatalf("inventory audit --help missing %q in output:\n%s", want, help)
		}
	}
}

// Inventory responses have no extractable id: stamp distinct synthetic ids on
// every mirror row so the real sync write path stores the fixtures.
func shopxAuditFixture(t *testing.T) *shopxFixture {
	f := shopxNewFixture(t)
	f.put("listings", map[string]any{"listing_id": 1000000001}, map[string]any{"listing_id": 1000000003})
	offer := func(id, qty, price int64) map[string]any {
		return map[string]any{"offering_id": id, "quantity": qty, "price": shopxMoneyObj(price, "USD")}
	}
	product := func(id int64, sku string, offerings ...map[string]any) map[string]any {
		return map[string]any{"product_id": id, "sku": sku, "offerings": offerings}
	}
	deleted := product(6, "", offer(6, 0, 1))
	deleted["is_deleted"] = true
	deletedOffer := offer(7, 0, 1)
	deletedOffer["is_deleted"] = true
	disabledOffer := offer(8, 0, 1)
	disabledOffer["is_enabled"] = false
	absentQty := offer(9, 0, 2000)
	delete(absentQty, "quantity")
	f.put("listings_inventory",
		map[string]any{"id": 1000000001, "listing_id": 1000000001, "products": []any{product(1, "APR-LIN-S", offer(1, 5, 2000)), product(2, "APR-LIN-M", offer(2, 0, 3000)), deleted, product(7, "SAFE", deletedOffer, disabledOffer, absentQty)}},
		map[string]any{"id": 1000000002, "parent_id": 1000000002, "product_id": 3, "sku": "  ", "offerings": []any{offer(3, 3, 1000)}},
		map[string]any{"id": 1000000004, "listing_id": 1000000002, "product_id": 4, "sku": "DUP-1", "offerings": []any{offer(4, 1, 1000), offer(10, 1, 2500)}},
		map[string]any{"id": 1000000003, "listing_id": 1000000003, "product_id": 5, "sku": " dup-1 ", "offerings": []any{offer(5, 1, 1000)}},
	)
	// An alias copy must not create additional findings or duplicate SKU pairs.
	f.put("listing_inventory", map[string]any{"id": 1000000001, "listing_id": 1000000001, "product_id": 2, "sku": "APR-LIN-M", "offerings": []any{offer(2, 0, 3000)}})
	// Inventory audit deliberately includes known inactive listings.
	f.put("listings", map[string]any{"listing_id": 1000000002, "title": "Linen apron", "state": "inactive"})
	return f
}

func TestNovelInventoryAuditFindings(t *testing.T) {
	for _, tc := range []struct {
		threshold string
		extra     bool
	}{{"0.5", false}, {"0.4", true}} {
		t.Run(tc.threshold, func(t *testing.T) {
			f := shopxAuditFixture(t)
			args := []string{"inventory", "audit"}
			if tc.extra {
				args = append(args, "--spread-threshold", tc.threshold)
			}
			out, _, err := shopxRun(t, f, args...)
			if err != nil {
				t.Fatal(err)
			}
			got := shopxMustList(t, out)
			wantJSON := `[
    {"type":"zero_quantity","listing_id":1000000001,"product_id":2,"sku":"APR-LIN-M","offering_id":2},
    {"type":"empty_sku","listing_id":1000000002,"product_id":3},
    {"type":"duplicate_sku","sku":"DUP-1","listing_ids":[1000000002,1000000003],"product_ids":[4,5]},
    {"type":"price_spread","listing_id":1000000002,"min_price":"10.00","max_price":"25.00","spread":1.5,"currency":"USD"}
   ]`
			// The lowest (listing, product) pair supplies the reported spelling.
			var want []map[string]any
			if err := json.Unmarshal([]byte(wantJSON), &want); err != nil {
				t.Fatal(err)
			}
			if tc.extra {
				want = append(want[:3], append([]map[string]any{{"type": "price_spread", "listing_id": float64(1000000001), "min_price": "20.00", "max_price": "30.00", "spread": 0.5, "currency": "USD"}}, want[3:]...)...)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("findings = %#v; want %#v", got, want)
			}
		})
	}
}

func TestNovelInventoryAuditEmptyAndValidation(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent", true: "empty"}[existing], func(t *testing.T) {
			var f *shopxFixture
			if existing {
				f = shopxNewFixture(t)
			}
			out, _, err := shopxRun(t, f, "inventory", "audit")
			if err != nil {
				t.Fatal(err)
			}
			if rows := shopxMustList(t, out); len(rows) != 0 {
				t.Fatalf("got %v, want []", rows)
			}
		})
	}
	_, _, err := shopxRun(t, nil, "inventory", "audit", "--spread-threshold", "-0.1")
	if err == nil {
		t.Fatal("negative threshold accepted")
	}
}

func TestNovelInventoryAuditDivisor(t *testing.T) {
	f := shopxNewFixture(t)
	f.put("listings", map[string]any{"listing_id": 1000000001})
	f.put("listings_inventory", map[string]any{"id": 1000000001, "parent_id": 1000000001, "product_id": 1, "sku": "APR-LIN-S", "offerings": []any{
		map[string]any{"price": map[string]any{"amount": 1000, "divisor": 1000, "currency_code": "USD"}},
		map[string]any{"price": map[string]any{"amount": 2000, "divisor": 1000, "currency_code": "USD"}},
	}})
	out, _, err := shopxRun(t, f, "inventory", "audit")
	if err != nil {
		t.Fatal(err)
	}
	rows := shopxMustList(t, out)
	if len(rows) != 1 || rows[0]["type"] != "price_spread" || rows[0]["min_price"] != "1.000" || rows[0]["max_price"] != "2.000" || rows[0]["spread"] != float64(1) {
		t.Fatalf("unexpected findings: %v", rows)
	}
}
