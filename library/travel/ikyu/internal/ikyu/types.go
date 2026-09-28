// Package ikyu implements bounded, anonymous, read-only accommodation queries.
package ikyu

import (
	"context"
	"net/http"
	"time"
)

const SchemaVersion = "ikyu-stay-v1"

// Stay follows Ikyu's source unit: adults and A-F children PER ROOM.
// Source quote amounts are totals across all Rooms and all requested nights.
type Stay struct {
	CheckIn  string `json:"check_in"`
	CheckOut string `json:"check_out"`
	Adults   int    `json:"adults_per_room"`
	Rooms    int    `json:"rooms"`
	Children [6]int `json:"children_per_room"`
}
type Options struct {
	HTTPClient       *http.Client
	BaseURL          string
	CacheDir         string
	Refresh          bool
	AllowStale       bool
	MaxRequests      int
	Concurrency      int
	Retries          int
	RequestTimeout   time.Duration
	MaxResponseBytes int64
	Now              func() time.Time
}
type Stats struct {
	Requests      int   `json:"requests"`
	ResponseBytes int64 `json:"response_bytes"`
	CacheHits     int   `json:"cache_hits"`
	StaleHits     int   `json:"stale_hits"`
}
type Freshness struct {
	FetchedAt     time.Time `json:"fetched_at"`
	AgeSeconds    int64     `json:"age_seconds"`
	Source        string    `json:"source"`
	Stale         bool      `json:"stale"`
	SourceError   *string   `json:"source_error,omitempty"`
	SchemaVersion string    `json:"schema_version"`
}
type Pagination struct {
	NextOffset *int `json:"next_offset"`
	Limit      int  `json:"limit"`
	Offset     int  `json:"offset"`
	Returned   int  `json:"returned"`
	Total      int  `json:"source_total"`
	HasNext    bool `json:"has_next"`
	Complete   bool `json:"complete"`
	Scanned    int  `json:"scanned"`
}
type Coverage struct {
	Applied []string `json:"applied"`
	Unknown []string `json:"unknown"`
	Basis   string   `json:"basis"`
	Note    string   `json:"note,omitempty"`
}
type Destination struct {
	ObservedName      *string  `json:"observed_name,omitempty"`
	FilterRemovalNote *string  `json:"filter_removal_note,omitempty"`
	ObservedPath      string   `json:"observed_path,omitempty"`
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Path              string   `json:"path"`
	Aliases           []string `json:"aliases,omitempty"`
}
type Attribute struct {
	Available *bool  `json:"available,omitempty"`
	Value     string `json:"value"`
	Name      string `json:"name"`
}
type CategoryScores struct {
	Count           *int     `json:"count"`
	Average         *float64 `json:"average"`
	RoomAmenity     *float64 `json:"room_amenity"`
	CustomerService *float64 `json:"customer_service"`
	Equipment       *float64 `json:"equipment"`
	Bath            *float64 `json:"bath"`
	Meal            *float64 `json:"meal"`
	Satisfaction    *float64 `json:"satisfaction"`
}
type PreviewMatch struct {
	RoomID           string `json:"room_id"`
	PlanID           string `json:"plan_id"`
	Meal             Meal   `json:"meal"`
	PointVariation   *int   `json:"source_point_variation"`
	DateEchoVerified bool   `json:"date_echo_verified"`
}

