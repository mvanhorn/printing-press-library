// PATCH (instacart-cart-search-stock-price): shared parser for the price,
// size and availability fields the web app's persisted Items query returns.

package gql

import (
	"encoding/json"
	"errors"
)

// Availability values reported for cart lines and search results.
const (
	AvailabilityAvailable   = "available"
	AvailabilityLowStock    = "low_stock"
	AvailabilityUnavailable = "unavailable"
	AvailabilityUnknown     = "unknown"
)

// ErrAvailabilityUnknown is returned alongside cart lines when the Items
// lookup failed but every line still had a cached name. Callers can show the
// cart and warn that availability could not be checked.
var ErrAvailabilityUnknown = errors.New("could not check item availability")

// ItemDetails is the subset of an Items entry the CLI reports.
type ItemDetails struct {
	ItemID       string
	Name         string
	Size         string
	Available    *bool
	StockLevel   string
	StockLabel   string
	Price        string
	PricePerUnit string
}

// Availability classifies an item. availability.available is authoritative:
// unavailable items frequently still carry stockLevel "inStock".
func (d ItemDetails) Availability() string {
	switch {
	case d.Available == nil:
		return AvailabilityUnknown
	case !*d.Available:
		return AvailabilityUnavailable
	case d.StockLevel == "lowStock":
		return AvailabilityLowStock
	default:
		return AvailabilityAvailable
	}
}

type itemsEnvelope struct {
	Data struct {
		Items []*struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			Size         string `json:"size"`
			Availability *struct {
				Available   *bool  `json:"available"`
				StockLevel  string `json:"stockLevel"`
				ViewSection struct {
					StockLevelLabelString string `json:"stockLevelLabelString"`
				} `json:"viewSection"`
			} `json:"availability"`
			Price *struct {
				ViewSection struct {
					PriceString string `json:"priceString"`
					ItemDetails struct {
						PricePerUnitString string `json:"pricePerUnitString"`
					} `json:"itemDetails"`
					ItemCard struct {
						PricePerUnitString string `json:"pricePerUnitString"`
					} `json:"itemCard"`
				} `json:"viewSection"`
			} `json:"price"`
		} `json:"items"`
	} `json:"data"`
}

// parseItemDetails decodes an Items response. Null entries and entries
// without an id are skipped. Partial responses (GraphQL errors on some
// fields, e.g. price) still yield whatever data came back.
func parseItemDetails(raw []byte) []ItemDetails {
	var env itemsEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil
	}
	out := make([]ItemDetails, 0, len(env.Data.Items))
	for _, it := range env.Data.Items {
		if it == nil || it.ID == "" {
			continue
		}
		d := ItemDetails{ItemID: it.ID, Name: it.Name, Size: it.Size}
		if a := it.Availability; a != nil {
			d.Available = a.Available
			d.StockLevel = a.StockLevel
			d.StockLabel = a.ViewSection.StockLevelLabelString
		}
		if p := it.Price; p != nil {
			d.Price = p.ViewSection.PriceString
			d.PricePerUnit = p.ViewSection.ItemDetails.PricePerUnitString
			if d.PricePerUnit == "" {
				d.PricePerUnit = p.ViewSection.ItemCard.PricePerUnitString
			}
		}
		out = append(out, d)
	}
	return out
}
