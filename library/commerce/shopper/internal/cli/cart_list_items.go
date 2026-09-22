// Copyright 2026 educrvz and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel command: the line items actually in the cart.
//
// PATCH: cart-list-items. GET /cart/summary — the only cart read the sniffed
// spec captured — returns TOTALS ONLY (totalCart, totalAmount, minValue, …) and
// carries no item array at all. The web basket page reads GET /cart/list, which
// answers {departments:[{id,name,quantity,products:[...]}], paused:{products:[...]},
// total_items, total_value, recurrent_cart_id}. Without it the CLI could report
// what the basket costs but never what was in it, and `basket diff` — which
// compares basket contents across cycles — had no item source to read from.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/shopper/internal/client"
	"github.com/spf13/cobra"
)

// cartListProduct is one product row inside a department of GET /cart/list.
// Field names match the live payload; the cart reuses the catalog product
// object, so the in-basket count is `cartQuantity`, not `quantity`.
type cartListProduct struct {
	ID              int64   `json:"id"`
	SKU             string  `json:"sku"` // string here, though the catalog returns it as a number
	Name            string  `json:"name"`
	Price           string  `json:"price"`
	PriceRaw        float64 `json:"priceRaw"`
	CartQuantity    float64 `json:"cartQuantity"`
	MaxCartQuantity float64 `json:"maxCartQuantity"`
	OutOfStock      bool    `json:"outOfStock"`
	Paused          bool    `json:"paused"`
}

// cartListDepartment is one department bucket of GET /cart/list.
type cartListDepartment struct {
	ID       int64             `json:"id"`
	Name     string            `json:"name"`
	Quantity float64           `json:"quantity"`
	Products []cartListProduct `json:"products"`
}

// cartListResponse is the GET /cart/list envelope.
type cartListResponse struct {
	TotalItems      float64              `json:"total_items"`
	TotalValue      string               `json:"total_value"`
	FinalValue      float64              `json:"final_value"`
	Discount        string               `json:"discount"`
	HasDiscount     bool                 `json:"has_discount"`
	RecurrentCartID int64                `json:"recurrent_cart_id"` // null on one-off stores
	Departments     []cartListDepartment `json:"departments"`
	Paused          *cartListDepartment  `json:"paused"`
}

// cartItem is one flattened basket line as this command reports it.
type cartItem struct {
	ID         int64   `json:"id"`
	SKU        string  `json:"sku,omitempty"`
	Name       string  `json:"name"`
	Department string  `json:"department"`
	Quantity   float64 `json:"quantity"`
	UnitPrice  float64 `json:"unit_price"`
	LineTotal  float64 `json:"line_total"`
	OutOfStock bool    `json:"out_of_stock,omitempty"`
	Paused     bool    `json:"paused,omitempty"`
}

type cartItemsView struct {
	Store           string     `json:"store"`
	TotalItems      float64    `json:"total_items"`
	TotalValue      string     `json:"total_value"`
	ItemsValue      float64    `json:"items_value"`
	RecurrentCartID int64      `json:"recurrent_cart_id,omitempty"`
	Items           []cartItem `json:"items"`
	PausedItems     []cartItem `json:"paused_items,omitempty"`
	Note            string     `json:"note,omitempty"`
}

