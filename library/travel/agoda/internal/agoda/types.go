// Copyright 2026 Victor Wibisono and contributors. Licensed under Apache-2.0. See LICENSE.

package agoda

// Property is the normalized shape every search-shaped command returns.
//
// The two price fields are the reason this CLI exists. Agoda's API returns both
// in the same response but its website renders only PriceAdvertised, so a stay
// routinely costs 20-30% more than the number the user was shown.
type Property struct {
	PropertyID int    `json:"property_id"`
	Name       string `json:"name"`
	Currency   string `json:"currency"`

	// PriceAdvertised is Agoda's "exclusive" price: the headline rate, before
	// taxes and fees. This is what the website shows.
	PriceAdvertised float64 `json:"price_advertised"`

	// PriceAllIn is Agoda's "inclusive" price: the real total for the stay.
	PriceAllIn float64 `json:"price_all_in"`

	// HiddenAmount and HiddenPct express the gap between the two.
	HiddenAmount float64 `json:"hidden_amount"`
	HiddenPct    float64 `json:"hidden_pct"`

	// PerNightAllIn is PriceAllIn divided across the stay, for comparing stays
	// of differing length.
	PerNightAllIn float64 `json:"per_night_all_in"`

	CrossedOutPrice float64 `json:"crossed_out_price,omitempty"`

	StarRating  float64 `json:"star_rating,omitempty"`
	ReviewScore float64 `json:"review_score,omitempty"`
	ReviewCount int     `json:"review_count,omitempty"`

	// FreeCancellation and FreeCancellationUntil come from
	// pricing.payment.cancellation. Agoda marks the large majority of stays
	// free-cancellation, so the deadline is the field carrying real
	// information: it is the date after which the booking stops being
	// refundable.
	//
	// There is deliberately no breakfast flag. The search response carries no
	// per-property breakfast signal, and a field that is always false would
	// read as "no property includes breakfast", which would be a lie.
	FreeCancellation      bool   `json:"free_cancellation"`
	FreeCancellationUntil string `json:"free_cancellation_until,omitempty"`
	SoldOut               bool   `json:"sold_out"`

	Address    string  `json:"address,omitempty"`
	Latitude   float64 `json:"latitude,omitempty"`
	Longitude  float64 `json:"longitude,omitempty"`
	BookingURL string  `json:"booking_url,omitempty"`
}

// Destination is one resolved autocomplete suggestion.
type Destination struct {
	CityID     int     `json:"city_id"`
	Name       string  `json:"name"`
	ResultText string  `json:"result_text"`
	CountryID  int     `json:"country_id"`
	HotelCount int     `json:"hotel_count"`
	IsHotel    bool    `json:"is_hotel"`
	Latitude   float64 `json:"latitude,omitempty"`
	Longitude  float64 `json:"longitude,omitempty"`
}

// TrendPoint is one (date, price) observation from Agoda's native price-trend
// operation, which returns a whole window in a single call.
type TrendPoint struct {
	PropertyID int     `json:"property_id"`
	CheckIn    string  `json:"checkin"`
	Price      float64 `json:"price"`
	TrendType  string  `json:"trend_type,omitempty"`
}

// SearchOptions carries everything the caller can vary about a hotel search.
type SearchOptions struct {
	CityID    int
	CheckIn   string // YYYY-MM-DD
	Nights    int
	Rooms     int
	Adults    int
	Children  int
	ChildAges []int
	Currency  string
	Locale    string
	Origin    string

	// SortField/SortOrder are passed through to Agoda's own ranking. Commands
	// that re-rank locally leave these at the Agoda default and sort afterwards.
	SortField string
	SortOrder string

	// Authenticated requests member pricing. It only has an effect when the
	// client also carries a Cookie.
	Authenticated bool
	MemberID      int
}
