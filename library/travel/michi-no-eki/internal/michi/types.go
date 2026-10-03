// Copyright 2026 zjsng. Licensed under Apache-2.0.
package michi

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

const Origin = "https://www.michi-no-eki.jp"
const PolicyURL = "https://www.mlit.go.jp/road/soudan/soudan_03_04.html"

var jst = time.FixedZone("JST", 9*3600)

func observedNow() string { return time.Now().In(jst).Format(time.RFC3339) }

//go:embed catalog.json
var catalogJSON []byte

type CatalogEntry struct {
	ID         string `json:"id,omitempty"`
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	Region     string `json:"region,omitempty"`
	RegionName string `json:"region_name,omitempty"`
}
type Catalog struct {
	Prefectures []CatalogEntry `json:"prefectures"`
	Facilities  []CatalogEntry `json:"facilities"`
	Regions     []CatalogEntry `json:"regions"`
	SourceURL   string         `json:"source_url"`
	ObservedAt  string         `json:"observed_at"`
}

func CatalogData() Catalog {
	var c Catalog
	if err := json.Unmarshal(catalogJSON, &c); err != nil {
		panic(err)
	}
	return c
}

type Coordinates struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}
type Parking struct {
	Raw          string `json:"source_text"`
	Large        *int   `json:"large_vehicles"`
	Cars         *int   `json:"ordinary_cars"`
	Accessible   *int   `json:"accessible_spaces"`
	Availability string `json:"live_space_availability"`
	VehicleFit   string `json:"vehicle_fit"`
}
type Permissions struct {
	Overnight  string `json:"overnight_lodging"`
	Camping    string `json:"outdoor_camping"`
	Designated string `json:"designated_overnight_space"`
}
type Station struct {
	ID                  string            `json:"id"`
	Name                string            `json:"name"`
	Prefecture          string            `json:"prefecture"`
	Municipality        string            `json:"municipality"`
	URL                 string            `json:"url"`
	SourceURL           string            `json:"source_url"`
	ObservedAt          string            `json:"observed_at"`
	Address             string            `json:"address"`
	Phone               string            `json:"phone"`
	PublishedHours      string            `json:"published_hours"`
	Registration        string            `json:"registration_source_text"`
	MapCode             string            `json:"map_code"`
	Coordinates         *Coordinates      `json:"coordinates"`
	DistanceKM          *float64          `json:"distance_km,omitempty"`
	Facilities          map[string]string `json:"facilities"`
	Parking             Parking           `json:"parking"`
	OperatorURLs        []string          `json:"operator_urls"`
	Permissions         Permissions       `json:"permissions"`
	LiveOpening         string            `json:"live_opening"`
	ChargerAvailability string            `json:"live_charger_availability"`
	Fees                string            `json:"fees"`
}

func emptyStation(id, url, at string) Station {
	f := map[string]string{}
	for _, x := range CatalogData().Facilities {
		f[x.Slug] = "unknown"
	}
	return Station{ID: id, URL: Origin + "/stations/views/" + id, SourceURL: url, ObservedAt: at, Facilities: f, OperatorURLs: []string{}, Permissions: Permissions{"unknown", "unknown", "unknown"}, LiveOpening: "unknown", ChargerAvailability: "unknown", Fees: "unknown", Parking: Parking{Availability: "unknown", VehicleFit: "unknown"}}
}

