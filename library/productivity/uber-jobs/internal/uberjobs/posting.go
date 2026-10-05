// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package uberjobs

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
)

// Lenient scalar decoding: the site has only ever sent strings, but ids and
// numbers are decoded from either JSON shape until proven otherwise.

// LooseString decodes a JSON string, number, bool, or null into a string.
type LooseString string

func (s *LooseString) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		*s = ""
		return nil
	}
	if b[0] == '"' {
		var v string
		if err := json.Unmarshal(b, &v); err != nil {
			return err
		}
		*s = LooseString(v)
		return nil
	}
	*s = LooseString(strings.Trim(string(b), `"`))
	return nil
}

// LooseFloat decodes a JSON number or numeric string; null and "" stay unset.
type LooseFloat struct {
	Value float64
	Set   bool
}

func (f *LooseFloat) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		*f = LooseFloat{}
		return nil
	}
	raw := strings.Trim(string(b), `"`)
	if raw == "" {
		*f = LooseFloat{}
		return nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		*f = LooseFloat{}
		return nil
	}
	*f = LooseFloat{Value: v, Set: true}
	return nil
}

// RawLocation is one entry of the site's Locations[] array.
type RawLocation struct {
	Identifier    LooseString     `json:"Identifier"`
	City          LooseString     `json:"City"`
	Region        LooseString     `json:"Region"`
	Country       LooseString     `json:"Country"`
	CountryCode   LooseString     `json:"CountryCode"`
	Address       LooseString     `json:"Address"`
	PostalCode    LooseString     `json:"PostalCode"`
	LocationPoint json.RawMessage `json:"LocationPoint"`
}

// RawSalary is the site's Salary block.
type RawSalary struct {
	MinValue    LooseFloat  `json:"MinValue"`
	MaxValue    LooseFloat  `json:"MaxValue"`
	Currency    LooseString `json:"Currency"`
	Period      LooseString `json:"Period"`
	Description LooseString `json:"Description"`
}

// RawURL is one entry of the site's Urls[] array.
type RawURL struct {
	Culture   LooseString `json:"Culture"`
	URL       LooseString `json:"Url"`
	IsDefault bool        `json:"IsDefault"`
}

// RawPosting is one search row exactly as /api/jobs/search/ returns it.
type RawPosting struct {
	ID                     LooseString   `json:"Id"`
	Reference              LooseString   `json:"Reference"`
	Title                  LooseString   `json:"Title"`
	Description            LooseString   `json:"Description"`
	Summary                LooseString   `json:"Summary"`
	AdditionalText         LooseString   `json:"AdditionalText"`
	AdditionalDescription1 LooseString   `json:"AdditionalDescription1"`
	DisplayDate            LooseString   `json:"DisplayDate"`
	Teams                  []string      `json:"Teams"`
	ContractType           LooseString   `json:"ContractType"`
	WorkPattern            LooseString   `json:"WorkPattern"`
	ExperienceLevel        LooseString   `json:"ExperienceLevel"`
	Remote                 *bool         `json:"Remote"`
	Salary                 *RawSalary    `json:"Salary"`
	Locations              []RawLocation `json:"Locations"`
	URLs                   []RawURL      `json:"Urls"`
	Score                  LooseFloat    `json:"@search.score"`
}

// Location is one normalized location of a posting.
type Location struct {
	City        *string  `json:"city"`
	Region      *string  `json:"region"`
	Country     *string  `json:"country"`
	CountryCode *string  `json:"country_code"`
	Address     *string  `json:"address"`
	Lat         *float64 `json:"lat"`
	Lng         *float64 `json:"lng"`
}

// SalaryRange is one parsed pay-transparency range.
type SalaryRange struct {
	Location *string `json:"location"`
	Currency string  `json:"currency"`
	Min      float64 `json:"min"`
	Max      float64 `json:"max"`
	Period   string  `json:"period"`
}

