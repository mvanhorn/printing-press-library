// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/dominos/internal/cartstore"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/dominos/internal/client"

	"github.com/spf13/cobra"
)

// menuCoupon is the per-coupon shape inside menu.Coupons (a map keyed by code).
type menuCoupon struct {
	Code        string `json:"Code"`
	Name        string `json:"Name"`
	Description string `json:"Description"`
	Price       string `json:"Price"`
	Local       bool   `json:"Local"`
}

// couponsForOrderResponse is what /auto-couponing-service/operation/all-coupons-for-order
// returns when given {Order: <full Domino's Order>}. Fulfilled = applies to
// this exact cart; unfulfilled = doesn't fit. The service returns coupon
// codes as strings (not objects), so we use json.RawMessage and decode
// per-element flexibly — sometimes string codes, sometimes objects with
// metadata. fetchCouponsForCart handles both shapes.
type couponsForOrderResponse struct {
	FulfilledCoupons   []json.RawMessage `json:"fulfilledCoupons"`
	UnfulfilledCoupons []json.RawMessage `json:"unfulfilledCoupons"`
}

// rawCouponToCode pulls a coupon code from one of the auto-couponing-service
// response items. Accepts either a bare JSON string ("1126") or an object
// with a Code/code field ({"Code":"1126","reason":"..."}).
func rawCouponToCode(raw json.RawMessage) (code, reason string) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, ""
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", ""
	}
	if c, ok := obj["Code"].(string); ok {
		code = c
	} else if c, ok := obj["code"].(string); ok {
		code = c
	}
	if r, ok := obj["reason"].(string); ok {
		reason = r
	} else if items, ok := obj["statusItems"].([]any); ok && len(items) > 0 {
		if first, ok := items[0].(map[string]any); ok {
			if c, ok := first["code"].(string); ok {
				reason = c
			}
		}
	}
	return code, reason
}

// dealsBestResult is the envelope returned to the caller.
type dealsBestResult struct {
	Best            []bestDeal `json:"best"`
	ConsideredCount int        `json:"considered_count"`
	Note            string     `json:"note,omitempty"`
}

type bestDeal struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Price       string `json:"price,omitempty"`
}

func newDealsBestCmd(flags *rootFlags) *cobra.Command {
	var cartName string

	cmd := &cobra.Command{
		Use:   "best",
		Short: "Find coupons that auto-apply to the active cart",
		Long: `Find coupons that auto-apply to the active cart.

Loads the active cart (or a named template), builds a Domino's Order from
it, and POSTs to the auto-couponing-service to get the list of coupons
that fulfill the order. Coupon names and descriptions are joined from the
store menu's Coupons map for display.

Note: 'best' is named for narrative parity with the absorb manifest; the
underlying auto-couponing-service returns ALL fulfilling coupons, not a
single best one. The order in 'best' reflects the service's natural order
(typically auto-applies first). Use 'deals eligible' for the verbose
fulfilled+unfulfilled+reason view.`,
		Example:     "  dominos-pp-cli deals best\n  dominos-pp-cli deals best --cart friday --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), dealsBestResult{Note: "dry-run"}, flags)
			}
			cart, err := loadCartOrTemplate(cartName)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			fulfilled, _, err := fetchCouponsForCart(c, cart)
			if err != nil {
				return apiErr(fmt.Errorf("auto-couponing failed: %w", err))
			}
			menu, _ := fetchMenuCoupons(c, cart.StoreID)
			result := dealsBestResult{ConsideredCount: len(fulfilled)}
			for _, raw := range fulfilled {
				code, _ := rawCouponToCode(raw)
				if code == "" {
					continue
				}
				bd := bestDeal{Code: code}
				if menuEntry, ok := menu[code]; ok {
					bd.Name = menuEntry.Name
					bd.Description = menuEntry.Description
					bd.Price = menuEntry.Price
				}
				result.Best = append(result.Best, bd)
			}
			if len(result.Best) == 0 {
				result.Note = "no coupons auto-applied to this cart shape; try 'deals eligible' to see what's missing"
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&cartName, "cart", "", "Use a named template instead of the active cart")
	return cmd
}

