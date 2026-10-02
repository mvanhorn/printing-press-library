package traveloka

import (
	"errors"
	"testing"
)

func TestSimulatedHotelEmptyPriceFallbackAndTaxInclusion(t *testing.T) {
	empty := map[string]any{"currency": "SGD", "amount": "0", "nullOrEmpty": true}
	for _, tt := range []struct {
		name                    string
		display                 map[string]any
		amount, inclusion, code string
	}{
		{"empty inclusive uses neutral total", map[string]any{"inclusiveFinalPrice": empty, "totalFare": coreMoney("10000"), "exclusiveFinalPrice": coreMoney("9000")}, "100.00", "unknown", ""},
		{"empty inclusive and neutral use exclusive", map[string]any{"inclusiveFinalPrice": empty, "totalFare": empty, "exclusiveFinalPrice": coreMoney("9000")}, "90.00", "exclusive", ""},
		{"missing amount uses neutral total", map[string]any{"inclusiveFinalPrice": map[string]any{"currency": "SGD"}, "totalFare": coreMoney("10000")}, "100.00", "unknown", ""},
		{"known zero retains inclusive basis", map[string]any{"inclusiveFinalPrice": coreMoney("0"), "totalFare": coreMoney("10000")}, "0.00", "inclusive", ""},
		{"all empty stays unknown", map[string]any{"inclusiveFinalPrice": empty, "totalFare": empty, "exclusiveFinalPrice": empty}, "", "unknown", ""},
		{"fallback currency mismatch is rejected", map[string]any{"inclusiveFinalPrice": empty, "totalFare": map[string]any{"currency": "USD", "amount": "10000"}}, "", "", "CURRENCY_MISMATCH"},
		{"malformed inclusive cannot be hidden by fallback", map[string]any{"inclusiveFinalPrice": coreMoney("1.5"), "totalFare": coreMoney("10000")}, "", "", "MALFORMED_RESPONSE"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt.display["numOfDecimalPoint"] = "2"
			price, err := hotelFinalPrice(map[string]any{"totalPriceRateDisplay": tt.display, "perRoomPerNightDisplay": tt.display}, "SGD")
			if tt.code != "" {
				var source *APIError
				if !errors.As(err, &source) || source.Code != tt.code {
					t.Fatalf("expected %s, got %v", tt.code, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			total, nightly := "", ""
			if price.Total != nil {
				total = price.Total.Amount
			}
			if price.PerRoomPerNight != nil {
				nightly = price.PerRoomPerNight.Amount
			}
			if total != tt.amount || nightly != tt.amount || price.TaxInclusion != tt.inclusion {
				t.Fatalf("source fallback/inclusion lost: total=%s nightly=%s inclusion=%s", total, nightly, price.TaxInclusion)
			}
		})
	}
}