type Query struct {
	Prefecture    string
	Region        string
	Facility      string
	Keyword       string
	Match         string
	Limit         int
	MaxCandidates int
}
type Scope struct {
	PrefectureIDs       []string `json:"prefecture_ids"`
	FacilityIDs         []string `json:"facility_ids"`
	Keyword             string   `json:"keyword"`
	FacilityMatch       string   `json:"facility_match"`
	NativeFacilityMatch string   `json:"native_facility_match"`
}
type Search struct {
	EmptyResultEvidence string    `json:"empty_result_evidence,omitempty"`
	Stations            []Station `json:"stations"`
	SourceURL           string    `json:"source_url"`
	ObservedAt          string    `json:"observed_at"`
	SourceTotal         *int      `json:"source_reported_total"`
	SourceCards         int       `json:"source_cards"`
	ScannedCandidates   int       `json:"scanned_candidates"`
	MatchedCount        int       `json:"matched_count"`
	ReturnedCount       int       `json:"returned_count"`
	Limit               int       `json:"limit"`
	MaxCandidates       int       `json:"max_candidates"`
	CoverageComplete    bool      `json:"candidate_coverage_complete"`
	Scope               Scope     `json:"scope"`
	Note                string    `json:"note,omitempty"`
	Warnings            []string  `json:"warnings"`
}

var evidenceWarnings = []string{"Facility presence is source-listed evidence; current opening, fees, charger availability and vehicle fit are unknown.", "A roadside-station listing does not establish overnight lodging or outdoor camping permission; confirm station-specific designated spaces and rules."}

type FetchFailure struct {
	ID        string `json:"id"`
	SourceURL string `json:"source_url"`
	Error     string `json:"error"`
}
type Comparison struct {
	SchemaVersion   int            `json:"schema_version"`
	Kind            string         `json:"kind"`
	ObservedAt      string         `json:"observed_at"`
	RequestedIDs    []string       `json:"requested_ids"`
	Stations        []Station      `json:"stations"`
	FetchFailures   []FetchFailure `json:"fetch_failures"`
	RequestedCount  int            `json:"requested_count"`
	SuccessfulCount int            `json:"successful_count"`
	Warnings        []string       `json:"warnings"`
}
type Notice struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	Prefecture       string   `json:"prefecture"`
	PublishedDate    string   `json:"published_date"`
	Timezone         string   `json:"date_timezone"`
	URL              string   `json:"url"`
	SourceURL        string   `json:"source_url"`
	ObservedAt       string   `json:"observed_at"`
	StationIDs       []string `json:"station_ids"`
	Excerpt          string   `json:"excerpt,omitempty"`
	ExcerptTruncated bool     `json:"excerpt_truncated,omitempty"`
	MatchReason      string   `json:"match_reason,omitempty"`
}
type Notices struct {
	Notices          []Notice       `json:"notices"`
	SourceURLs       []string       `json:"source_urls"`
	ObservedAt       string         `json:"observed_at"`
	ScannedPages     int            `json:"scanned_pages"`
	ScannedRecords   int            `json:"scanned_records"`
	DetailRecords    int            `json:"detail_records"`
	MaxScanPages     int            `json:"max_scan_pages"`
	MaxDetailRecords int            `json:"max_detail_records"`
	ReturnedCount    int            `json:"returned_count"`
	NextPage         *int           `json:"next_page"`
	PublicationStart string         `json:"oldest_scanned_publication_date"`
	PublicationEnd   string         `json:"newest_scanned_publication_date"`
	StationID        string         `json:"station_id,omitempty"`
	MatchCoverage    string         `json:"match_coverage"`
	FetchFailures    []FetchFailure `json:"fetch_failures"`
	Note             string         `json:"note"`
}

