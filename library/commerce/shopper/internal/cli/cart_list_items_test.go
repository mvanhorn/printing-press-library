// Copyright 2026 educrvz and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for cart_list_items.go — PATCH: cart-list-items.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// liveCartListBody is a trimmed but shape-faithful GET /cart/list response,
// captured from the live API on 2026-09-22. The in-basket count is
// `cartQuantity` (the cart reuses the catalog product object); `quantity` on a
// department is the department's own roll-up, not a product count.
const liveCartListBody = `{
  "discount": "R$ 0,00", "has_discount": false,
  "total_items": 4, "total_value": "R$ 27,96", "final_value": 19.9,
  "show_paused": false, "show_restore_button": false, "show_open_cart": true,
  "recurrent_cart_id": null,
  "departments": [
    {"id": 22, "name": "Alimentos", "icon": "https://cdn/alimentos.svg", "quantity": 3, "products": [
      {"id": 36756, "name": "ARROZ      INTEGRAL 1KG ", "description": "...",
       "image": "https://cdn/919524_000.jpg", "price": "R$ 6,99",
       "cartQuantity": 3, "maxCartQuantity": 10, "sku": "919524",
       "taxonomy": ["Alimentos", "Arroz, Feijao e Graos"], "outOfStock": false,
       "priceRaw": 6.99, "paused": false, "substitute": null,
       "fullUrl": "https://unica.shopper.com.br"}
    ], "suggestions": []},
    {"id": 21, "name": "Bebidas", "icon": "https://cdn/bebidas.svg", "quantity": 1, "products": [
      {"id": 4011, "name": "SUCO DE UVA 1L", "price": "R$ 6,99",
       "cartQuantity": 1, "maxCartQuantity": 6, "sku": "111",
       "outOfStock": true, "priceRaw": 6.99, "paused": false}
    ], "suggestions": []},
    {"id": 23, "name": "Limpeza", "icon": "https://cdn/limpeza.svg", "quantity": 0, "products": [], "suggestions": []}
  ],
  "paused": {"quantity": 1, "name": "Itens pausados", "icon": "https://cdn/pause.svg", "products": [
    {"id": 777, "name": "SABAO EM PO 1KG", "price": "R$ 12,00", "sku": "999",
     "cartQuantity": 2, "maxCartQuantity": 4, "outOfStock": false,
     "priceRaw": 12.0, "paused": true}
  ]}
}`

func decodeCartList(t *testing.T) cartListResponse {
	t.Helper()
	var resp cartListResponse
	if err := json.Unmarshal([]byte(liveCartListBody), &resp); err != nil {
		t.Fatalf("decode /cart/list body: %v", err)
	}
	return resp
}

// TestCartListDecodesLiveShape pins the field names against the real payload.
// GET /cart/summary carries no item array at all, which is why this command
// reads /cart/list instead — decoding the wrong key would silently produce an
// empty basket, the exact failure mode this patch removes.
func TestCartListDecodesLiveShape(t *testing.T) {
	resp := decodeCartList(t)
	if resp.TotalItems != 4 || resp.TotalValue != "R$ 27,96" {
		t.Errorf("totals = %v/%q, want 4/\"R$ 27,96\"", resp.TotalItems, resp.TotalValue)
	}
	// recurrent_cart_id is null on one-off storefronts and a number on the
	// recurring ones; null must decode to 0 rather than failing the whole read.
	if resp.RecurrentCartID != 0 {
		t.Errorf("recurrent_cart_id = %d, want 0 for a null value", resp.RecurrentCartID)
	}
	// sku is a STRING in /cart/list even though the catalog returns it as a
	// number — decoding it as an integer fails the whole basket read.
	if resp.Departments[0].Products[0].SKU != "919524" {
		t.Errorf("sku = %q, want \"919524\"", resp.Departments[0].Products[0].SKU)
	}
	if len(resp.Departments) != 3 {
		t.Fatalf("departments = %d, want 3", len(resp.Departments))
	}
	arroz := resp.Departments[0].Products[0]
	if arroz.CartQuantity != 3 {
		t.Errorf("cartQuantity = %v, want 3 (NOT the department quantity)", arroz.CartQuantity)
	}
	if arroz.PriceRaw != 6.99 {
		t.Errorf("priceRaw = %v, want 6.99", arroz.PriceRaw)
	}
	if resp.Paused == nil || len(resp.Paused.Products) != 1 {
		t.Fatal("paused bucket did not decode")
	}
}

