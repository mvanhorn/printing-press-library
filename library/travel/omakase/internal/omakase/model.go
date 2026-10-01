// Package omakase reads anonymous first-party restaurant documents.
package omakase

import "time"

const Origin = "https://omakase.in"

type Evidence struct {
	URL       string    `json:"url"`
	FetchedAt time.Time `json:"fetched_at"`
	Cached    bool      `json:"cached"`
	Stale     bool      `json:"stale"`
}
type Summary struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	NameJA    *string `json:"name_ja"`
	Cuisine   *string `json:"cuisine"`
	Area      *string `json:"area"`
	URL       string  `json:"url"`
	SeatState string  `json:"seat_state"`
}
type Page struct {
	Results []Summary           `json:"results"`
	Total   *int                `json:"source_total"`
	Next    *string             `json:"next_url"`
	Filters map[string][]Option `json:"filters"`
}
type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}
type Price struct {
	ServiceChargeIncluded *bool  `json:"service_charge_included"`
	MaximumAmount         *int   `json:"maximum_amount"`
	Amount                *int   `json:"amount"`
	Currency              string `json:"currency"`
	Basis                 string `json:"basis"`
	TaxIncluded           *bool  `json:"tax_included"`
	Minimum               bool   `json:"minimum"`
	Variable              bool   `json:"variable"`
	Raw                   string `json:"raw"`
}
type Course struct {
	Name  string `json:"name"`
	Price Price  `json:"price"`
}
type Fee struct {
	Amount   *int    `json:"amount"`
	Currency string  `json:"currency"`
	Basis    string  `json:"basis"`
	Raw      *string `json:"raw"`
}
type Charge struct {
	Percent *float64 `json:"percent"`
	Raw     *string  `json:"raw"`
}
type Cancellation struct {
	When    string   `json:"when"`
	Percent *float64 `json:"percent"`
}
type Release struct {
	State            string  `json:"state"`
	CurrentPeriod    *string `json:"current_period"`
	NextRoundRaw     *string `json:"next_round_raw"`
	NextRoundAt      *string `json:"next_round_at"`
	Timezone         string  `json:"timezone"`
	MaximumFrequency *string `json:"maximum_frequency"`
}
type Detail struct {
	Summary
	Location         *string           `json:"location"`
	Courses          []Course          `json:"courses"`
	CourseNotes      *string           `json:"course_notes"`
	ServiceCharge    Charge            `json:"service_charge"`
	ReservationFee   Fee               `json:"reservation_fee"`
	Cancellation     []Cancellation    `json:"cancellation"`
	CancellationNote *string           `json:"cancellation_note"`
	Release          Release           `json:"release"`
	ReservationRules *string           `json:"reservation_rules"`
	Information      map[string]string `json:"information"`
	BookingMethod    string            `json:"booking_method"`
	ActionRaw        *string           `json:"action_raw"`
	Access           string            `json:"access"`
	Evidence         []Evidence        `json:"evidence"`
	Warnings         []string          `json:"warnings"`
}
type Membership struct {
	URL                string            `json:"url"`
	MonthlyJPY         *int              `json:"green_monthly_jpy"`
	AnnualJPY          *int              `json:"green_annual_jpy"`
	GoldEligibility    string            `json:"gold_eligibility"`
	GoldPriceJPY       *int              `json:"gold_price_jpy"`
	AccountEligibility *bool             `json:"account_eligible"`
	Features           []string          `json:"features"`
	Sections           map[string]string `json:"sections"`
}
type Stats struct {
	Requests      int   `json:"requests"`
	CacheHits     int   `json:"cache_hits"`
	ResponseBytes int64 `json:"response_bytes"`
}
