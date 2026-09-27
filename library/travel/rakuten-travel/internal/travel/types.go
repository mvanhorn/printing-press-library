// Package travel reads bounded anonymous Rakuten Travel HTML documents.
package travel

import (
	"context"
	"net/http"
	"time"
)

const (
	StatusOK             = "ok"
	StatusNoMatches      = "no_matches"
	StatusNoAvailability = "no_availability"
)

// Config controls one invocation. Only an injected HTTPClient permits a test
// MinInterval below one second; normal public requests always honor that floor.
type Config struct {
	CacheDir        string
	NoCache         bool
	Refresh         bool
	InventoryTTL    time.Duration
	Timeout         time.Duration
	MaxRequests     int
	HTTPClient      *http.Client
	MinInterval     time.Duration
	CacheMaxEntries int
	CacheMaxBytes   int64
	Now             func() time.Time
	// OnCacheWriteError observes optional persistence failures after a successful
	// parse. It never changes result status; nil silently ignores those failures.
	OnCacheWriteError func(error)
}

type Children struct {
	Upper         int `json:"child_upper"`
	Lower         int `json:"child_lower"`
	InfantMealBed int `json:"infant_meal_bed"`
	InfantMeal    int `json:"infant_meal"`
	InfantBed     int `json:"infant_bed"`
	InfantNone    int `json:"infant_none"`
}

type HotelQuery struct {
	Query  string `json:"query"`
	Area   string `json:"area"`
	Page   int    `json:"page"`
	Offset int    `json:"offset"`
	Limit  int    `json:"limit"`
}

type OfferQuery struct {
	HotelID       string   `json:"hotel_id"`
	Checkin       string   `json:"checkin"`
	Checkout      string   `json:"checkout"`
	Rooms         int      `json:"rooms"`
	AdultsPerRoom int      `json:"adults_per_room"`
	Children      Children `json:"children_per_room"`
	Page          int      `json:"page"`
	Offset        int      `json:"offset"`
	Limit         int      `json:"limit"`
}

type SourceInfo struct {
	Name            string    `json:"name"`
	URL             string    `json:"url"`
	FetchedAt       time.Time `json:"fetched_at"`
	ObservedAt      time.Time `json:"observed_at"`
	CacheState      string    `json:"cache_state"`
	CacheAgeSeconds float64   `json:"cache_age_seconds"`
	CacheTTLSeconds int       `json:"cache_ttl_seconds"`
}

type PageInfo struct {
	SourcePage      int    `json:"source_page"`
	SourceUnit      string `json:"source_unit"`
	Offset          int    `json:"offset"`
	Limit           int    `json:"limit"`
	SourceItemsSeen int    `json:"source_items_seen"`
	RowsSeen        int    `json:"rows_seen"`
	RowsScanned     int    `json:"rows_scanned"`
	Emitted         int    `json:"emitted"`
	SourceTotal     *int   `json:"source_total"`
	HasMore         bool   `json:"has_more"`
	NextPage        *int   `json:"next_page"`
	NextOffset      *int   `json:"next_offset"`
	Coverage        string `json:"coverage"`
}

type RequestStats struct {
	Requests         int   `json:"requests"`
	Retries          int   `json:"retries"`
	Bytes            int64 `json:"bytes"`
	CacheHits        int   `json:"cache_hits"`
	NetworkLatencyMS int64 `json:"network_latency_ms"`
}

type Area struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Parent *string `json:"parent"`
	URL    string  `json:"url"`
}

type Rating struct {
	Source      string  `json:"source"`
	Score       float64 `json:"score"`
	ReviewCount *int    `json:"review_count"`
}

type Coordinates struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Datum     string  `json:"datum"`
	Unit      string  `json:"unit"`
}