// fetchCouponsForCart wraps the cart in a Domino's-shaped Order and POSTs
// to /auto-couponing-service/operation/all-coupons-for-order. Returns
// (fulfilled, unfulfilled, error). Response items may be bare string codes
// or objects with metadata; pass each to rawCouponToCode for flexible parsing.
func fetchCouponsForCart(c *client.Client, cart *cartstore.Cart) ([]json.RawMessage, []json.RawMessage, error) {
	order := cartToOrder(cart)
	body := map[string]any{"Order": order}
	data, _, err := c.Post("/auto-couponing-service/operation/all-coupons-for-order", body)
	if err != nil {
		return nil, nil, err
	}
	var resp couponsForOrderResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, nil, fmt.Errorf("parse auto-couponing response: %w", err)
	}
	return resp.FulfilledCoupons, resp.UnfulfilledCoupons, nil
}

// fetchMenuCoupons fetches the store menu (no auth required) and returns the
// Coupons map keyed by code. Best-effort; errors here are non-fatal because
// callers can fall back to code-only display.
func fetchMenuCoupons(c *client.Client, storeID string) (map[string]menuCoupon, error) {
	if storeID == "" {
		return nil, fmt.Errorf("store id required to fetch menu")
	}
	data, err := c.Get("/power/store/"+storeID+"/menu", map[string]string{"lang": "en", "structured": "true"})
	if err != nil {
		return nil, err
	}
	var menu struct {
		Coupons map[string]menuCoupon `json:"Coupons"`
	}
	if err := json.Unmarshal(data, &menu); err != nil {
		return nil, err
	}
	return menu.Coupons, nil
}

// cartToOrder builds a minimal Domino's Order shape from the cart that the
// auto-couponing-service accepts. Only fields the service requires for
// fulfillment evaluation are populated; fields like Email/Phone/Payments
// are intentionally left empty (this isn't a real order placement).
func cartToOrder(cart *cartstore.Cart) map[string]any {
	products := make([]map[string]any, 0, len(cart.Items))
	for i, item := range cart.Items {
		opts := map[string]any{}
		// Default options for a base pizza if none specified
		if len(item.Options) == 0 {
			opts = map[string]any{
				"X": map[string]string{"1/1": "1"}, // sauce
				"C": map[string]string{"1/1": "1"}, // cheese
			}
		} else {
			for k, v := range item.Options {
				opts[k] = map[string]string{"1/1": v}
			}
		}
		qty := item.Qty
		if qty == 0 {
			qty = 1
		}
		products = append(products, map[string]any{
			"ID":      i + 1,
			"Code":    item.Code,
			"Qty":     qty,
			"Options": opts,
		})
	}
	return map[string]any{
		"StoreID":               cart.StoreID,
		"ServiceMethod":         cart.Service,
		"LanguageCode":          "en",
		"OrderChannel":          "OLO",
		"Market":                "UNITED_STATES",
		"Currency":              "USD",
		"OrderMethod":           "Web",
		"SourceOrganizationURI": "order.dominos.com",
		"Address":               map[string]any{"Street": cart.Address, "Type": "House"},
		"Products":              products,
		"Coupons":               []any{},
		"Extension":             map[string]any{},
		"OrderID":               "",
		"Email":                 "",
		"FirstName":             "",
		"LastName":              "",
		"Phone":                 "",
	}
}

// loadCartOrTemplate centralizes the active-cart-or-named-template choice
// shared by deals best, deals eligible, and order-quick. Used wherever a
// command takes a `--cart <name>` flag where empty means "active cart."
func loadCartOrTemplate(cartName string) (*cartstore.Cart, error) {
	if cartName == "" {
		cart, err := cartstore.LoadActive()
		if err != nil {
			if errors.Is(err, cartstore.ErrNotFound) {
				return nil, usageErr(fmt.Errorf("no active cart; run 'dominos-pp-cli cart new ...' or pass --cart <template>"))
			}
			return nil, err
		}
		return cart, nil
	}
	tpl, err := cartstore.LoadTemplate(cartName)
	if err != nil {
		if errors.Is(err, cartstore.ErrNotFound) {
			return nil, usageErr(fmt.Errorf("template %q not found; run 'dominos-pp-cli template list' to see saved templates", cartName))
		}
		return nil, err
	}
	// Template -> Cart shape conversion
	cart := &cartstore.Cart{
		StoreID: tpl.StoreID,
		Service: tpl.Service,
		Address: tpl.Address,
		Items:   tpl.Items,
	}
	return cart, nil
}