// Posting is the tracker contract row. Fields the site does not carry are
// null, never invented; pointer fields have no omitempty on purpose.
type Posting struct {
	ID                      string        `json:"id"`
	Title                   string        `json:"title"`
	PostedDate              *string       `json:"posted_date"`
	PostedOn                *string       `json:"posted_on"`
	PostedRaw               *string       `json:"posted_raw"`
	PostedDateIsFloor       bool          `json:"posted_date_is_floor"`
	JobCategory             *string       `json:"job_category"`
	SubTeam                 *string       `json:"sub_team"`
	CountryCode             *string       `json:"country_code"`
	Country                 *string       `json:"country"`
	Location                *string       `json:"location"`
	NormalizedLocation      *string       `json:"normalized_location"`
	Locations               []Location    `json:"locations"`
	BasicQualifications     *string       `json:"basic_qualifications"`
	PreferredQualifications *string       `json:"preferred_qualifications"`
	Description             *string       `json:"description"`
	Summary                 *string       `json:"summary"`
	ContractType            *string       `json:"contract_type"`
	WorkPattern             *string       `json:"work_pattern"`
	ExperienceLevel         *string       `json:"experience_level"`
	Remote                  *bool         `json:"remote"`
	IsIntern                *bool         `json:"is_intern"`
	UniversityJob           *bool         `json:"university_job"`
	SalaryText              *string       `json:"salary_text"`
	SalaryMin               *float64      `json:"salary_min"`
	SalaryMax               *float64      `json:"salary_max"`
	SalaryCurrency          *string       `json:"salary_currency"`
	SalaryPeriod            *string       `json:"salary_period"`
	SalaryRanges            []SalaryRange `json:"salary_ranges"`
	JobPath                 string        `json:"job_path"`
	URL                     string        `json:"url"`
	Employer                string        `json:"employer"`
	Source                  string        `json:"source"`
	Score                   *float64      `json:"score"`
}

// ContractFields lists every tracker-contract key so --agent/--compact keep them.
var ContractFields = []string{
	"id", "title", "posted_date", "posted_on", "posted_raw", "posted_date_is_floor",
	"job_category", "sub_team", "country_code", "country", "location", "normalized_location",
	"locations", "basic_qualifications", "preferred_qualifications", "description", "summary",
	"contract_type", "work_pattern", "experience_level", "remote", "is_intern", "university_job",
	"salary_text", "salary_min", "salary_max", "salary_currency", "salary_period", "salary_ranges",
	"job_path", "url", "employer", "source", "score",
}

