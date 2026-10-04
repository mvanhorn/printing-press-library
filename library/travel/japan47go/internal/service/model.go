// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package service

type LeadTime struct {
	Value    int    `json:"value"`
	Unit     string `json:"unit"`
	Original string `json:"original"`
}
type Request struct {
	Status    string     `json:"status"`
	LeadTimes []LeadTime `json:"lead_times"`
	Options   []string   `json:"options"`
	Original  string     `json:"original"`
}
type Amount struct {
	JPY       int     `json:"jpy"`
	Qualifier string  `json:"qualifier"`
	Unit      *string `json:"unit"`
	Original  string  `json:"original"`
}
type Price struct {
	Status     string   `json:"status"`
	Amounts    []Amount `json:"amounts"`
	Qualifiers []string `json:"qualifiers"`
	TotalJPY   *int     `json:"total_jpy"`
	Original   string   `json:"original"`
}
type Duration struct {
	MinMinutes int    `json:"min_minutes"`
	MaxMinutes int    `json:"max_minutes"`
	Original   string `json:"original"`
}
type Schedule struct {
	StartDate *string `json:"start_date"`
	EndDate   *string `json:"end_date"`
	Original  string  `json:"original"`
	Operation string  `json:"operation"`
}
type Service struct {
	ID                    string     `json:"id"`
	NameJA                string     `json:"name_ja"`
	Kind                  string     `json:"kind"`
	PrefectureJA          string     `json:"prefecture_ja"`
	CityJA                string     `json:"city_ja"`
	SourceURL             string     `json:"source_url"`
	SourceUpdatedAt       *string    `json:"source_updated_at"`
	ObservedAt            string     `json:"observed_at"`
	SourceClosed          *bool      `json:"source_closed"`
	OpenNow               *bool      `json:"open_now"`
	Availability          string     `json:"availability"`
	Request               Request    `json:"request"`
	Price                 Price      `json:"price"`
	MinimumParty          *int       `json:"minimum_party"`
	PartyEvidence         string     `json:"party_evidence"`
	Durations             []Duration `json:"durations"`
	DurationsMinutes      []int      `json:"durations_minutes"`
	Schedule              Schedule   `json:"schedule"`
	DescriptionEvidence   string     `json:"description_evidence"`
	AccessEvidence        string     `json:"access_evidence"`
	AccessibilityEvidence string     `json:"accessibility_evidence"`
	LanguageEvidence      string     `json:"language_evidence"`
	OrganizationURLs      []string   `json:"organization_urls"`
	Transport             string     `json:"transport"`
	CacheAgeSeconds       int64      `json:"cache_age_seconds"`
	Stale                 bool       `json:"stale"`
	SourceFailure         *string    `json:"source_failure"`
	CacheWarning          *string    `json:"cache_warning"`
}
type Candidate struct {
	ID               string  `json:"id"`
	NameJA           string  `json:"name_ja"`
	Kind             string  `json:"kind"`
	PrefectureJA     string  `json:"prefecture_ja"`
	CityJA           string  `json:"city_ja"`
	SourceURL        string  `json:"source_url"`
	SourceUpdatedAt  *string `json:"source_updated_at"`
	ObservedAt       string  `json:"observed_at"`
	DetailsInspected bool    `json:"details_inspected"`
}
type Coverage struct {
	Pages         int      `json:"scanned_pages"`
	Records       int      `json:"scanned_records"`
	Returned      int      `json:"returned"`
	MatchingTotal int      `json:"source_matching_total"`
	TotalPages    int      `json:"source_total_pages"`
	PageSize      int      `json:"source_page_size"`
	NextPage      *int     `json:"next_page"`
	More          bool     `json:"more_in_source"`
	Complete      bool     `json:"complete"`
	Routes        []string `json:"routes"`
	Note          string   `json:"note"`
}
type Discovery struct {
	Candidates     []Candidate    `json:"candidates"`
	Query          map[string]any `json:"query"`
	Coverage       Coverage       `json:"coverage"`
	SourceBoundary string         `json:"source_boundary"`
}
