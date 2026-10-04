// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// Package trip extracts bounded factual evidence from public Iko-yo Trip pages.
package trip

import "time"

const BaseURL = "https://trip.iko-yo.net"
const MaxBody = 2 << 20
const ScopeNote = "Iko-yo Trip's selected family experiences and local events; the full core Iko-yo catalog is not integrated. Published facts are not live availability or admission guarantees."

type Evidence struct {
	ID       string `json:"id"`
	Location string `json:"location"`
	Text     string `json:"text"`
}
type Fact struct {
	Status      string   `json:"status"`
	EvidenceIDs []string `json:"evidence_ids"`
}
type Age struct {
	Status             string   `json:"status"`
	MinMonths          *int     `json:"min_months"`
	MaxExclusiveMonths *int     `json:"max_exclusive_months"`
	EvidenceIDs        []string `json:"evidence_ids"`
	Note               string   `json:"note"`
}
type Schedule struct {
	Raw       string `json:"published_text"`
	Start     string `json:"start_date"`
	End       string `json:"end_date"`
	Precision string `json:"precision"`
	Status    string `json:"status"`
	Note      string `json:"note"`
}
type Fees struct {
	PublishedText string `json:"published_text"`
	Child         string `json:"child_statement"`
	Adult         string `json:"adult_statement"`
	Currency      string `json:"currency"`
	Unit          string `json:"unit"`
	Note          string `json:"note"`
}
type Booking struct {
	Status           string   `json:"status"`
	ApplicationStart string   `json:"application_start"`
	ApplicationEnd   string   `json:"application_end"`
	CapacityPeople   *int     `json:"published_capacity_people"`
	Lottery          Fact     `json:"lottery"`
	EvidenceIDs      []string `json:"evidence_ids"`
	Availability     string   `json:"availability"`
}
type Coverage struct {
	Kind               string   `json:"kind"`
	Region             int      `json:"region"`
	Prefecture         int      `json:"prefecture"`
	SourceURLs         []string `json:"source_urls"`
	PagesScanned       int      `json:"pages_scanned"`
	RecordsScanned     int      `json:"records_scanned"`
	NextPage           int      `json:"next_page"`
	LastPage           int      `json:"last_page"`
	RemainingPages     *int     `json:"remaining_pages"`
	CompleteForListing bool     `json:"complete_for_listing"`
	SourceComplete     bool     `json:"source_complete"`
	ObservedAt         string   `json:"observed_at"`
}
type Record struct {
	ID          string          `json:"id"`
	Ref         string          `json:"ref"`
	Kind        string          `json:"kind"`
	Name        string          `json:"name"`
	Reading     string          `json:"reading"`
	Location    string          `json:"location"`
	Address     string          `json:"address"`
	Tags        []string        `json:"tags"`
	SourceURL   string          `json:"source_url"`
	OfficialURL string          `json:"official_url"`
	PublishedAt string          `json:"published_at"`
	UpdatedAt   string          `json:"updated_at"`
	ObservedAt  string          `json:"observed_at"`
	DataSource  string          `json:"data_source"`
	Detail      bool            `json:"detail"`
	Hours       string          `json:"published_hours"`
	ClosedDays  string          `json:"published_closed_days"`
	Access      string          `json:"published_access"`
	Schedule    Schedule        `json:"schedule"`
	Age         Age             `json:"age"`
	Amenities   map[string]Fact `json:"amenities"`
	Fees        Fees            `json:"fees"`
	Booking     Booking         `json:"booking"`
	Evidence    []Evidence      `json:"evidence"`
	Collection  Coverage        `json:"collection"`
}
type Page struct {
	Records  []Record       `json:"records"`
	Areas    map[int]string `json:"prefecture_links"`
	NextURL  string         `json:"next_url"`
	LastPage int            `json:"last_page"`
}
type Query struct {
	Kind      string
	Keyword   string
	From      string
	To        string
	AgeMonths int
	Amenities []string
	Limit     int
}
type Candidate struct {
	Record
	WindowMatch string `json:"window_match"`
}
type Discovery struct {
	Records                   []Candidate `json:"records"`
	Coverage                  []Coverage  `json:"coverage"`
	ScannedRecords            int         `json:"scanned_records"`
	MatchedRecords            int         `json:"matched_records"`
	UnknownDateRecords        int         `json:"unknown_date_records"`
	UnknownRequirementRecords int         `json:"unknown_requirement_records"`
	ExcludedRecords           int         `json:"excluded_records"`
	OmittedMatches            int         `json:"omitted_matches"`
	Note                      string      `json:"note"`
	Scope                     string      `json:"scope"`
}
type Check struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}
type Assessment struct {
	Record      Record           `json:"record"`
	Age         Check            `json:"age_check"`
	Amenities   map[string]Check `json:"amenity_checks"`
	Schedule    Check            `json:"schedule_check"`
	Application Check            `json:"application_check"`
	Overall     string           `json:"overall"`
}

func unknownFact() Fact { return Fact{Status: "unknown", EvidenceIDs: make([]string, 0)} }
func blankRecord(kind, id string, now time.Time) Record {
	a := map[string]Fact{}
	for _, name := range []string{"indoor", "nursing", "changing", "stroller"} {
		a[name] = unknownFact()
	}
	return Record{ID: id, Ref: kind + "/" + id, Kind: kind, SourceURL: BaseURL + "/" + kind + "/" + id, Tags: make([]string, 0), ObservedAt: now.UTC().Format(time.RFC3339), DataSource: "live", Amenities: a, Evidence: make([]Evidence, 0), Age: Age{Status: "unknown", EvidenceIDs: make([]string, 0), Note: "Published age description only; admission and safety are not verified."}, Schedule: Schedule{Precision: "unknown", Status: "unknown", Note: "Published dates do not establish individual operating days or live availability."}, Fees: Fees{Currency: "unknown", Unit: "unspecified", Note: "Preserve qualifiers; no family total or dated quote is inferred."}, Booking: Booking{Status: "unknown", Lottery: unknownFact(), EvidenceIDs: make([]string, 0), Availability: "unknown"}, Collection: Coverage{Kind: kind, SourceURLs: []string{BaseURL + "/" + kind + "/" + id}, RecordsScanned: 1, ObservedAt: now.UTC().Format(time.RFC3339)}}
}
