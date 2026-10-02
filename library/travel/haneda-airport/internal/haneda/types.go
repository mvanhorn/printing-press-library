// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// Package haneda normalizes the airport's anonymous, first-party source contracts.
package haneda

import "time"

const Origin = "https://www.tokyo-haneda.com"
const SnapshotSchema = "haneda-board-snapshot/v1"

var JST = time.FixedZone("Asia/Tokyo", 9*60*60)

type Airport struct {
	Code        string `json:"airport_code"`
	SearchValue string `json:"search_value"`
	Name        string `json:"name"`
	NameJA      string `json:"name_ja"`
	Region      string `json:"region"`
	Kind        string `json:"kind"`
}
type Airline struct {
	Code   string `json:"airline_code"`
	Prefix string `json:"flight_prefix"`
	Name   string `json:"name"`
	NameJA string `json:"name_ja"`
	URL    string `json:"source_url"`
	Kind   string `json:"kind"`
}
type ListedFlight struct {
	Number      string `json:"flight_number"`
	AirlineCode string `json:"airline_code"`
	Name        string `json:"airline_name"`
	NameJA      string `json:"airline_name_ja"`
	URL         string `json:"airline_url"`
}
type Status struct {
	Category string  `json:"category"`
	Text     string  `json:"text"`
	Reason   *string `json:"reason"`
	Known    bool    `json:"known"`
}
type Facility struct {
	Type   string  `json:"type"`
	Title  string  `json:"title"`
	Name   string  `json:"name"`
	MapURL *string `json:"map_url"`
}
type Flight struct {
	ID                  string         `json:"id"`
	Kind                string         `json:"kind"`
	Direction           string         `json:"direction"`
	ServiceDate         string         `json:"service_date"`
	SourcePrimaryFlight string         `json:"source_primary_flight"`
	OperatingFlight     *string        `json:"operating_flight"`
	ListedFlights       []ListedFlight `json:"listed_flights"`
	CodeshareFlights    []string       `json:"codeshare_flights"`
	Airport             *Airport       `json:"other_airport"`
	SourceAreaName      string         `json:"source_area_name"`
	ViaAreaName         *string        `json:"via_area_name"`
	ScheduledAt         *string        `json:"scheduled_at"`
	RevisedAt           *string        `json:"revised_at"`
	ActualAt            *string        `json:"actual_at"`
	TimeChangeMinutes   *int           `json:"time_change_minutes"`
	Status              Status         `json:"status"`
	Terminal            *string        `json:"terminal"`
	BoardingGates       []string       `json:"boarding_gates"`
	CheckinCounters     []string       `json:"checkin_counters"`
	SecurityChecks      []string       `json:"security_checks"`
	ArrivalExits        []string       `json:"arrival_exits"`
	Facilities          []Facility     `json:"facilities,omitempty"`
	UnknownFields       []string       `json:"unknown_fields"`
	SourceURL           string         `json:"source_url"`
	SourceReportedAt    *string        `json:"source_reported_at"`
}
type Coverage struct {
	Kind          string `json:"kind"`
	Direction     string `json:"direction"`
	RequestedDate string `json:"requested_date"`
	Origin        string `json:"origin"`
	QueryMode     string `json:"query_mode"`
	FlightLookup  string `json:"flight_lookup,omitempty"`
}
type SourceInfo struct {
	Kind               string  `json:"kind"`
	Direction          string  `json:"direction,omitempty"`
	URL                string  `json:"url"`
	ReportedAt         *string `json:"reported_at"`
	UpdatedAt          *string `json:"updated_at"`
	TimestampSemantics string  `json:"timestamp_semantics"`
	SourceTotal        int     `json:"source_total"`
	PeriodStart        string  `json:"period_start,omitempty"`
	PeriodEnd          string  `json:"period_end,omitempty"`
}
type Budget struct {
	Requests      int   `json:"request_count"`
	ResponseBytes int64 `json:"response_bytes"`
	MaxRequests   int   `json:"max_requests"`
	MaxBodyBytes  int64 `json:"max_body_bytes"`
	MaxTotalBytes int64 `json:"max_total_bytes"`
}
type BoardResult struct {
	ObservedAt         string       `json:"observed_at"`
	Coverage           Coverage     `json:"coverage"`
	Sources            []SourceInfo `json:"sources"`
	SourceUpdatedAt    *string      `json:"source_updated_at"`
	Flights            []Flight     `json:"flights"`
	TotalMatches       int          `json:"total_matches"`
	ScannedRecords     int          `json:"scanned_records"`
	MaxScanRecords     int          `json:"max_scan_records"`
	ScanCapHit         bool         `json:"scan_cap_hit"`
	Offset             int          `json:"offset"`
	Limit              int          `json:"limit"`
	NextOffset         *int         `json:"next_offset"`
	Complete           bool         `json:"complete_source_scope"`
	SnapshotAgeSeconds int64        `json:"snapshot_age_seconds"`
	Stale              bool         `json:"stale_snapshot"`
	Summary            any          `json:"airport_disruption_summary,omitempty"`
	Budget             Budget       `json:"budget"`
	Notes              []string     `json:"notes"`
}
type Query struct {
	Kind, Direction, Date, Flight, Airline, Destination, Status, Terminal string
	Limit, Offset, MaxScan                                                int
	ServiceDayOnly, RolloverOnly, IncludeFacilities                       bool
}
type RawBoard struct {
	Count *int `json:"count"`
	Date  struct {
		Date    string `json:"date"`
		Display string `json:"display_date"`
	} `json:"date"`
	Flights []RawFlight `json:"flightlists"`
}
type RawFlight struct {
	Kind      string `json:"flightType"`
	Direction string `json:"direction"`
	Area      string `json:"area_name"`
	Via       string `json:"via_area_name"`
	Airlines  []struct {
		Code   string `json:"airline"`
		Number string `json:"flightNumber"`
		URL    string `json:"link_url"`
	} `json:"airlines"`
	Date struct {
		Key     string `json:"key"`
		Display string `json:"display_date"`
		Change  string `json:"change_date"`
	} `json:"date"`
	Scheduled string `json:"on_time"`
	Changed   string `json:"change_time"`
	Terminal  *struct {
		Name string `json:"terminal"`
	} `json:"terminal"`
	Status struct {
		Category string `json:"category"`
		Text     string `json:"text"`
		Reason   string `json:"reason"`
	} `json:"status"`
	Options []struct {
		Type  string `json:"type"`
		Title string `json:"title"`
		Items []struct {
			Name string `json:"name"`
			Map  string `json:"pm_map_deep_link"`
		} `json:"items"`
	} `json:"options"`
}
type Snapshot struct {
	Schema  string      `json:"schema"`
	SavedAt string      `json:"saved_at"`
	Board   BoardResult `json:"board"`
}

