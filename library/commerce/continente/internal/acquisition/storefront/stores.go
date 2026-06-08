package storefront

import (
	"context"
	"encoding/json"
	"fmt"
)

type NearbyStoresResponse struct {
	Action string        `json:"action,omitempty"`
	Stores []StoreRecord `json:"stores"`
}

type StoreRecord struct {
	ID            string  `json:"ID"`
	Name          string  `json:"name"`
	Address1      string  `json:"address1,omitempty"`
	Address2      string  `json:"address2,omitempty"`
	City          string  `json:"city,omitempty"`
	PostalCode    string  `json:"postalCode,omitempty"`
	Latitude      float64 `json:"latitude,omitempty"`
	Longitude     float64 `json:"longitude,omitempty"`
	Phone         string  `json:"phone,omitempty"`
	StateCode     string  `json:"stateCode,omitempty"`
	CountryCode   string  `json:"countryCode,omitempty"`
	StoreHours    string  `json:"storeHours,omitempty"`
	IsGalpStore   bool    `json:"isGalpStore,omitempty"`
	IsPickupStore bool    `json:"isPickupStore,omitempty"`
}

func FetchNearbyStores(ctx context.Context, c Getter, latitude, longitude float64) (json.RawMessage, error) {
	if c == nil {
		return nil, fmt.Errorf("storefront nearby stores fetch: nil client")
	}
	return c.Get(ctx, "/on/demandware.store/Sites-continente-Site/default/Stores-FindStores", map[string]string{
		"latitude":  fmt.Sprintf("%v", latitude),
		"longitude": fmt.Sprintf("%v", longitude),
	})
}
