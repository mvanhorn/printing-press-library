// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// Package hgj preserves source evidence without making dietary or access assurances.
package hgj

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

const Origin = "https://halalgourmet.jp"
const Restaurant = "restaurant"
const Prayer = "prayer"
const Reported = "reported"
const NotReported = "not_reported"
const ReportedNegative = "reported_negative"
const Inapplicable = "inapplicable"
const CardUnknown = "not_visible_on_card"

var foodKeys = []string{"certified", "owner", "noAlcoholicDrinks", "porkFree", "meat", "seasoning", "tableware", "meal", "vegetarian", "prayer"}
var prayerKeys = []string{"wudu", "wifi", "hotWater", "qibla"}
var labels = map[string]string{"certified": "Halal Certified", "owner": "Muslim Owner", "noAlcoholicDrinks": "No Alcoholic Drinks", "porkFree": "Pork Free", "meat": "Halal Meat", "seasoning": "Halal Seasoning", "tableware": "Halal Kitchen & Tableware", "meal": "Halal Meal", "vegetarian": "Vegetarian Meal", "prayer": "Prayer Space", "wudu": "Wudu", "wifi": "Wi-Fi", "hotWater": "Hot Water", "qibla": "Qibla"}
var numericID = regexp.MustCompile(`^[1-9][0-9]{0,11}$`)
var monthRE = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

// Condition is a source-reported label. A missing label is never an explicit negative.
type Condition struct {
	Label      string `json:"label"`
	State      string `json:"state"`
	SourceURL  string `json:"source_url"`
	Provenance string `json:"provenance"`
}
type Fact struct {
	State     string `json:"state"`
	Value     string `json:"value,omitempty"`
	SourceURL string `json:"source_url"`
}
type Certification struct {
	LabelState string `json:"label_state"`
	Certifier  Fact   `json:"certifier"`
	ValidUntil Fact   `json:"valid_until"`
}
type Verification struct {
	State     string `json:"state"`
	Label     string `json:"label,omitempty"`
	Month     string `json:"month,omitempty"`
	SourceURL string `json:"source_url"`
}
type Coordinates struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}
type Hours struct {
	Day        string `json:"day"`
	SourceText string `json:"source_text"`
}
type Place struct {
	ID                   string               `json:"id"`
	Kind                 string               `json:"kind"`
	Name                 string               `json:"name"`
	NameJapanese         string               `json:"name_japanese,omitempty"`
	Prefecture           string               `json:"prefecture,omitempty"`
	Category             string               `json:"category,omitempty"`
	PrayerType           string               `json:"prayer_type,omitempty"`
	SourceURL            string               `json:"source_url"`
	ObservedAt           string               `json:"observed_at"`
	EvidenceScope        string               `json:"evidence_scope"`
	HiddenConditions     bool                 `json:"hidden_conditions"`
	Conditions           map[string]Condition `json:"conditions"`
	Certification        Certification        `json:"certification"`
	Verification         Verification         `json:"verification"`
	Address              string               `json:"address,omitempty"`
	Coordinates          *Coordinates         `json:"coordinates,omitempty"`
	WeeklyHours          []Hours              `json:"weekly_hours"`
	HoursNotes           []string             `json:"hours_notes"`
	HoursState           string               `json:"hours_state"`
	AccessNotes          []string             `json:"access_notes"`
	AccessState          string               `json:"access_state"`
	AccessNotesTruncated bool                 `json:"access_notes_truncated"`
	PriceRange           string               `json:"price_range,omitempty"`
	PriceScope           string               `json:"price_scope,omitempty"`
}

type SearchResult struct {
	Results       []Place `json:"results"`
	SourceURL     string  `json:"source_url"`
	ObservedAt    string  `json:"observed_at"`
	SourceTotal   int     `json:"source_total"`
	ParsedCount   int     `json:"parsed_count"`
	ReturnedCount int     `json:"returned_count"`
	Truncated     bool    `json:"truncated"`
	Note          string  `json:"note"`
}

func CanonicalURL(kind, id string) (string, error) {
	if kind != Restaurant && kind != Prayer {
		return "", fmt.Errorf("kind must be restaurant or prayer")
	}
	if !numericID.MatchString(id) {
		return "", fmt.Errorf("source ID must contain 1–12 digits and cannot start with zero")
	}
	path := "/restaurant/"
	if kind == Prayer {
		path = "/pray/"
	}
	return Origin + path + id, nil
}
func ConditionKeys(kind string) []string {
	if kind == Prayer {
		return append([]string{}, prayerKeys...)
	}
	return append([]string{}, foodKeys...)
}
func NormalizeCondition(key string) (string, error) {
	for k, l := range labels {
		if strings.EqualFold(key, k) || strings.EqualFold(key, l) || strings.EqualFold(strings.ReplaceAll(key, "-", ""), strings.ReplaceAll(k, "-", "")) {
			return k, nil
		}
	}
	return "", fmt.Errorf("unknown condition %q; use certified, owner, noAlcoholicDrinks, porkFree, meat, seasoning, tableware, meal, vegetarian, prayer, wudu, wifi, hotWater or qibla", key)
}
func emptyPlace(kind, id, scope, observed string) Place {
	url, _ := CanonicalURL(kind, id)
	p := Place{ID: id, Kind: kind, SourceURL: url, ObservedAt: observed, EvidenceScope: scope, Conditions: map[string]Condition{}, WeeklyHours: []Hours{}, HoursNotes: []string{}, AccessNotes: []string{}, HoursState: NotReported, AccessState: NotReported, Verification: Verification{State: NotReported, SourceURL: url}}
	applicable := map[string]bool{}
	for _, k := range ConditionKeys(kind) {
		applicable[k] = true
	}
	for k, l := range labels {
		state := Inapplicable
		if applicable[k] {
			state = NotReported
			if scope == "card" {
				state = CardUnknown
			}
		}
		p.Conditions[k] = Condition{Label: l, State: state, SourceURL: url, Provenance: "HGJ listing condition"}
	}
	p.Certification = Certification{LabelState: p.Conditions["certified"].State, Certifier: Fact{State: NotReported, SourceURL: url}, ValidUntil: Fact{State: NotReported, SourceURL: url}}
	if kind == Prayer {
		p.Certification.Certifier.State = Inapplicable
		p.Certification.ValidUntil.State = Inapplicable
	}
	return p
}
func setReported(p *Place, label string) {
	key, err := NormalizeCondition(label)
	if err != nil {
		return
	}
	c := p.Conditions[key]
	if c.State == Inapplicable {
		return
	}
	c.State = Reported
	p.Conditions[key] = c
	p.Certification.LabelState = p.Conditions["certified"].State
}
func observedNow() string { return time.Now().UTC().Format(time.RFC3339Nano) }