// TestCartItemFlatteningSeparatesPausedLines checks the flattening rules:
// empty departments drop out, active lines sort by line total, out-of-stock is
// preserved, and paused products stay out of both the active list and its value.
func TestCartItemFlatteningSeparatesPausedLines(t *testing.T) {
	resp := decodeCartList(t)
	var view cartItemsView
	view.Items = []cartItem{}
	for _, dept := range resp.Departments {
		for _, p := range dept.Products {
			view.Items = append(view.Items, newCartItem(p, dept.Name, false))
			view.ItemsValue += p.PriceRaw * p.CartQuantity
		}
	}
	for _, p := range resp.Paused.Products {
		view.PausedItems = append(view.PausedItems, newCartItem(p, resp.Paused.Name, true))
	}

	if len(view.Items) != 2 {
		t.Fatalf("active items = %d, want 2 (the empty department must not produce a row)", len(view.Items))
	}
	if len(view.PausedItems) != 1 || !view.PausedItems[0].Paused {
		t.Fatal("paused line missing or not flagged")
	}
	// 3 x 6.99 + 1 x 6.99 = 27.96; the paused 2 x 12.00 must NOT be counted.
	if got := round2(view.ItemsValue); got != 27.96 {
		t.Errorf("items_value = %v, want 27.96 (paused lines excluded)", got)
	}
	if got := round2(view.Items[0].LineTotal); got != 20.97 {
		t.Errorf("line_total = %v, want 20.97", got)
	}
	if !view.Items[1].OutOfStock {
		t.Error("out-of-stock flag was dropped")
	}
	if view.Items[0].Name != "ARROZ INTEGRAL 1KG" {
		t.Errorf("name = %q, want padding spaces collapsed", view.Items[0].Name)
	}
}

func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}

// TestCartSnapshotItemsFeedsBasketDiff guards the reason this command exists:
// basket diff had no item source, because it snapshotted /cart/summary.
func TestCartSnapshotItemsFeedsBasketDiff(t *testing.T) {
	view := cartItemsView{Items: []cartItem{
		{ID: 36756, Name: "ARROZ INTEGRAL 1KG", Quantity: 3, UnitPrice: 6.99, LineTotal: 20.97},
		{ID: 0, Name: "no id — must be dropped", Quantity: 1, UnitPrice: 1},
	}}
	got := cartSnapshotItems(view)
	if len(got) != 1 {
		t.Fatalf("snapshot rows = %d, want 1 (the id-less row must drop)", len(got))
	}
	if got[0].ID != "36756" || got[0].Qty != 3 || got[0].UnitPrice != 6.99 {
		t.Errorf("snapshot row = %+v, want id 36756, qty 3, unit 6.99", got[0])
	}
	// diffCartSnapshots compares Price for price_changed, so it must hold the
	// UNIT price. Storing the 20.97 line total here made every quantity change
	// also report a price move — dropping one of two units read as a 50% cut.
	if got[0].Price != 6.99 {
		t.Errorf("snapshot Price = %v, want the 6.99 unit price, not the line total", got[0].Price)
	}
}

// TestBasketPriceSnapshotsTrackUnitPrice guards price-watch: tracking the line
// total would report a price move every time the quantity changed.
func TestBasketPriceSnapshotsTrackUnitPrice(t *testing.T) {
	view := cartItemsView{Items: []cartItem{
		{ID: 36756, Name: "ARROZ INTEGRAL 1KG", Quantity: 3, UnitPrice: 6.99, LineTotal: 20.97},
		{ID: 55, Name: "free sample", Quantity: 1, UnitPrice: 0},
	}}
	got := basketPriceSnapshots(view)
	if len(got) != 1 {
		t.Fatalf("snapshots = %d, want 1 (a zero-priced line carries no signal)", len(got))
	}
	if got[0].PriceCents != 699 {
		t.Errorf("price_cents = %d, want 699 (unit price, not the 2097 line total)", got[0].PriceCents)
	}
}

func TestFormatQuantityKeepsWeighedItemsHonest(t *testing.T) {
	cases := map[float64]string{3: "3", 0: "0", 1: "1", 0.5: "0.500", 1.25: "1.250"}
	for in, want := range cases {
		if got := formatQuantity(in); got != want {
			t.Errorf("formatQuantity(%v) = %q, want %q", in, got, want)
		}
	}
}

// TestCartListItemsHelpWires smoke-tests command wiring.
func TestCartListItemsHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"cart", "list-items", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cart list-items --help error = %v", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "list-items", "/cart/list"} {
		if !strings.Contains(help, want) {
			t.Fatalf("cart list-items --help missing %q in:\n%s", want, help)
		}
	}
}