type Property struct {
	Match          *PreviewMatch  `json:"qualifying_preview,omitempty"`
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Type           *string        `json:"type"`
	URL            string         `json:"url"`
	Area           *string        `json:"area"`
	Address        *string        `json:"address"`
	Latitude       *float64       `json:"latitude"`
	Longitude      *float64       `json:"longitude"`
	Attributes     []Attribute    `json:"attributes"`
	Scores         CategoryScores `json:"scores"`
	Notes          *string        `json:"notes"`
	BathFacilities []BathFacility `json:"bath_facilities,omitempty"`
	Price          *Price         `json:"from_price"`
	DetailGap      *string        `json:"detail_gap,omitempty"`
}
type BathFacility struct {
	Name      string `json:"name"`
	HotSpring *bool  `json:"hot_spring"`
	Note      string `json:"note"`
}
type BathEvidence struct {
	Private     *bool       `json:"private"`
	Shared      *bool       `json:"shared"`
	Outdoor     *bool       `json:"outdoor"`
	SemiOutdoor *bool       `json:"semi_outdoor"`
	HotSpring   *bool       `json:"hot_spring"`
	Proof       []Attribute `json:"proof"`
}
type Bed struct {
	Count    *int `json:"count"`
	People   *int `json:"people"`
	WidthCM  *int `json:"width_cm"`
	LengthCM *int `json:"length_cm"`
}
type Room struct {
	CheckInFrom    *string       `json:"check_in_from"`
	CheckInTo      *string       `json:"check_in_to"`
	CheckOut       *string       `json:"check_out"`
	PlanPagination Pagination    `json:"plan_pagination"`
	ID             string        `json:"id"`
	PropertyID     string        `json:"property_id"`
	Name           string        `json:"name"`
	Type           *string       `json:"type"`
	URL            string        `json:"url"`
	SizeM2         *float64      `json:"size_m2"`
	SizeMinM2      *float64      `json:"size_min_m2"`
	SizeMaxM2      *float64      `json:"size_max_m2"`
	SizeText       *string       `json:"source_size_text"`
	BeddingText    *string       `json:"bedding_text"`
	ViewEvidence   []string      `json:"view_evidence"`
	CapacityMin    *int          `json:"capacity_min"`
	CapacityMax    *int          `json:"capacity_max"`
	Beds           []Bed         `json:"beds"`
	Layout         *string       `json:"layout"`
	Description    *string       `json:"description"`
	Attributes     []Attribute   `json:"attributes"`
	Bath           BathEvidence  `json:"bath"`
	Plans          []PlanSummary `json:"plans"`
	PlanTotal      *int          `json:"plan_total"`
}
type Meal struct {
	Code string `json:"code"`
	Name string `json:"name"`
}
type CancellationRule struct {
	Type        string  `json:"type"`
	Day         *int    `json:"day"`
	HourMinutes *string `json:"hour_minutes"`
	Rate        *int    `json:"rate"`
	Amount      *int64  `json:"amount"`
}
type Cancellation struct {
	ID    string             `json:"id"`
	Rules []CancellationRule `json:"rules"`
	Known bool               `json:"known"`
}
type ChildPrice struct {
	Kind   string `json:"kind"`
	Type   string `json:"type"`
	Rate   *int   `json:"rate"`
	Amount *int64 `json:"amount"`
}
type MealDetail struct {
	Type        *string `json:"type"`
	Name        *string `json:"name"`
	Place       *string `json:"place"`
	Menu        *string `json:"menu"`
	Description *string `json:"description"`
	Notes       *string `json:"notes"`
	OpenAllDay  *bool   `json:"open_all_day"`
	StartTime   *string `json:"start_time"`
	EndTime     *string `json:"end_time"`
	LastOrder   *string `json:"last_order_time"`
}

type Plan struct {
	MealDetails      []MealDetail `json:"meal_details"`
	MealDetailsKnown bool         `json:"meal_details_known"`
	CheckInFrom      *string      `json:"check_in_from"`
	CheckInTo        *string      `json:"check_in_to"`
	CheckOut         *string      `json:"check_out"`
	UseCheckInOut    *bool        `json:"source_use_check_in_out"`
	PointVariation   *int         `json:"source_point_variation"`
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	Meal             Meal         `json:"meal"`
	Cancellation     Cancellation `json:"cancellation"`
	Payment          *string      `json:"payment"`
	Notes            *string      `json:"notes"`
	Children         []ChildPrice `json:"children"`
	MinNights        *int         `json:"min_nights"`
	MaxNights        *int         `json:"max_nights"`
	MemberRank       *string      `json:"member_rank"`
	BathTaxApply     *bool        `json:"source_bath_tax_apply"`
	MinRooms         *int         `json:"min_rooms"`
	MaxRooms         *int         `json:"max_rooms"`
	Content          *string      `json:"content,omitempty"`
}
type Coupon struct {
	Name             *string `json:"name"`
	DiscountAmount   *int64  `json:"discount_amount"`
	EligibilityKnown bool    `json:"eligibility_known"`
}

type PriceScenario struct {
	Name                   string `json:"name"`
	PublishedPayable       *int64 `json:"published_payable"`
	PotentialPointsEarned  *int64 `json:"potential_points_earned"`
	PotentialPointsApplied *int64 `json:"potential_points_applied"`
	EligibilityKnown       bool   `json:"eligibility_known"`
}