func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimSuffix(s, "県")
	s = strings.TrimSuffix(s, "府")
	s = strings.TrimSuffix(s, "都")
	return strings.ReplaceAll(s, " ", "-")
}
func resolveCSV(value string, entries []CatalogEntry) ([]string, error) {
	out := []string{}
	seen := map[string]bool{}
	if strings.TrimSpace(value) == "" {
		return out, nil
	}
	for _, token := range strings.Split(value, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			return nil, fmt.Errorf("empty filter value; use a comma-separated catalog slug, Japanese label or ID")
		}
		found := ""
		for _, x := range entries {
			if token == x.ID || normalize(token) == normalize(x.Slug) || normalize(token) == normalize(x.Name) {
				found = x.ID
				break
			}
		}
		if found == "" {
			return nil, fmt.Errorf("unknown filter %q; run michi-no-eki-pp-cli catalog", token)
		}
		if !seen[found] {
			seen[found] = true
			out = append(out, found)
		}
	}
	sort.Slice(out, func(i, j int) bool { a, _ := strconv.Atoi(out[i]); b, _ := strconv.Atoi(out[j]); return a < b })
	return out, nil
}
func resolveQuery(q Query) (Scope, error) {
	c := CatalogData()
	if q.Prefecture != "" && q.Region != "" {
		return Scope{}, fmt.Errorf("choose --prefecture or --region, not both")
	}
	p, e := resolveCSV(q.Prefecture, c.Prefectures)
	if e != nil {
		return Scope{}, e
	}
	if q.Region != "" {
		seen := map[string]bool{}
		for _, token := range strings.Split(q.Region, ",") {
			found := ""
			for _, r := range c.Regions {
				if normalize(token) == normalize(r.Slug) || strings.TrimSpace(token) == r.Name {
					found = r.Slug
				}
			}
			if found == "" {
				return Scope{}, fmt.Errorf("unknown --region %q; run catalog", token)
			}
			for _, x := range c.Prefectures {
				if x.Region == found && !seen[x.ID] {
					seen[x.ID] = true
					p = append(p, x.ID)
				}
			}
		}
		sort.Slice(p, func(i, j int) bool { a, _ := strconv.Atoi(p[i]); b, _ := strconv.Atoi(p[j]); return a < b })
	}
	f, e := resolveCSV(q.Facility, c.Facilities)
	if e != nil {
		return Scope{}, e
	}
	if q.Match != "all" && q.Match != "any" {
		return Scope{}, fmt.Errorf("--match must be all or any")
	}
	if q.Limit < 1 || q.Limit > 50 {
		return Scope{}, fmt.Errorf("--limit must be 1–50")
	}
	if q.MaxCandidates < 1 || q.MaxCandidates > 5000 {
		return Scope{}, fmt.Errorf("--max-candidates must be 1–5000")
	}
	if len([]rune(q.Keyword)) > 120 {
		return Scope{}, fmt.Errorf("--keyword must be at most 120 characters")
	}
	if strings.EqualFold(strings.TrimSpace(q.Keyword), "all") {
		return Scope{}, fmt.Errorf("--keyword all is the provider's reserved empty-query token; omit --keyword to search all")
	}
	return Scope{p, f, strings.TrimSpace(q.Keyword), q.Match, "any"}, nil
}
func validateID(id string) error {
	if len(id) < 1 || len(id) > 12 {
		return fmt.Errorf("station/notice ID must be 1–12 digits")
	}
	for _, r := range id {
		if r < '0' || r > '9' {
			return fmt.Errorf("invalid numeric station/notice ID %q", id)
		}
	}
	return nil
}
func idsFromCSV(csv string, max int) ([]string, error) {
	if strings.TrimSpace(csv) == "" {
		return nil, fmt.Errorf("--ids is required; for example --ids 19187,19189")
	}
	out := []string{}
	seen := map[string]bool{}
	for _, s := range strings.Split(csv, ",") {
		s = strings.TrimSpace(s)
		if e := validateID(s); e != nil {
			return nil, e
		}
		if seen[s] {
			return nil, fmt.Errorf("duplicate station ID %s", s)
		}
		seen[s] = true
		out = append(out, s)
	}
	if len(out) > max {
		return nil, fmt.Errorf("at most %d station IDs are supported", max)
	}
	return out, nil
}

// ValidateQuery checks public filter values before any network requests.
func ValidateQuery(q Query) error { _, err := resolveQuery(q); return err }

// ValidateIDs checks a bounded ordered station-ID list.
func ValidateIDs(csv string) error { _, err := idsFromCSV(csv, 6); return err }
