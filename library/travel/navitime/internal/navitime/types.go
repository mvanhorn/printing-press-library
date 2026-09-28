// Package navitime reads and normalizes public NAVITIME timetable responses.
package navitime

import "time"

type Options struct {
	CacheDir         string
	Timeout          time.Duration
	Refresh, NoCache bool
}
type Query struct {
	From     string `json:"from"`
	To       string `json:"to"`
	DepartAt string `json:"depart_at"`
	ArriveBy string `json:"arrive_by"`
	FirstOn  string `json:"first_on"`
	LastOn   string `json:"last_on"`
	Pass     string `json:"pass"`
}
type Metadata struct {
	SourceURL        string     `json:"source_url"`
	FetchedAt        time.Time  `json:"fetched_at"`
	CacheHit         bool       `json:"cache_hit"`
	AgeSeconds       float64    `json:"age_seconds"`
	SourceUpdatedAt  *time.Time `json:"source_updated_at"`
	DataKind         string     `json:"data_kind"`
	LiveStatus       bool       `json:"live_status_available"`
	SeatAvailability bool       `json:"seat_availability_available"`
}
type Metrics struct {
	Requests      int   `json:"requests"`
	CacheHits     int   `json:"cache_hits"`
	CacheWrites   int   `json:"cache_writes"`
	Retries       int   `json:"retries"`
	BytesReceived int64 `json:"bytes_received"`
}
type Coordinates struct {
	Lat          *float64 `json:"latitude"`
	Lon          *float64 `json:"longitude"`
	Alt          *float64 `json:"altitude"`
	AngularUnit  string   `json:"angular_unit"`
	AltitudeUnit string   `json:"altitude_unit"`
}
type Place struct {
	ID          string            `json:"id"`
	Ref         string            `json:"ref"`
	Kind        string            `json:"kind"`
	Name        map[string]string `json:"name"`
	Address     map[string]string `json:"address"`
	Category    any               `json:"category"`
	Coordinates *Coordinates      `json:"coordinates"`
	SourceURL   string            `json:"source_url"`
}
type PlacesResult struct {
	Meta                 Metadata `json:"meta"`
	Query                string   `json:"query"`
	Kind                 string   `json:"kind"`
	Places               []Place  `json:"places"`
	Ambiguous            bool     `json:"ambiguous"`
	SourceCandidateCount int      `json:"source_candidate_count"`
	SourceLimit          int      `json:"source_limit"`
	Notes                []string `json:"notes"`
}
type Point struct {
	Name        string       `json:"name"`
	Kind        *string      `json:"kind"`
	SourceID    *string      `json:"source_id"`
	SpotID      *string      `json:"spot_id"`
	Coordinates *Coordinates `json:"coordinates"`
}
type Fare struct {
	TotalJPY             *int    `json:"total_jpy"`
	ICJPY                *int    `json:"ic_jpy"`
	Currency             string  `json:"currency"`
	Basis                string  `json:"basis"`
	SourceText           *string `json:"source_text"`
	SourceCaveat         *string `json:"source_caveat"`
	PassengerAssumptions *string `json:"passenger_assumptions"`
	PassHolderCostJPY    *int    `json:"pass_holder_cost_jpy"`
}
type SeatOption struct {
	Label         string `json:"label"`
	SupplementJPY *int   `json:"supplement_jpy"`
	Selected      bool   `json:"selected"`
}
type FareGroup struct {
	ID                 string   `json:"id"`
	LegIndexes         []int    `json:"leg_indexes"`
	BaseFareJPY        *int     `json:"base_fare_jpy"`
	DefaultSeatFareJPY *int     `json:"default_seat_fare_jpy"`
	DisplayedTotalJPY  *int     `json:"displayed_total_jpy"`
	ICJPY              *int     `json:"ic_jpy"`
	SourceLabels       []string `json:"source_labels"`
	SourceTexts        []string `json:"source_texts"`
	Basis              string   `json:"basis"`
}
type Leg struct {
	Index               int          `json:"index"`
	Kind                string       `json:"kind"`
	From                Point        `json:"from"`
	To                  Point        `json:"to"`
	LineName            *string      `json:"line_name"`
	Service             *string      `json:"service"`
	Destination         *string      `json:"destination"`
	Platform            *string      `json:"platform"`
	DepartureAt         *string      `json:"departure_at"`
	ArrivalAt           *string      `json:"arrival_at"`
	TimestampProvenance string       `json:"timestamp_provenance"`
	SourceDepartureText *string      `json:"source_departure_text"`
	SourceArrivalText   *string      `json:"source_arrival_text"`
	DurationMinutes     *int         `json:"duration_minutes"`
	WalkingMeters       *int         `json:"walking_meters"`
	FareGroupID         *string      `json:"fare_group_id"`
	SeatFareJPY         *int         `json:"seat_fare_jpy"`
	SeatOptions         []SeatOption `json:"seat_options"`
}
type PassInfo struct {
	RequestedID       *string  `json:"requested_id"`
	Coverage          string   `json:"coverage"`
	SourceTexts       []string `json:"source_texts"`
	PassHolderCostJPY *int     `json:"pass_holder_cost_jpy"`
}
type Route struct {
	ID                   string      `json:"id"`
	IDProvenance         string      `json:"id_provenance"`
	SourceID             *string     `json:"source_id"`
	SourceIndex          int         `json:"source_index"`
	SourceURL            string      `json:"source_url"`
	From                 Point       `json:"from"`
	To                   Point       `json:"to"`
	DepartureAt          *string     `json:"departure_at"`
	ArrivalAt            *string     `json:"arrival_at"`
	TimestampProvenance  string      `json:"timestamp_provenance"`
	DurationMinutes      *int        `json:"duration_minutes"`
	DurationSeconds      *int        `json:"duration_seconds"`
	DurationMinutesBasis string      `json:"duration_minutes_basis"`
	TransportKinds       []string    `json:"transport_kinds"`
	TimingBasis          string      `json:"timing_basis"`
	WalkingMeters        *int        `json:"walking_meters"`
	Transfers            *int        `json:"transfers"`
	Fare                 Fare        `json:"fare"`
	Legs                 []Leg       `json:"legs"`
	FareGroups           []FareGroup `json:"fare_groups"`
	Pass                 PassInfo    `json:"pass"`
}
type RouteResult struct {
	Meta   Metadata `json:"meta"`
	Query  Query    `json:"query"`
	Routes []Route  `json:"routes"`
	Notes  []string `json:"notes"`
}
type Pass struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	SupportStatus        string `json:"support_status"`
	LiveTested           bool   `json:"live_tested"`
	AnonymousMaxSelected int    `json:"anonymous_max_selected"`
}
type PassResult struct {
	Meta   Metadata `json:"meta"`
	Passes []Pass   `json:"passes"`
	Notes  []string `json:"notes"`
}
type DetailResult struct {
	Meta           Metadata `json:"meta"`
	Route          *Route   `json:"route"`
	StoredSnapshot bool     `json:"stored_snapshot"`
	Notes          []string `json:"notes"`
}
type RouteSummary struct {
	ID                   string   `json:"id"`
	IDProvenance         string   `json:"id_provenance"`
	SourceID             *string  `json:"source_id"`
	SourceIndex          int      `json:"source_index"`
	SourceURL            string   `json:"source_url"`
	From                 Point    `json:"from"`
	To                   Point    `json:"to"`
	DepartureAt          *string  `json:"departure_at"`
	ArrivalAt            *string  `json:"arrival_at"`
	DurationMinutes      *int     `json:"duration_minutes"`
	DurationSeconds      *int     `json:"duration_seconds"`
	DurationMinutesBasis string   `json:"duration_minutes_basis"`
	TransportKinds       []string `json:"transport_kinds"`
	TimingBasis          string   `json:"timing_basis"`
	WalkingMeters        *int     `json:"walking_meters"`
	Transfers            *int     `json:"transfers"`
	Fare                 Fare     `json:"fare"`
	Pass                 PassInfo `json:"pass"`
}
type SummaryResult struct {
	Meta                 Metadata       `json:"meta"`
	Query                Query          `json:"query"`
	Routes               []RouteSummary `json:"routes"`
	ReturnedAlternatives int            `json:"returned_alternatives"`
	Notes                []string       `json:"notes"`
}
type CompareOptions struct {
	Sort                                                        string
	MaxDurationMinutes, MaxFareJPY, MaxWalkMeters, MaxTransfers int
}
type CompareResult struct {
	Meta   Metadata       `json:"meta"`
	Query  Query          `json:"query"`
	Scope  string         `json:"scope"`
	Sort   string         `json:"sort"`
	Routes []RouteSummary `json:"routes"`
	Notes  []string       `json:"notes"`
}
type ArgumentError struct{ Message string }

func (e *ArgumentError) Error() string { return e.Message }

type SourceError struct {
	Message string
	Status  int
	URL     string
}

func (e *SourceError) Error() string { return e.Message }

type NotFoundError struct{ Message string }

func (e *NotFoundError) Error() string { return e.Message }
