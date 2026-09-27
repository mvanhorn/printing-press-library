package jalan

import "encoding/json"

// Evidence is a short, source-linked excerpt supporting one extracted fact.
type Evidence struct {
	Field string `json:"field"`
	Text  string `json:"text"`
	URL   string `json:"url"`
}

// BathFacts keeps room facilities independent from reservable property baths.
// A nil fact means the source did not establish either presence or absence.
type BathFacts struct {
	Indoor            *bool      `json:"indoor"`
	InRoom            *bool      `json:"in_room"`
	Outdoor           *bool      `json:"outdoor"`
	Private           *bool      `json:"private"`
	PrivateReservable *bool      `json:"private_reservable"`
	HotSpring         *bool      `json:"hot_spring"`
	Evidence          []Evidence `json:"evidence"`
}

// Price is the observed base quote, before conditional coupons or points.
type Price struct {
	Amount                 *int64     `json:"amount"`
	Currency               string     `json:"currency"`
	Basis                  string     `json:"basis"`
	TaxInclusion           string     `json:"tax_inclusion"`
	ServiceChargeInclusion string     `json:"service_charge_inclusion"`
	ExtraFees              string     `json:"extra_fees"`
	ConditionalDiscount    string     `json:"conditional_discount"`
	Points                 string     `json:"points"`
	ReferenceQuote         string     `json:"reference_quote"`
	QuoteText              string     `json:"quote_text"`
	OccupancyText          string     `json:"occupancy_text"`
	FinalPayable           *int64     `json:"final_payable"`
	Evidence               []Evidence `json:"evidence"`
}

func (p Price) MarshalJSON() ([]byte, error) {
	type alias Price
	return json.Marshal(struct {
		alias
		ExtraFees           *string `json:"extra_fees"`
		ConditionalDiscount *string `json:"conditional_discount"`
		Points              *string `json:"points"`
		ReferenceQuote      *string `json:"reference_quote"`
		QuoteText           *string `json:"quote_text"`
		OccupancyText       *string `json:"occupancy_text"`
	}{alias: alias(p), ExtraFees: knownString(p.ExtraFees), ConditionalDiscount: knownString(p.ConditionalDiscount), Points: knownString(p.Points), ReferenceQuote: knownString(p.ReferenceQuote), QuoteText: knownString(p.QuoteText), OccupancyText: knownString(p.OccupancyText)})
}

type Property struct {
	ID               string         `json:"id"`
	NameJa           string         `json:"name_ja"`
	URL              string         `json:"url"`
	LodgingType      string         `json:"lodging_type"`
	Address          string         `json:"address"`
	Access           string         `json:"access"`
	Description      string         `json:"description"`
	Amenities        []string       `json:"amenities"`
	Baths            BathFacts      `json:"baths"`
	RoomBaths        BathFacts      `json:"room_baths"`
	ReviewCategories map[string]any `json:"review_categories"`
	Price            Price          `json:"price"`
	Evidence         []Evidence     `json:"evidence"`
}

// MarshalJSON makes unavailable property scalars explicit nulls.
func (p Property) MarshalJSON() ([]byte, error) {
	type alias Property
	return json.Marshal(struct {
		alias
		NameJa      *string `json:"name_ja"`
		LodgingType *string `json:"lodging_type"`
		Address     *string `json:"address"`
		Access      *string `json:"access"`
		Description *string `json:"description"`
	}{alias: alias(p), NameJa: knownString(p.NameJa), LodgingType: knownString(p.LodgingType), Address: knownString(p.Address), Access: knownString(p.Access), Description: knownString(p.Description)})
}

func knownString(s string) *string {
	if s == "" || s == "unknown" {
		return nil
	}
	return &s
}

type Offer struct {
	PropertyID   string     `json:"property_id"`
	PlanID       string     `json:"plan_id"`
	RoomID       string     `json:"room_id"`
	PlanName     string     `json:"plan_name"`
	RoomName     string     `json:"room_name"`
	URL          string     `json:"url"`
	Meals        string     `json:"meals"`
	Smoking      string     `json:"smoking"`
	Baths        BathFacts  `json:"baths"`
	Price        Price      `json:"price"`
	Availability string     `json:"availability"`
	Evidence     []Evidence `json:"evidence"`
}

type Plan struct {
	Offer
	CheckIn         string   `json:"check_in"`
	CheckOut        string   `json:"check_out"`
	Cancellation    string   `json:"cancellation"`
	Fees            string   `json:"fees"`
	Payment         string   `json:"payment"`
	BookingDeadline string   `json:"booking_deadline"`
	RoomDescription string   `json:"room_description"`
	Description     string   `json:"description"`
	OccupancyText   string   `json:"occupancy_text"`
	PriceBreakdown  string   `json:"price_breakdown"`
	Restrictions    []string `json:"restrictions"`
}

type SearchPage struct {
	Items     []Property `json:"items"`
	Total     *int       `json:"total"`
	HasNext   bool       `json:"has_next"`
	NoResults bool       `json:"no_results"`
	Warnings  []string   `json:"warnings"`
}

type OfferPage struct {
	Items     []Offer  `json:"items"`
	Total     *int     `json:"total"`
	PlanTotal *int     `json:"plan_total"`
	HasNext   bool     `json:"has_next"`
	NoResults bool     `json:"no_results"`
	Warnings  []string `json:"warnings"`
}

func emptyBaths() BathFacts { return BathFacts{Evidence: []Evidence{}} }
func emptyPrice() Price {
	return Price{Currency: "JPY", Basis: "unknown", TaxInclusion: "unknown", ServiceChargeInclusion: "unknown", Evidence: []Evidence{}}
}
func emptyProperty(id, sourceURL string) Property {
	return Property{ID: id, URL: sourceURL, Amenities: []string{}, Baths: emptyBaths(), RoomBaths: emptyBaths(), ReviewCategories: map[string]any{}, Price: emptyPrice(), Evidence: []Evidence{}}
}
func emptyOffer(propertyID, sourceURL string) Offer {
	return Offer{PropertyID: propertyID, URL: sourceURL, PlanName: "unknown", RoomName: "unknown", Meals: "unknown", Smoking: "unknown", Baths: emptyBaths(), Price: emptyPrice(), Availability: "unknown", Evidence: []Evidence{}}
}
