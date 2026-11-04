// PATCH (instacart-cart-search-stock-price): one-line size/price/stock
// summary shared by `cart show` and `search` text output.

package cli

import (
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/gql"
)

// itemStatusLine renders e.g. "1.5 kg  $20.99 ($1.40/100g)  in stock (Many in stock)".
// Empty parts are dropped.
func itemStatusLine(size, price, pricePerUnit, availability, stockLabel string) string {
	var parts []string
	if size != "" {
		parts = append(parts, size)
	}
	if price != "" {
		if pricePerUnit != "" {
			price += " (" + pricePerUnit + ")"
		}
		parts = append(parts, price)
	}
	if s := availabilityText(availability, stockLabel); s != "" {
		parts = append(parts, s)
	}
	return strings.Join(parts, "  ")
}

func availabilityText(availability, stockLabel string) string {
	var s string
	switch availability {
	case gql.AvailabilityAvailable:
		s = "in stock"
	case gql.AvailabilityLowStock:
		s = "low stock"
	case gql.AvailabilityUnavailable:
		s = "UNAVAILABLE"
	case gql.AvailabilityUnknown:
		return "availability unknown"
	default:
		return ""
	}
	if stockLabel != "" && !strings.EqualFold(stockLabel, s) {
		s += " (" + stockLabel + ")"
	}
	return s
}