// fetchCartItems reads GET /cart/list and flattens it into basket lines.
// Paused products are returned separately: they belong to the recurring basket
// but are skipped for the upcoming cycle, so folding them into the active list
// would overstate both the count and the value.
func fetchCartItems(cmd *cobra.Command, c *client.Client, flags *rootFlags) (cartItemsView, error) {
	view := cartItemsView{Store: storeLabel(flags.store), Items: []cartItem{}}

	data, err := c.Get(cmd.Context(), "/cart/list", nil)
	if err != nil {
		return view, classifyAPIError(err, flags)
	}
	var resp cartListResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return view, fmt.Errorf("decode /cart/list response: %w", err)
	}

	view.TotalItems = resp.TotalItems
	view.TotalValue = resp.TotalValue
	view.RecurrentCartID = resp.RecurrentCartID

	for _, dept := range resp.Departments {
		for _, p := range dept.Products {
			view.Items = append(view.Items, newCartItem(p, dept.Name, false))
			view.ItemsValue += p.PriceRaw * p.CartQuantity
		}
	}
	if resp.Paused != nil {
		for _, p := range resp.Paused.Products {
			view.PausedItems = append(view.PausedItems, newCartItem(p, resp.Paused.Name, true))
		}
	}

	// Stable, useful ordering: biggest line first, then name, so two runs of
	// the same basket always print in the same order.
	sort.SliceStable(view.Items, func(i, j int) bool {
		if view.Items[i].LineTotal != view.Items[j].LineTotal {
			return view.Items[i].LineTotal > view.Items[j].LineTotal
		}
		return view.Items[i].Name < view.Items[j].Name
	})

	if len(view.Items) == 0 && len(view.PausedItems) == 0 {
		view.Note = "Basket is empty for this store. Pass --store to check another storefront: " +
			"programada, fresh, pet, unica, now, now-bebidas."
	}
	return view, nil
}

func newCartItem(p cartListProduct, department string, paused bool) cartItem {
	return cartItem{
		ID:         p.ID,
		SKU:        p.SKU,
		Name:       normalizeSpace(p.Name),
		Department: department,
		Quantity:   p.CartQuantity,
		UnitPrice:  p.PriceRaw,
		LineTotal:  p.PriceRaw * p.CartQuantity,
		OutOfStock: p.OutOfStock,
		Paused:     paused || p.Paused,
	}
}

func newCartListItemsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list-items",
		Aliases: []string{"items", "ls"},
		Short:   "List the products actually in the cart (name, quantity, unit price, line total)",
		Long: `Reads GET /cart/list — the endpoint behind the web basket page — and prints
one row per product in the cart.

'cart list-summary' only returns totals; this is the command that shows what
those totals are made of. Paused products (in the recurring basket but skipped
for the upcoming cycle) are reported separately and excluded from the active
item value.

Store-scoped: pass --store to read another storefront's basket.`,
		Example: "  shopper-pp-cli cart list-items\n" +
			"  shopper-pp-cli cart list-items --store fresh --json",
		Annotations: map[string]string{
			"mcp:read-only":          "true",
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), `{"dry_run":true,"would":"GET /cart/list","store":%q}`+"\n", storeLabel(flags.store))
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			view, err := fetchCartItems(cmd, c, flags)
			if err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(view.Items) == 0 && len(view.PausedItems) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: basket is empty.\n", view.Store)
				return nil
			}
			rows := make([][]string, 0, len(view.Items)+len(view.PausedItems))
			for _, it := range append(append([]cartItem{}, view.Items...), view.PausedItems...) {
				name := it.Name
				if it.Paused {
					name += " (paused)"
				} else if it.OutOfStock {
					name += " (out of stock)"
				}
				rows = append(rows, []string{
					strconv.FormatInt(it.ID, 10),
					name,
					it.Department,
					formatQuantity(it.Quantity),
					"R$ " + formatBRL(it.UnitPrice),
					"R$ " + formatBRL(it.LineTotal),
				})
			}
			if err := flags.printTable(cmd, []string{"ID", "PRODUCT", "DEPARTMENT", "QTY", "UNIT", "LINE"}, rows); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%s: %s item(s), %s\n",
				view.Store, formatQuantity(view.TotalItems), view.TotalValue)
			return nil
		},
	}
	return cmd
}

// formatQuantity renders a cart quantity without a trailing ".0" for whole units
// while still showing weighed items (0.5 kg) honestly.
func formatQuantity(q float64) string {
	if q == float64(int64(q)) {
		return strconv.FormatInt(int64(q), 10)
	}
	return strconv.FormatFloat(q, 'f', 3, 64)
}

// normalizeSpace collapses the runs of padding spaces the catalog embeds in
// product names ("ARROZ      DE PILAO ... 1KG ") so table columns line up.
func normalizeSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