type Hotel struct {
	ID                 string       `json:"hotel_id"`
	Name               *string      `json:"name"`
	URL                string       `json:"url"`
	Rating             *Rating      `json:"rating"`
	Address            *string      `json:"address"`
	PostalCode         *string      `json:"postal_code"`
	Phone              *string      `json:"phone"`
	Access             []string     `json:"access"`
	Parking            []string     `json:"parking"`
	HotelAmenities     []string     `json:"hotel_amenities"`
	RoomAmenities      []string     `json:"room_amenities"`
	Notes              []string     `json:"notes"`
	CancellationPolicy []string     `json:"property_cancellation_policy"`
	PolicyCaveat       *string      `json:"policy_caveat"`
	Coordinates        *Coordinates `json:"coordinates"`
	CoordinatesReason  string       `json:"coordinates_reason"`
}

type Price struct {
	Currency         string `json:"currency"`
	PerRoomStayJPY   int64  `json:"per_room_whole_stay_jpy"`
	PerPersonStayJPY *int64 `json:"per_person_whole_stay_jpy"`
	ConsumptionTax   string `json:"consumption_tax"`
	AccommodationTax string `json:"accommodation_tax"`
	OtherTaxes       string `json:"other_taxes"`
	OptionalFees     string `json:"optional_fees"`
	SourceLabel      string `json:"source_label"`
}

type Meals struct {
	Breakfast   *bool   `json:"breakfast"`
	Dinner      *bool   `json:"dinner"`
	SourceLabel *string `json:"source_label"`
}

type Policy struct {
	Level  string   `json:"level"`
	Text   []string `json:"text"`
	Caveat *string  `json:"caveat"`
}

type Offer struct {
	HotelID            string     `json:"hotel_id"`
	PlanID             string     `json:"plan_id"`
	RoomID             string     `json:"room_id"`
	PlanName           *string    `json:"plan_name"`
	RoomName           *string    `json:"room_name"`
	Description        *string    `json:"description"`
	RoomDescription    *string    `json:"room_description"`
	Price              Price      `json:"price"`
	Meals              Meals      `json:"meals"`
	CancellationPolicy *Policy    `json:"plan_cancellation_policy"`
	SourceCaveat       *string    `json:"source_caveat"`
	SourceURL          string     `json:"source_url"`
	BookingURL         string     `json:"booking_url"`
	RoomAnchor         string     `json:"room_anchor"`
	Query              OfferQuery `json:"query"`
}

type AreaResult struct {
	Status string     `json:"status"`
	Areas  []Area     `json:"areas"`
	Source SourceInfo `json:"source"`
	Page   PageInfo   `json:"page"`
}

type HotelSearchResult struct {
	Status string     `json:"status"`
	Hotels []Hotel    `json:"hotels"`
	Query  HotelQuery `json:"query"`
	Source SourceInfo `json:"source"`
	Page   PageInfo   `json:"page"`
}

type HotelResult struct {
	Status        string     `json:"status"`
	Hotel         Hotel      `json:"hotel"`
	Source        SourceInfo `json:"source"`
	DetailsSource SourceInfo `json:"details_source"`
}

type OfferResult struct {
	Status string     `json:"status"`
	Offers []Offer    `json:"offers"`
	Query  OfferQuery `json:"query"`
	Source SourceInfo `json:"source"`
	Page   PageInfo   `json:"page"`
}

// API is the stable domain contract used by the command package.
type API interface {
	Areas(context.Context, string) (AreaResult, error)
	SearchHotels(context.Context, HotelQuery) (HotelSearchResult, error)
	Hotel(context.Context, string) (HotelResult, error)
	Offers(context.Context, OfferQuery) (OfferResult, error)
	Stats() RequestStats
}

// SourceError never represents successful empty inventory.
type SourceError struct {
	Kind       string `json:"kind"`
	URL        string `json:"url"`
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Cause      error  `json:"-"`
}

func (e *SourceError) Error() string { return e.Kind + ": " + e.Message }
func (e *SourceError) Unwrap() error { return e.Cause }
