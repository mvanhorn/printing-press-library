// Package tenki reads bounded, public tenki.jp HTML products.
package tenki

import (
	"net/http"
	"time"
)

const Timezone = "Asia/Tokyo"

type Config struct {
	CacheDir   string
	Refresh    bool
	AllowStale bool
	Local      bool
	Transport  http.RoundTripper
	Now        func() time.Time
	Timeout    time.Duration
	// RateLimit can lower this client's 1 request/second ceiling. Zero uses that default.
	// Each command invocation creates a client; concurrent CLI/MCP invocations pace independently.
	RateLimit float64
	// SearchDirectory selects an explicit leisure directory for bounded name filtering.
	SearchDirectory string
}

type Metrics struct {
	HTTPRequests  int   `json:"http_requests"`
	CacheHits     int   `json:"cache_hits"`
	ResponseBytes int64 `json:"response_bytes"`
}

type Source struct {
	URL                string `json:"url"`
	IssueAt            string `json:"issue_at,omitempty"`
	FetchedAt          string `json:"fetched_at"`
	ExpiresAt          string `json:"expires_at"`
	Timezone           string `json:"timezone"`
	FromCache          bool   `json:"from_cache"`
	CacheStale         bool   `json:"cache_stale"`
	SourceStale        bool   `json:"source_stale"`
	Freshness          string `json:"freshness"`
	FreshnessReference string `json:"freshness_reference,omitempty"`
	FreshnessAt        string `json:"freshness_at,omitempty"`
	IssueRaw           string `json:"issue_raw,omitempty"`
}

type Place struct {
	ID                    string   `json:"id"`
	Name                  string   `json:"name"`
	Kind                  string   `json:"kind"`
	URL                   string   `json:"url"`
	Prefecture            string   `json:"prefecture,omitempty"`
	Address               string   `json:"address,omitempty"`
	PostalCode            string   `json:"postal_code,omitempty"`
	NameSource            string   `json:"name_source,omitempty"`
	ForecastReferenceURL  string   `json:"forecast_reference_url,omitempty"`
	ForecastReferenceName string   `json:"forecast_reference_name,omitempty"`
	Scope                 string   `json:"scope"`
	ElevationM            *float64 `json:"elevation_m"`
}

type PlaceResult struct {
	Place    Place    `json:"place"`
	Source   Source   `json:"source"`
	Status   string   `json:"status"`
	Warnings []string `json:"warnings"`
}

type SearchResult struct {
	Places       []Place  `json:"places"`
	Source       Source   `json:"source"`
	Status       string   `json:"status"`
	Warnings     []string `json:"warnings"`
	Ambiguous    bool     `json:"ambiguous"`
	Truncated    bool     `json:"truncated"`
	Pages        int      `json:"pages"`
	Scanned      int      `json:"scanned"`
	SearchScope  string   `json:"search_scope"`
	DirectoryURL string   `json:"directory_url,omitempty"`
}

// Period keeps precipitation intervals distinct from instantaneous values.
// Numeric absence is always null. Daily minimum is the morning low and the
// maximum is the daytime high, not guaranteed whole-day extrema.
type Period struct {
	Date                   string   `json:"date"`
	Start                  string   `json:"start,omitempty"`
	End                    string   `json:"end,omitempty"`
	ValidAt                string   `json:"valid_at,omitempty"`
	Kind                   string   `json:"kind"`
	TemperatureKind        string   `json:"temperature_kind,omitempty"`
	WeatherProbabilityKind string   `json:"weather_probability_kind,omitempty"`
	WeatherProbabilityFrom string   `json:"weather_probability_from,omitempty"`
	Weather                string   `json:"weather,omitempty"`
	TemperatureC           *float64 `json:"temperature_c"`
	MinTemperatureC        *float64 `json:"min_temperature_c"`
	MaxTemperatureC        *float64 `json:"max_temperature_c"`
	PrecipProbabilityPct   *float64 `json:"precip_probability_pct"`
	PrecipAmountMM         *float64 `json:"precip_amount_mm"`
	PrecipRateMMH          *float64 `json:"precip_rate_mm_h"`
	WindSpeedMS            *float64 `json:"wind_speed_m_s"`
	WindDirection          string   `json:"wind_direction,omitempty"`
	HumidityPct            *float64 `json:"humidity_pct"`
	Confidence             string   `json:"confidence,omitempty"`
	Partial                bool     `json:"partial"`
}

type ForecastResult struct {
	Place    Place    `json:"place"`
	Source   Source   `json:"source"`
	Status   string   `json:"status"`
	Warnings []string `json:"warnings"`
	Periods  []Period `json:"periods"`
	// Daily detail has four six-hour intervals and five instant values per
	// supported date. These arrays intentionally have different lengths.
	Intervals     []Period `json:"intervals,omitempty"`
	Instants      []Period `json:"instants,omitempty"`
	CoverageStart string   `json:"coverage_start,omitempty"`
	CoverageEnd   string   `json:"coverage_end,omitempty"`
}

type SeasonalSpot struct {
	Place                  Place    `json:"place"`
	Year                   int      `json:"year"`
	Condition              string   `json:"condition,omitempty"`
	ReportAt               string   `json:"report_at,omitempty"`
	ReportDate             string   `json:"report_date,omitempty"`
	ReportRaw              string   `json:"report_raw,omitempty"`
	PredictedFloweringDate string   `json:"predicted_flowering_date,omitempty"`
	PredictedFullBloomDate string   `json:"predicted_full_bloom_date,omitempty"`
	PredictedBestPeriod    string   `json:"predicted_best_period,omitempty"`
	NormalPeriod           string   `json:"normal_period,omitempty"`
	Species                []string `json:"species"`
}

type SeasonalListResult struct {
	Kind          string         `json:"kind"`
	Year          int            `json:"year"`
	RequestedYear int            `json:"requested_year"`
	YearRelation  string         `json:"year_relation"`
	UpdateState   string         `json:"update_state"`
	Scanned       int            `json:"scanned"`
	Spots         []SeasonalSpot `json:"spots"`
	Source        Source         `json:"source"`
	Status        string         `json:"status"`
	Warnings      []string       `json:"warnings"`
	Truncated     bool           `json:"truncated"`
	Pages         int            `json:"pages"`
}

type SeasonalResult struct {
	Kind          string       `json:"kind"`
	Year          int          `json:"year"`
	RequestedYear int          `json:"requested_year"`
	YearRelation  string       `json:"year_relation"`
	UpdateState   string       `json:"update_state"`
	Spot          SeasonalSpot `json:"spot"`
	Place         Place        `json:"place"`
	Source        Source       `json:"source"`
	Status        string       `json:"status"`
	Warnings      []string     `json:"warnings"`
}

type ModelLevel struct {
	ElevationM    float64  `json:"elevation_m"`
	ValidAt       string   `json:"valid_at"`
	TemperatureC  *float64 `json:"temperature_c"`
	WindSpeedMS   *float64 `json:"wind_speed_m_s"`
	WindDirection string   `json:"wind_direction,omitempty"`
}

type MountainResult struct {
	Place                   Place        `json:"place"`
	Source                  Source       `json:"source"`
	Status                  string       `json:"status"`
	Warnings                []string     `json:"warnings"`
	ModelKind               string       `json:"model_kind"`
	ModelInitialAt          string       `json:"model_initial_at,omitempty"`
	ModelInitialRaw         string       `json:"model_initial_raw,omitempty"`
	ModelSource             Source       `json:"model_source"`
	Levels                  []ModelLevel `json:"levels"`
	SummitForecastAvailable bool         `json:"summit_forecast_available"`
}
