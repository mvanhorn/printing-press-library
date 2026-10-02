// Package traveloka implements normalized public quotes and private, scoped HTTP replay.
package traveloka

import (
	"encoding/json"
	"fmt"
)

type Shopper struct {
	Market   string `json:"market"`
	Locale   string `json:"locale"`
	Currency string `json:"currency"`
}
type Query struct {
	Kind          string `json:"kind"`
	Market        string `json:"market"`
	Locale        string `json:"locale"`
	Currency      string `json:"currency"`
	Origin        string `json:"origin"`
	Destination   string `json:"destination"`
	Depart        string `json:"depart"`
	ReturnDate    string `json:"return_date"`
	Cabin         string `json:"cabin"`
	GeoID         string `json:"geo_id"`
	PropertyID    string `json:"property_id"`
	PropertyName  string `json:"property_name"`
	CheckIn       string `json:"check_in"`
	CheckOut      string `json:"check_out"`
	Adults        int    `json:"adults"`
	Children      int    `json:"children"`
	Infants       int    `json:"infants"`
	Rooms         int    `json:"rooms"`
	Limit         int    `json:"limit"`
	Offset        int    `json:"offset"`
	MaxCandidates int    `json:"max_candidates"`
	ChildAges     []int  `json:"child_ages"`
}

// ContextKey excludes only retrieval/output caps; all shopper and travel fields remain.
func (q Query) ContextKey() string {
	q.Limit, q.Offset, q.MaxCandidates = 0, 0, 0
	if q.ChildAges == nil {
		q.ChildAges = []int{}
	}
	b, _ := json.Marshal(q)
	return string(b)
}
func (q Query) SameContext(other Query) bool { return q.ContextKey() == other.ContextKey() }
func (q Query) Shopper() Shopper             { return Shopper{q.Market, q.Locale, q.Currency} }

type Money struct {
	Currency   string `json:"currency"`
	Amount     string `json:"amount"`
	MinorUnits string `json:"minor_units"`
	Decimals   *int   `json:"decimals"`
}
type Price struct {
	Total           *Money `json:"total"`
	PerPassenger    *Money `json:"per_passenger"`
	PerRoomPerNight *Money `json:"per_room_per_night"`
	BaseFare        *Money `json:"base_fare"`
	Taxes           *Money `json:"taxes"`
	Fees            *Money `json:"fees"`
	TaxInclusion    string `json:"tax_inclusion"`
}
type Segment struct {
	Origin                    string         `json:"origin"`
	Destination               string         `json:"destination"`
	DepartureDate             string         `json:"departure_date"`
	DepartureTime             string         `json:"departure_time"`
	ArrivalDate               string         `json:"arrival_date"`
	ArrivalTime               string         `json:"arrival_time"`
	DepartureUTCOffsetMinutes *int           `json:"departure_utc_offset_minutes"`
	ArrivalUTCOffsetMinutes   *int           `json:"arrival_utc_offset_minutes"`
	MarketingAirline          string         `json:"marketing_airline"`
	OperatingAirline          string         `json:"operating_airline"`
	FlightNumber              string         `json:"flight_number"`
	Cabin                     string         `json:"cabin"`
	DurationMinutes           *int           `json:"duration_minutes"`
	CheckedBaggage            any            `json:"checked_baggage"`
	CabinBaggage              any            `json:"cabin_baggage"`
	Details                   map[string]any `json:"details"`
}
type Leg struct {
	Segments  []Segment      `json:"segments"`
	FareRules map[string]any `json:"fare_rules"`
	Details   map[string]any `json:"details"`
}
type Offer struct {
	ID              string         `json:"id"`
	Kind            string         `json:"kind"`
	PropertyID      string         `json:"property_id"`
	PropertyName    string         `json:"property_name"`
	RoomID          string         `json:"room_id"`
	RoomName        string         `json:"room_name"`
	MealPlan        string         `json:"meal_plan"`
	Payment         string         `json:"payment"`
	BookingURL      string         `json:"booking_url"`
	Price           Price          `json:"price"`
	Legs            []Leg          `json:"legs"`
	Stops           *int           `json:"stops"`
	DurationMinutes *int           `json:"duration_minutes"`
	OccupancyMatch  *bool          `json:"occupancy_match"`
	Refundable      *bool          `json:"refundable"`
	Reschedulable   *bool          `json:"reschedulable"`
	Cancellation    map[string]any `json:"cancellation"`
	Details         map[string]any `json:"details"`
}
type Snapshot struct {
	ID             string         `json:"id"`
	Kind           string         `json:"kind"`
	Status         string         `json:"status"`
	RetrievedAt    string         `json:"retrieved_at"`
	Query          Query          `json:"query"`
	Offers         []Offer        `json:"offers"`
	SearchComplete bool           `json:"search_complete"`
	Coverage       map[string]any `json:"coverage"`
	Warnings       []string       `json:"warnings"`
	Indicative     bool           `json:"indicative"`
	Freshness      string         `json:"freshness"`
}

type APIError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Status    int    `json:"status"`
	Retryable bool   `json:"retryable"`
	Cause     error  `json:"-"`
}

func (e *APIError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }
func (e *APIError) Unwrap() error { return e.Cause }
func apiError(code, message string, status int, retryable bool) *APIError {
	return &APIError{Code: code, Message: message, Status: status, Retryable: retryable}
}