type Schedule struct {
	ID                   string         `json:"id"`
	Kind                 string         `json:"kind"`
	Direction            string         `json:"direction"`
	PeriodStart          string         `json:"period_start"`
	PeriodEnd            string         `json:"period_end"`
	Weekdays             []string       `json:"weekdays"`
	ListedFlights        []ListedFlight `json:"listed_flights"`
	OperatingFlight      *string        `json:"operating_flight"`
	Airport              *Airport       `json:"other_airport"`
	HanedaTime           *string        `json:"scheduled_haneda_time"`
	HanedaTimezone       string         `json:"haneda_timezone"`
	OtherAirportTime     *string        `json:"other_airport_published_time"`
	OtherAirportTimezone *string        `json:"other_airport_timezone"`
	SourceURL            string         `json:"source_url"`
}
type ScheduleResult struct {
	ObservedAt     string       `json:"observed_at"`
	RequestedDate  string       `json:"requested_date,omitempty"`
	Sources        []SourceInfo `json:"sources"`
	Schedules      []Schedule   `json:"schedules"`
	TotalMatches   int          `json:"total_matches"`
	ScannedRecords int          `json:"scanned_records"`
	ScanCapHit     bool         `json:"scan_cap_hit"`
	Limit          int          `json:"limit"`
	Offset         int          `json:"offset"`
	NextOffset     *int         `json:"next_offset"`
	Budget         Budget       `json:"budget"`
	Notes          []string     `json:"notes"`
}
