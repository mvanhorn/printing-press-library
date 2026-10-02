// Copyright 2026 zjsng. Licensed under Apache-2.0.
// Package jreast reads bounded facts from JR East's anonymous first-party pages.
package jreast

import "time"

const Origin = "https://traininfo.jreast.co.jp"

var JST = time.FixedZone("Asia/Tokyo", 9*60*60)

type Region struct {
	ID       string `json:"id"`
	NameJA   string `json:"name_ja"`
	NameEN   string `json:"name_en"`
	Path     string `json:"-"`
	SourceJA string `json:"source_ja"`
	SourceEN string `json:"source_en"`
}

func Regions() []Region {
	return []Region{
		{"kanto", "関東エリア", "Kanto Area", "kanto", Origin + "/train_info/kanto.aspx", Origin + "/train_info/e/kanto.aspx"},
		{"tohoku", "東北エリア", "Tohoku Area", "tohoku", Origin + "/train_info/tohoku.aspx", Origin + "/train_info/e/tohoku.aspx"},
		{"shinetsu", "信越エリア", "Shinetsu Area", "shinetsu", Origin + "/train_info/shinetsu.aspx", Origin + "/train_info/e/shinetsu.aspx"},
		{"express", "在来線特急等", "Express, Night train", "chyokyori", Origin + "/train_info/chyokyori.aspx", Origin + "/train_info/e/chyokyori.aspx"},
		{"shinkansen", "新幹線", "Shinkansen", "shinkansen", Origin + "/train_info/shinkansen.aspx", Origin + "/train_info/e/shinkansen.aspx"},
	}
}

type Coverage struct {
	Timezone           string `json:"timezone"`
	ReportingHours     string `json:"reporting_hours"`
	ReportingState     string `json:"reporting_state"`
	ServiceDay         string `json:"reporting_service_day"`
	EnglishThreshold   string `json:"english_delay_threshold"`
	JapaneseThreshold  string `json:"japanese_delay_threshold"`
	ThresholdConflict  bool   `json:"threshold_language_difference"`
	ActualDelayMinutes *int   `json:"actual_delay_minutes"`
	NormalMeaning      string `json:"normal_label_meaning"`
	BRT                string `json:"brt_coverage"`
	SourceURL          string `json:"source_url"`
}

type SourceState struct {
	URL            string `json:"url"`
	Language       string `json:"language"`
	ObservedAt     string `json:"observed_at"`
	UpdatedAt      string `json:"source_updated_at,omitempty"`
	Freshness      string `json:"freshness"`
	AgeSeconds     *int64 `json:"age_seconds"`
	ReportingState string `json:"reporting_state"`
	ScannedRows    int    `json:"scanned_rows"`
}

type Group struct {
	ID     string `json:"id"`
	NameJA string `json:"name_ja,omitempty"`
	NameEN string `json:"name_en,omitempty"`
}

type Section struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Language string `json:"language"`
}

type Notice struct {
	AffectedScope       string    `json:"affected_scope"`
	ResumeMinimumMonths *int      `json:"resumption_estimated_minimum_months"`
	ResumeEstimate      bool      `json:"resumption_is_estimate"`
	ServiceName         string    `json:"source_service_name,omitempty"`
	GroupID             string    `json:"source_group_id,omitempty"`
	Status              string    `json:"status"`
	Label               string    `json:"source_label"`
	Language            string    `json:"language"`
	Direction           string    `json:"direction"`
	Sections            []Section `json:"affected_sections"`
	Cause               string    `json:"cause,omitempty"`
	Planned             bool      `json:"planned_work"`
	Dates               []string  `json:"source_date_expressions"`
	Year                *int      `json:"calendar_year"`
	LocalStart          string    `json:"start_time_local,omitempty"`
	LocalEnd            string    `json:"end_time_local,omitempty"`
	Approximate         bool      `json:"time_approximate"`
	Replacement         string    `json:"replacement_transport"`
	SourceURL           string    `json:"source_url"`
}

type Line struct {
	IdentitySource     string   `json:"identity_source"`
	ID                 string   `json:"id"`
	SourceID           string   `json:"source_id"`
	Region             string   `json:"region"`
	NameJA             string   `json:"name_ja"`
	NameEN             string   `json:"name_en"`
	Code               string   `json:"line_code,omitempty"`
	Groups             []Group  `json:"source_groups"`
	Statuses           []string `json:"reported_statuses"`
	Assessment         string   `json:"assessment"`
	ActualDelayMinutes *int     `json:"actual_delay_minutes"`
	LanguageConflict   bool     `json:"status_language_conflict"`
	EnglishMapped      bool     `json:"english_identity_mapped"`
	NoticeFactCount    int      `json:"notice_fact_count"`
	Notices            []Notice `json:"notices"`
	SourceJA           string   `json:"source_ja"`
	SourceEN           string   `json:"source_en"`
}

type row struct {
	ID, Name, Code, GroupID, GroupName, Status, Label, Text, URL, Language string
}

type Page struct {
	Region Region
	State  SourceState
	rows   []row
}

type Snapshot struct {
	Region   Region        `json:"region"`
	Sources  []SourceState `json:"sources"`
	Lines    []Line        `json:"lines"`
	Warnings []string      `json:"warnings"`
}

type Area struct {
	Region
	Status string `json:"reported_status"`
	Label  string `json:"source_label"`
}

type Failure struct {
	Source string `json:"source"`
	Error  string `json:"error"`
}

type Meta struct {
	Source        string        `json:"source"`
	ObservedAt    string        `json:"observed_at"`
	Coverage      Coverage      `json:"coverage"`
	RequestCount  int           `json:"request_count"`
	MaxRequests   int           `json:"max_requests"`
	Sources       []SourceState `json:"sources"`
	FetchFailures []Failure     `json:"fetch_failures"`
	Note          string        `json:"note,omitempty"`
	Truncated     bool          `json:"truncated"`
}

type Envelope[T any] struct {
	Meta    Meta `json:"meta"`
	Results []T  `json:"results"`
}
