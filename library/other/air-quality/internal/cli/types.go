// Copyright 2026 Dhilip Subramanian and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

type GuidanceResult struct {
	Source     string         `json:"source"`
	Configured bool           `json:"configured"`
	Command    string         `json:"command"`
	Query      map[string]any `json:"query,omitempty"`
	Title      string         `json:"title"`
	Setup      []string       `json:"setup"`
	Caveats    []string       `json:"caveats"`
}

type Measurement struct {
	Parameter string `json:"parameter,omitempty"`
	Value     string `json:"value,omitempty"`
	Unit      string `json:"unit,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
	SensorID  string `json:"sensor_id,omitempty"`
	Raw       any    `json:"raw,omitempty"`
}

type LocationSummary struct {
	ID          string             `json:"id,omitempty"`
	Name        string             `json:"name,omitempty"`
	Locality    string             `json:"locality,omitempty"`
	Country     string             `json:"country,omitempty"`
	Coordinates map[string]float64 `json:"coordinates,omitempty"`
	Providers   []string           `json:"providers,omitempty"`
	Sensors     []SensorSummary    `json:"sensors,omitempty"`
	Raw         any                `json:"raw,omitempty"`
}

type SensorSummary struct {
	ID        string `json:"id,omitempty"`
	Parameter string `json:"parameter,omitempty"`
	Unit      string `json:"unit,omitempty"`
}

type CurrentResult struct {
	Source       string          `json:"source"`
	Configured   bool            `json:"configured"`
	Query        map[string]any  `json:"query"`
	Location     LocationSummary `json:"location"`
	Measurements []Measurement   `json:"measurements"`
	Freshness    string          `json:"freshness,omitempty"`
	Caveats      []string        `json:"caveats"`
	Setup        []string        `json:"setup,omitempty"`
	Raw          map[string]any  `json:"raw,omitempty"`
}

type NearestResult struct {
	Source     string            `json:"source"`
	Configured bool              `json:"configured"`
	Query      map[string]any    `json:"query"`
	Locations  []LocationSummary `json:"locations"`
	Caveats    []string          `json:"caveats"`
	Setup      []string          `json:"setup,omitempty"`
}

type HistoryResult struct {
	Source       string         `json:"source"`
	Configured   bool           `json:"configured"`
	Query        map[string]any `json:"query"`
	Measurements []Measurement  `json:"measurements"`
	Freshness    string         `json:"freshness,omitempty"`
	Caveats      []string       `json:"caveats"`
	Setup        []string       `json:"setup,omitempty"`
}

type CompareResult struct {
	Source     string         `json:"source"`
	Configured bool           `json:"configured"`
	Query      map[string]any `json:"query"`
	Left       CurrentResult  `json:"left"`
	Right      CurrentResult  `json:"right"`
	Caveats    []string       `json:"caveats"`
	Setup      []string       `json:"setup,omitempty"`
}

type AirNowObservation struct {
	ReportingArea string `json:"reporting_area,omitempty"`
	StateCode     string `json:"state_code,omitempty"`
	Latitude      string `json:"latitude,omitempty"`
	Longitude     string `json:"longitude,omitempty"`
	Parameter     string `json:"parameter,omitempty"`
	AQI           any    `json:"aqi,omitempty"`
	Category      string `json:"category,omitempty"`
	Observed      string `json:"observed,omitempty"`
	Raw           any    `json:"raw,omitempty"`
}

type AirNowResult struct {
	Source       string              `json:"source"`
	Configured   bool                `json:"configured"`
	Query        map[string]any      `json:"query"`
	Observations []AirNowObservation `json:"observations"`
	Freshness    string              `json:"freshness,omitempty"`
	Caveats      []string            `json:"caveats"`
	Setup        []string            `json:"setup,omitempty"`
}

type SourcesResult struct {
	Source  string       `json:"source"`
	Sources []SourceInfo `json:"sources"`
	Caveats []string     `json:"caveats"`
}

type SourceInfo struct {
	Name       string   `json:"name"`
	URL        string   `json:"url"`
	Configured bool     `json:"configured"`
	Auth       string   `json:"auth"`
	Limits     []string `json:"limits"`
	Caveats    []string `json:"caveats"`
}

type DoctorResult struct {
	Source           string   `json:"source"`
	OpenAQConfigured bool     `json:"openaq_configured"`
	AirNowConfigured bool     `json:"airnow_configured"`
	EnabledCommands  []string `json:"enabled_commands"`
	MissingCommands  []string `json:"missing_commands"`
	Caveats          []string `json:"caveats"`
}