func strPtr(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func boolPtr(b bool) *bool { return &b }

// Normalize turns one raw site row into the tracker contract row.
func Normalize(raw RawPosting, baseURL string) Posting {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	id := strings.TrimSpace(string(raw.ID))
	if id == "" {
		id = strings.TrimSpace(string(raw.Reference))
	}
	p := Posting{
		ID:       id,
		Title:    strings.TrimSpace(string(raw.Title)),
		Employer: "uber",
		Source:   SourceSite,
	}
	p.PostedRaw = strPtr(string(raw.DisplayDate))
	p.PostedOn, p.PostedDate, p.PostedDateIsFloor = postingDates(string(raw.DisplayDate))

	team := ""
	if len(raw.Teams) > 0 {
		team = strings.TrimSpace(raw.Teams[0])
	}
	p.JobCategory = strPtr(team)
	p.SubTeam = subTeamFrom(team, string(raw.AdditionalText))

	for _, l := range raw.Locations {
		loc := Location{
			City:    strPtr(string(l.City)),
			Region:  strPtr(string(l.Region)),
			Country: strPtr(string(l.Country)),
			Address: strPtr(string(l.Address)),
		}
		if code := strings.TrimSpace(string(l.CountryCode)); code != "" {
			loc.CountryCode = strPtr(ToISO3(code))
		}
		if loc.CountryCode == nil && loc.Country != nil {
			if iso, ok := CountryNameToISO3(*loc.Country); ok {
				loc.CountryCode = &iso
			}
		}
		loc.Lat, loc.Lng = pointLatLng(l.LocationPoint)
		p.Locations = append(p.Locations, loc)
	}
	if p.Locations == nil {
		p.Locations = []Location{}
	}
	if len(p.Locations) > 0 {
		primary := p.Locations[0]
		p.CountryCode = primary.CountryCode
		p.Country = primary.Country
		p.NormalizedLocation = joinLocation(primary)
		if primary.Address != nil {
			p.Location = primary.Address
		} else {
			p.Location = p.NormalizedLocation
		}
	}

	desc := StripHTML(string(raw.Description))
	p.Description = strPtr(desc)
	p.Summary = strPtr(StripHTML(string(raw.Summary)))
	p.ContractType = strPtr(string(raw.ContractType))
	p.WorkPattern = strPtr(string(raw.WorkPattern))
	p.ExperienceLevel = strPtr(string(raw.ExperienceLevel))
	p.Remote = raw.Remote
	p.IsIntern, p.UniversityJob = internFlags(string(raw.WorkPattern), raw.Teams)

	if raw.Salary != nil {
		text := StripHTML(string(raw.Salary.Description))
		p.SalaryText = strPtr(text)
		p.SalaryRanges = ParseSalaryRanges(text)
		if raw.Salary.MinValue.Set {
			v := raw.Salary.MinValue.Value
			p.SalaryMin = &v
		}
		if raw.Salary.MaxValue.Set {
			v := raw.Salary.MaxValue.Value
			p.SalaryMax = &v
		}
		p.SalaryCurrency = strPtr(string(raw.Salary.Currency))
		p.SalaryPeriod = strPtr(string(raw.Salary.Period))
		if p.SalaryMin == nil && p.SalaryMax == nil {
			applySingleRange(&p)
		}
	}
	if p.SalaryRanges == nil {
		p.SalaryRanges = []SalaryRange{}
	}

	p.JobPath = jobPath(raw, id)
	p.URL = strings.TrimRight(baseURL, "/") + p.JobPath
	if raw.Score.Set {
		v := raw.Score.Value
		p.Score = &v
	}
	return p
}

// subTeamFrom derives the subteam from AdditionalText ("<Team> <SubTeam>").
func subTeamFrom(team, additional string) *string {
	additional = strings.TrimSpace(additional)
	if team == "" || additional == "" {
		return nil
	}
	if strings.HasPrefix(additional, team+" ") {
		return strPtr(strings.TrimPrefix(additional, team+" "))
	}
	return nil
}

// internFlags maps the site's own WorkPattern and Teams values. An empty
// WorkPattern means the site did not say, so both stay null.
func internFlags(workPattern string, teams []string) (*bool, *bool) {
	wp := strings.TrimSpace(workPattern)
	university := false
	for _, t := range teams {
		if strings.EqualFold(strings.TrimSpace(t), "University") {
			university = true
		}
	}
	if wp == "" && !university {
		return nil, nil
	}
	intern := strings.EqualFold(wp, "Intern")
	ncg := strings.EqualFold(wp, "Direct NCG Hire")
	return boolPtr(intern), boolPtr(university || ncg)
}

func jobPath(raw RawPosting, id string) string {
	for _, u := range raw.URLs {
		if u.IsDefault && strings.TrimSpace(string(u.URL)) != "" {
			return ensureTrailingSlash(string(u.URL))
		}
	}
	for _, u := range raw.URLs {
		if strings.TrimSpace(string(u.URL)) != "" {
			return ensureTrailingSlash(string(u.URL))
		}
	}
	return "/en/jobs/" + id + "/"
}

func ensureTrailingSlash(p string) string {
	p = strings.TrimSpace(p)
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return p
}

func joinLocation(l Location) *string {
	parts := make([]string, 0, 3)
	for _, v := range []*string{l.City, l.Region, l.Country} {
		if v != nil && *v != "" {
			if len(parts) > 0 && parts[len(parts)-1] == *v {
				continue
			}
			parts = append(parts, *v)
		}
	}
	if len(parts) == 0 {
		return nil
	}
	s := strings.Join(parts, ", ")
	return &s
}

func pointLatLng(raw json.RawMessage) (*float64, *float64) {
	if len(raw) == 0 {
		return nil, nil
	}
	var pt struct {
		Coordinates []float64 `json:"coordinates"`
	}
	if json.Unmarshal(raw, &pt) != nil || len(pt.Coordinates) < 2 {
		return nil, nil
	}
	lng, lat := pt.Coordinates[0], pt.Coordinates[1]
	return &lat, &lng
}
