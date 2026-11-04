package gql

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/auth"
	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/config"
	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/store"
)

// CartLineItem is one row of a shopper's cart with a resolved product name.
type CartLineItem struct {
	Name         string  `json:"name"`
	ItemID       string  `json:"item_id"`
	ProductID    string  `json:"product_id"`
	Quantity     float64 `json:"quantity"`
	QuantityType string  `json:"quantity_type,omitempty"`
	Retailer     string  `json:"retailer,omitempty"`
	// PATCH (instacart-cart-search-stock-price): live availability, price
	// and size from Items. Availability is always set; "unknown" means the
	// Items lookup did not return this line (e.g. not offered at this store).
	Available    *bool  `json:"available,omitempty"`
	Availability string `json:"availability"`
	StockLevel   string `json:"stock_level,omitempty"`
	StockLabel   string `json:"stock_label,omitempty"`
	Price        string `json:"price,omitempty"`
	PricePerUnit string `json:"price_per_unit,omitempty"`
	Size         string `json:"size,omitempty"`
}

// FetchCartItems chains three graphql operations to resolve the full contents
// of a cart with real product names and live availability:
//
//  1. CartData(cartId) -> list of {basketProduct.id (items_LOC-PROD), quantity}
//  2. ensureInventoryToken(retailerSlug) -> shopId + zoneId context for Items
//  3. Items(ids, shopId, zoneId, postalCode) -> names, size, price, availability
//
// Items is always called because availability and price change; cached
// product names from earlier searches only fill in names Items doesn't
// return. When Items fails but every line has a cached name, the lines are
// returned with availability "unknown" and ErrAvailabilityUnknown.
func FetchCartItems(ctx context.Context, sess *auth.Session, cfg *config.Config, st *store.Store, cartID, retailerSlug string) ([]CartLineItem, error) {
	if cartID == "" {
		return nil, fmt.Errorf("cart id is required")
	}

	client := NewClient(sess, cfg, st)

	// Step 1: fetch cart data.
	resp, err := client.Query(ctx, "CartData", map[string]any{"id": cartID})
	if err != nil {
		return nil, fmt.Errorf("CartData: %w", err)
	}

	rawLines, err := parseCartData(resp.RawBody)
	if err != nil {
		return nil, err
	}
	if len(rawLines) == 0 {
		return []CartLineItem{}, nil
	}

	lines := make([]CartLineItem, 0, len(rawLines))
	ids := make([]string, 0, len(rawLines))
	allNamed := true
	for _, raw := range rawLines {
		line := CartLineItem{
			ItemID:       raw.itemID,
			Quantity:     raw.quantity,
			QuantityType: raw.quantityType,
			Retailer:     retailerSlug,
			Availability: AvailabilityUnknown,
		}
		if idx := strings.LastIndex(raw.itemID, "-"); idx > 0 {
			line.ProductID = raw.itemID[idx+1:]
		}
		if cached, _ := st.GetProduct(raw.itemID); cached != nil && cached.Name != "" {
			line.Name = cached.Name
		} else {
			allNamed = false
		}
		lines = append(lines, line)
		ids = append(ids, raw.itemID)
	}

	// Step 2: bootstrap shopId/zoneId the same way search does.
	tok, err := ensureInventoryToken(ctx, client, st, cfg, retailerSlug)
	if err != nil {
		if allNamed {
			return lines, fmt.Errorf("%w: bootstrap retailer context for %s: %v", ErrAvailabilityUnknown, retailerSlug, err)
		}
		return lines, fmt.Errorf("bootstrap retailer context for %s: %w", retailerSlug, err)
	}

	// Step 3: Items for every line.
	itemsVars := map[string]any{
		"ids":        ids,
		"shopId":     tok.ShopID,
		"zoneId":     tok.ZoneID,
		"postalCode": cfg.PostalCode,
	}
	resp, err = client.Query(ctx, "Items", itemsVars)
	// Instacart returns partial data with a "Cross Shop Load", "Missing
	// arguments for item price" or "Internal Error" (on price) error for some
	// items. The other fields still come through in that partial response.
	// Tolerate the error when RawBody still has items.
	var details []ItemDetails
	if resp != nil && len(resp.RawBody) > 0 {
		details = parseItemDetails(resp.RawBody)
	}
	if err != nil && len(details) == 0 {
		if allNamed {
			return lines, fmt.Errorf("%w: Items lookup: %v", ErrAvailabilityUnknown, err)
		}
		return lines, fmt.Errorf("Items lookup for cart: %w", err)
	}
	applyItemDetails(lines, details)
	for _, d := range details {
		if d.Name == "" {
			continue
		}
		prodID := d.ItemID
		if idx := strings.LastIndex(d.ItemID, "-"); idx > 0 {
			prodID = d.ItemID[idx+1:]
		}
		_ = st.UpsertProduct(store.Product{
			ItemID:       d.ItemID,
			ProductID:    prodID,
			RetailerSlug: retailerSlug,
			Name:         d.Name,
		})
	}
	return lines, nil
}

// applyItemDetails merges Items data into cart lines by item id. Lines that
// Items did not return keep availability "unknown".
func applyItemDetails(lines []CartLineItem, details []ItemDetails) {
	byID := make(map[string]ItemDetails, len(details))
	for _, d := range details {
		byID[d.ItemID] = d
	}
	for i := range lines {
		d, ok := byID[lines[i].ItemID]
		if !ok {
			continue
		}
		if d.Name != "" {
			lines[i].Name = d.Name
		}
		lines[i].Available = d.Available
		lines[i].Availability = d.Availability()
		lines[i].StockLevel = d.StockLevel
		lines[i].StockLabel = d.StockLabel
		lines[i].Price = d.Price
		lines[i].PricePerUnit = d.PricePerUnit
		lines[i].Size = d.Size
	}
}

type rawCartLine struct {
	itemID       string
	quantity     float64
	quantityType string
}

// parseCartData walks a CartData response and extracts cart items.
// The response shape is {data: {userCart: {cartItemCollection: {cartItems: [{quantity, basketProduct: {id}}]}}}}.
func parseCartData(raw []byte) ([]rawCartLine, error) {
	var envelope struct {
		Data struct {
			UserCart struct {
				CartItemCollection struct {
					CartItems []struct {
						Quantity      float64 `json:"quantity"`
						QuantityType  string  `json:"quantityType"`
						BasketProduct struct {
							ID string `json:"id"`
						} `json:"basketProduct"`
					} `json:"cartItems"`
				} `json:"cartItemCollection"`
			} `json:"userCart"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("parse CartData: %w", err)
	}
	var out []rawCartLine
	for _, it := range envelope.Data.UserCart.CartItemCollection.CartItems {
		if it.BasketProduct.ID == "" {
			continue
		}
		out = append(out, rawCartLine{
			itemID:       it.BasketProduct.ID,
			quantity:     it.Quantity,
			quantityType: it.QuantityType,
		})
	}
	return out, nil
}
