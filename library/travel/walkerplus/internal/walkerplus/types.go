// Package walkerplus reads bounded public Walkerplus HTML and preserves source
// facts separately from the date matches derived for a trip.
package walkerplus

import (
	"net/http"
	"time"
)

type Options struct {
	BaseURL     string
	CacheDir    string
	CacheTTL    time.Duration
	Timeout     time.Duration
	NoCache     bool
	Refresh     bool
	Concurrency int
	Retries     int
	HTTPClient  *http.Client
}

type Query struct {
	cityPath   string
	cityName   string
	Prefecture string `json:"prefecture"`
	City       string `json:"city"`
	Category   string `json:"category"`
	From       string `json:"from"`
	To         string `json:"to"`
	Timing     string `json:"timing"`
	Sort       string `json:"sort"`
	Limit      int    `json:"limit"`
	Page       int    `json:"page"`
	MaxPages   int    `json:"max_pages"`
	MaxDetails int    `json:"max_details"`
	Free       bool   `json:"free"`
	Indoor     bool   `json:"indoor"`
}

type Result struct {
	Query    Query    `json:"query"`
	Events   []Event  `json:"events"`
	Coverage Coverage `json:"coverage"`
}

type Coverage struct {
	CatalogRequests  int      `json:"catalog_requests"`
	CatalogRoutes    []string `json:"catalog_routes"`
	RequestedPages   int      `json:"requested_pages"`
	ScannedPages     int      `json:"scanned_pages"`
	CandidateCount   int      `json:"candidate_count"`
	DetailCount      int      `json:"detail_count"`
	ReturnedCount    int      `json:"returned_count"`
	ExcludedCount    int      `json:"excluded_count"`
	RequestCount     int      `json:"request_count"`
	CacheHits        int      `json:"cache_hits"`
	ElapsedMS        int64    `json:"elapsed_ms"`
	Truncated        bool     `json:"truncated"`
	Incomplete       bool     `json:"incomplete"`
	Reasons          []string `json:"reasons"`
	Routes           []string `json:"routes"`
	NativeYearLabels []string `json:"native_year_labels"`
	NextPage         *string  `json:"next_page"`
	SourceTotal      *int     `json:"source_total"`
}

type Event struct {
	Timezone            string        `json:"timezone"`
	ID                  string        `json:"id"`
	TitleJA             string        `json:"title_ja"`
	SourceURL           string        `json:"source_url"`
	StartDate           *string       `json:"start_date"`
	EndDate             *string       `json:"end_date"`
	EditionYear         *int          `json:"edition_year"`
	DateCertainty       string        `json:"date_certainty"`
	Location            Location      `json:"location"`
	Categories          []CatalogItem `json:"categories"`
	Description         *string       `json:"description"`
	Schedule            Schedule      `json:"schedule"`
	Hours               *string       `json:"hours"`
	Access              *string       `json:"access"`
	Admission           Admission     `json:"admission"`
	Indoor              *bool         `json:"indoor"`
	ReservationRequired *bool         `json:"reservation_required"`
	ReservationText     *string       `json:"reservation_text"`
	Weather             *string       `json:"weather"`
	Cancellation        *string       `json:"cancellation"`
	OrganizerURLs       []string      `json:"organizer_urls"`
	Evidence            []Evidence    `json:"evidence"`
	Sources             []Source      `json:"sources"`
	SourceUpdated       *string       `json:"source_updated"`
	Match               *Match        `json:"match"`
	indoorBlocked       bool
	venueFromDisplay    bool
}

type Location struct {
	PrefectureCode *string `json:"prefecture_code"`
	PrefectureJA   *string `json:"prefecture_ja"`
	CityCode       *string `json:"city_code"`
	CityJA         *string `json:"city_ja"`
	Venue          *string `json:"venue"`
	Address        *string `json:"address"`
}

type Schedule struct {
	Raw             *string  `json:"raw"`
	OccurrenceDates []string `json:"occurrence_dates"`
	ExcludedDates   []string `json:"excluded_dates"`
	ClosedWeekdays  []string `json:"closed_weekdays"`
	Recurrence      *string  `json:"recurrence"`
	Unresolved      []string `json:"unresolved"`
	weekdays        []time.Weekday
	closed          []time.Weekday
	daily           bool
}

type Admission struct {
	Status   string   `json:"status"`
	Price    *float64 `json:"price"`
	Currency *string  `json:"currency"`
	Raw      *string  `json:"raw"`
}

type Evidence struct {
	Field     string `json:"field"`
	Text      string `json:"text"`
	SourceURL string `json:"source_url"`
}

type Source struct {
	URL             string `json:"url"`
	FetchedAt       string `json:"fetched_at"`
	CacheHit        bool   `json:"cache_hit"`
	CacheAgeSeconds int64  `json:"cache_age_seconds"`
}

type Match struct {
	Timing              string   `json:"timing"`
	State               string   `json:"state"`
	EnvelopeOverlapDays int      `json:"envelope_overlap_days"`
	ConfirmedDays       []string `json:"confirmed_days"`
	PossibleDays        []string `json:"possible_days"`
	Reasons             []string `json:"reasons"`
}

type CatalogItem struct {
	Code           string   `json:"code"`
	NameJA         string   `json:"name_ja"`
	Aliases        []string `json:"aliases"`
	Path           string   `json:"path"`
	Kind           string   `json:"kind"`
	PrefectureCode *string  `json:"prefecture_code"`
}

type CatalogResult struct {
	Items     []CatalogItem `json:"items"`
	SourceURL string        `json:"source_url"`
	Coverage  Coverage      `json:"coverage"`
}
