package domain

import "time"

// Budget preserves the source bracket, not a bill estimate.
type Budget struct {
	Raw    string `json:"raw"`
	MinJPY *int   `json:"min_jpy"`
	MaxJPY *int   `json:"max_jpy"`
	Source string `json:"source"`
}

type Area struct {
	Prefecture string `json:"prefecture"`
	Area1      string `json:"area1"`
	Area2      string `json:"area2"`
	Verified   bool   `json:"verified"`
}

// Restaurant is a complete source snapshot. Personal notes and membership do
// not belong here. Evidence distinguishes unknown from not-yet-fetched facts.
type Restaurant struct {
	ID                      string            `json:"id"`
	Name                    string            `json:"name"`
	URL                     string            `json:"url"`
	SourceURL               string            `json:"source_url"`
	Rating                  *float64          `json:"rating"`
	ReviewCount             *int              `json:"review_count"`
	Categories              []string          `json:"categories"`
	Area                    Area              `json:"area"`
	NearestStation          string            `json:"nearest_station"`
	NearestStationDistanceM *int              `json:"nearest_station_distance_m"`
	LunchBudget             Budget            `json:"lunch_budget"`
	DinnerBudget            Budget            `json:"dinner_budget"`
	ReviewLunchBudget       *Budget           `json:"review_lunch_budget,omitempty"`
	ReviewDinnerBudget      *Budget           `json:"review_dinner_budget,omitempty"`
	Closures                *string           `json:"closures"`
	Hours                   *string           `json:"hours,omitempty"`
	Payment                 *string           `json:"payment,omitempty"`
	Reservation             *string           `json:"reservation,omitempty"`
	Address                 *string           `json:"address,omitempty"`
	Transportation          *string           `json:"transportation,omitempty"`
	ServiceCharge           *string           `json:"service_charge,omitempty"`
	Status                  *string           `json:"status,omitempty"`
	SourceWarnings          []string          `json:"source_warnings,omitempty"`
	Facilities              []string          `json:"facilities,omitempty"`
	Awards                  []string          `json:"awards,omitempty"`
	FetchedAt               time.Time         `json:"fetched_at"`
	Surface                 string            `json:"source_surface"`
	Sections                []string          `json:"sections"`
	Evidence                map[string]string `json:"evidence,omitempty"`
}

// FieldState is the saved-evidence contract consumed by lists audit/refresh.
func (r Restaurant) FieldState(field string) string {
	if state, ok := r.Evidence[field]; ok {
		return state
	}
	if r.Surface != "detail" && (field == "hours" || field == "payment" || field == "reservation" || field == "address" || field == "transportation") {
		return "detail_not_fetched"
	}
	return "source_unknown"
}

type Meta struct {
	Source             string         `json:"source"`
	SourceURL          string         `json:"source_url,omitempty"`
	FetchedAt          time.Time      `json:"fetched_at,omitzero"`
	AgeSeconds         int64          `json:"age_seconds"`
	Stale              bool           `json:"stale"`
	Criteria           map[string]any `json:"criteria,omitempty"`
	SourceSort         string         `json:"source_sort,omitempty"`
	SourceSurface      string         `json:"source_surface,omitempty"`
	BudgetSource       string         `json:"budget_source,omitempty"`
	Returned           int            `json:"returned"`
	Scanned            int            `json:"scanned"`
	Pages              int            `json:"pages"`
	HasMore            bool           `json:"has_more"`
	NextURL            string         `json:"next_url,omitempty"`
	Requests           int            `json:"requests"`
	Bytes              int64          `json:"bytes"`
	Coverage           string         `json:"coverage,omitempty"`
	OmittedFromScanned int            `json:"omitted_from_scanned,omitempty"`
	NextURLScope       string         `json:"next_url_scope,omitempty"`
	Note               string         `json:"note,omitempty"`
	PartialFailure     bool           `json:"partial_failure,omitempty"`
	Failed             int            `json:"failed,omitempty"`
}

type Envelope struct {
	Items any  `json:"items"`
	Meta  Meta `json:"meta"`
}

type Choice struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Selector   string `json:"selector"`
	URL        string `json:"url,omitempty"`
	Prefecture string `json:"prefecture,omitempty"`
	Area1      string `json:"area1,omitempty"`
	Area2      string `json:"area2,omitempty"`
	StationID  string `json:"station_id,omitempty"`
	Genre      string `json:"genre,omitempty"`
	Exact      bool   `json:"exact_match"`
}