type Price struct {
	HeadlineBeforePoints        *int64          `json:"headline_before_points"`
	Scenarios                   []PriceScenario `json:"scenarios"`
	Currency                    string          `json:"currency"`
	Unit                        string          `json:"unit"`
	Amount                      *int64          `json:"source_amount"`
	BaseDiscountAmount          *int64          `json:"base_discount_amount"`
	DiscountAmount              *int64          `json:"instant_points_payable"`
	DiscountAmountEarn          *int64          `json:"earn_points_payable"`
	DiscountAmountWithoutCoupon *int64          `json:"payable_without_coupon"`
	Point                       *int64          `json:"points_earned"`
	PointRate                   *float64        `json:"points_rate"`
	InstantPoint                *int64          `json:"points_applied"`
	InstantPointRate            *float64        `json:"points_applied_rate"`
	Coupon                      *Coupon         `json:"source_coupon"`
	CheckoutConfirmedPayable    *int64          `json:"checkout_confirmed_payable"`
	EligibilityKnown            bool            `json:"eligibility_known"`
	Assumptions                 []string        `json:"assumptions"`
}
type PlanSummary struct {
	PointVariation *int   `json:"source_point_variation"`
	ID             string `json:"id"`
	Name           string `json:"name"`
	Meal           Meal   `json:"meal"`
	Price          Price  `json:"price"`
	Inventory      *int   `json:"inventory"`
	URL            string `json:"url"`
}
type DatedOffer struct {
	Stay              Stay   `json:"stay"`
	EchoStay          *Stay  `json:"source_stay"`
	Available         *bool  `json:"available"`
	Inventory         *int   `json:"inventory"`
	Price             *Price `json:"price"`
	UnavailableReason string `json:"unavailable_reason,omitempty"`
	DateVerified      bool   `json:"date_verified"`
}
type Preferences struct {
	MinBudget     *int64   `json:"min_budget,omitempty"`
	MaxBudget     *int64   `json:"max_budget,omitempty"`
	Meals         []string `json:"meals,omitempty"`
	OutdoorBath   bool     `json:"outdoor_bath,omitempty"`
	HotSpringBath bool     `json:"hot_spring_bath,omitempty"`
	Nonsmoking    bool     `json:"nonsmoking,omitempty"`
	MinSizeM2     *float64 `json:"min_size_m2,omitempty"`
}
type SearchRequest struct {
	Destination string      `json:"destination"`
	Stay        Stay        `json:"stay"`
	Limit       int         `json:"limit"`
	Offset      int         `json:"offset"`
	Preferences Preferences `json:"preferences"`
}
type RoomsRequest struct {
	PlanLimit   int         `json:"plan_limit"`
	PlanOffset  int         `json:"plan_offset"`
	PropertyID  string      `json:"property_id"`
	Stay        Stay        `json:"stay"`
	Limit       int         `json:"limit"`
	Offset      int         `json:"offset"`
	Preferences Preferences `json:"preferences"`
}
type OfferRequest struct {
	PropertyID string `json:"property_id"`
	RoomID     string `json:"room_id"`
	PlanID     string `json:"plan_id"`
	Stay       Stay   `json:"stay"`
}
type DestinationsResult struct {
	Data       []Destination `json:"data"`
	Pagination Pagination    `json:"pagination"`
	Freshness  Freshness     `json:"freshness"`
}
type SearchResult struct {
	Dependencies map[string]Freshness `json:"dependency_freshness"`
	Data         []Property           `json:"data"`
	Stay         Stay                 `json:"stay"`
	Pagination   Pagination           `json:"pagination"`
	Coverage     Coverage             `json:"filter_coverage"`
	Freshness    Freshness            `json:"freshness"`
}
type PropertyResult struct {
	Data      Property  `json:"data"`
	Freshness Freshness `json:"freshness"`
}
type RoomsResult struct {
	OccupancyEchoVerified bool       `json:"occupancy_echo_verified"`
	DateEchoVerified      bool       `json:"date_echo_verified"`
	Data                  []Room     `json:"data"`
	Stay                  Stay       `json:"stay"`
	Pagination            Pagination `json:"pagination"`
	Coverage              Coverage   `json:"filter_coverage"`
	Freshness             Freshness  `json:"freshness"`
}
type OfferData struct {
	Property Property   `json:"property"`
	Room     Room       `json:"room"`
	Plan     Plan       `json:"plan"`
	Offer    DatedOffer `json:"offer"`
	URL      string     `json:"url"`
}
type OfferResult struct {
	Data      OfferData `json:"data"`
	Freshness Freshness `json:"freshness"`
}

// Reader is the narrow command-facing interface, also used by deterministic tests.
type Reader interface {
	Destinations(context.Context, string, int, int) (DestinationsResult, error)
	Search(context.Context, SearchRequest) (SearchResult, error)
	Property(context.Context, string) (PropertyResult, error)
	Rooms(context.Context, RoomsRequest) (RoomsResult, error)
	Offer(context.Context, OfferRequest) (OfferResult, error)
	Stats() Stats
}
