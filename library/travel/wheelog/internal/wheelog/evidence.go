// Copyright 2026 Jet Sng and contributors. Licensed under Apache-2.0.
package wheelog

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const MaxSpots = 50
const MaxDetailReads = 5

type Category struct {
	Value       string `json:"value"`
	Name        string `json:"name"`
	QuestionIDs []int  `json:"question_ids"`
}

func Categories() []Category {
	return []Category{
		{"shop", "Restaurant", ids(401, 409)}, {"station", "Station", ids(601, 605)},
		{"hotel", "Hotel", ids(701, 710)}, {"other", "Other", ids(501, 505)},
		{"favorite", "Attraction", ids(1001, 1008)}, {"toilet", "Restroom", ids(101, 109)},
		{"elevator", "Elevator", ids(201, 203)}, {"parking", "Parking", ids(301, 308)},
		{"slope", "Ramp", []int{}}, {"barrierSpot", "Barrier", []int{}},
	}
}

func ids(first, last int) []int {
	out := make([]int, 0, last-first+1)
	for n := first; n <= last; n++ {
		out = append(out, n)
	}
	return out
}

func QuestionCategory(id int) (string, bool) {
	for _, category := range Categories() {
		for _, qid := range category.QuestionIDs {
			if qid == id {
				return category.Value, true
			}
		}
	}
	return "", false
}

func ValidCategory(value string) bool {
	for _, category := range Categories() {
		if category.Value == value {
			return true
		}
	}
	return false
}

type Count int

func (n *Count) UnmarshalJSON(data []byte) error {
	value := strings.Trim(string(data), `"`)
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil || parsed < 0 || parsed > 1000000 {
		return fmt.Errorf("invalid aggregate report count")
	}
	*n = Count(parsed)
	return nil
}

type Scalar string

func (s *Scalar) UnmarshalJSON(data []byte) error {
	var value string
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
	} else {
		value = string(data)
	}
	*s = Scalar(value)
	return nil
}

type RawLocation struct {
	Lat Scalar `json:"lat"`
	Lng Scalar `json:"lng"`
}

func (location *RawLocation) UnmarshalJSON(data []byte) error {
	// Timeline cards use an empty string when their coordinates are omitted.
	if len(data) == 0 || data[0] != '{' {
		return nil
	}
	type locationFields RawLocation
	return json.Unmarshal(data, (*locationFields)(location))
}

type RawQuestion struct {
	ID    int    `json:"id"`
	Label string `json:"question"`
	Good  *Count `json:"totalGood"`
	Bad   *Count `json:"totalBad"`
}

type RawCategory struct {
	ID        int           `json:"id"`
	Category  string        `json:"category"`
	Questions []RawQuestion `json:"questionList"`
}

type CategoryField struct {
	Name   string
	Detail *RawCategory
}

func (c *CategoryField) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		return json.Unmarshal(data, &c.Name)
	}
	var detail RawCategory
	if err := json.Unmarshal(data, &detail); err != nil {
		return err
	}
	c.Name = detail.Category
	c.Detail = &detail
	return nil
}
func (c CategoryField) MarshalJSON() ([]byte, error) {
	if c.Detail != nil {
		return json.Marshal(c.Detail)
	}
	return json.Marshal(c.Name)
}

// RawSpot deliberately has no contributor, narrative, photo or comment fields.
type RawSpot struct {
	ID       int64         `json:"id"`
	Name     string        `json:"name"`
	Address  string        `json:"address"`
	Country  string        `json:"country"`
	Created  string        `json:"created"`
	Updated  string        `json:"updated"`
	Location *RawLocation  `json:"location"`
	Category CategoryField `json:"spotCategory"`
}

type QueryEcho struct {
	Word       string   `json:"word"`
	Categories []string `json:"categoryList"`
	Questions  []string `json:"questionList"`
	From       *string  `json:"startDatetime"`
	To         *string  `json:"endDatetime"`
	Type       string   `json:"type"`
	Page       Scalar   `json:"pagenum"`
	Detail     Scalar   `json:"isDetail"`
}

type RawTimeline struct {
	Type string  `json:"type"`
	Spot RawSpot `json:"timeline"`
}
type Content struct {
	Spot     *RawSpot      `json:"spot,omitempty"`
	Timeline []RawTimeline `json:"timelineList,omitempty"`
	Request  *QueryEcho    `json:"request,omitempty"`
}
type Envelope struct {
	Result []struct {
		Code json.RawMessage `json:"resultCode"`
	} `json:"result"`
	Content *Content `json:"content"`
}

// SanitizeEnvelope is applied before the generated client can cache or print a body.
func SanitizeEnvelope(data []byte, detail bool) ([]byte, error) {
	var envelope Envelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("WheeLog source JSON contract: %w", err)
	}
	if len(envelope.Result) == 0 {
		return nil, fmt.Errorf("WheeLog response lacks a semantic result")
	}
	for _, result := range envelope.Result {
		if strings.Trim(string(result.Code), `"`) != "0" {
			return nil, fmt.Errorf("WheeLog returned a source semantic error")
		}
	}
	if envelope.Content == nil {
		return nil, fmt.Errorf("WheeLog response lacks spot content")
	}
	if detail {
		if envelope.Content.Spot == nil {
			return nil, fmt.Errorf("WheeLog spot detail is unavailable")
		}
		if err := validateRawSpot(*envelope.Content.Spot); err != nil {
			return nil, err
		}
		envelope.Content = &Content{Spot: envelope.Content.Spot}
	} else {
		if envelope.Content.Request == nil {
			return nil, fmt.Errorf("WheeLog search lacks its request echo")
		}
		if len(envelope.Content.Timeline) > 100 {
			return nil, fmt.Errorf("WheeLog returned more than 100 records in a page")
		}
		timeline := make([]RawTimeline, 0, len(envelope.Content.Timeline))
		for _, item := range envelope.Content.Timeline {
			if item.Type == "spot" {
				if err := validateRawSpot(item.Spot); err != nil {
					return nil, err
				}
				timeline = append(timeline, item)
			}
		}
		envelope.Content = &Content{Timeline: timeline, Request: envelope.Content.Request}
	}
	content := map[string]any{"spot": envelope.Content.Spot}
	if !detail {
		content = map[string]any{"timelineList": envelope.Content.Timeline, "request": envelope.Content.Request}
	}
	return json.Marshal(map[string]any{"result": []map[string]any{{"resultCode": 0, "message": ""}}, "content": content})
}

func validateRawSpot(spot RawSpot) error {
	if spot.ID <= 0 || spot.ID > 1000000000 || strings.TrimSpace(spot.Name) == "" {
		return fmt.Errorf("WheeLog returned an invalid public spot identity")
	}
	if !ValidCategory(spot.Category.Name) {
		return fmt.Errorf("WheeLog returned an unsupported source category")
	}
	if spot.Category.Detail != nil && len(spot.Category.Detail.Questions) > 30 {
		return fmt.Errorf("WheeLog returned too many questions for a spot")
	}
	return nil
}

type Coordinate struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}
type Question struct {
	ID         int     `json:"id"`
	Label      string  `json:"label"`
	Positive   *int    `json:"positive_reports"`
	Negative   *int    `json:"negative_reports"`
	State      string  `json:"state"`
	ObservedAt *string `json:"report_observed_at"`
}
type Spot struct {
	ID           int64       `json:"id"`
	Name         string      `json:"name"`
	Address      string      `json:"address"`
	Country      *string     `json:"country"`
	Category     string      `json:"category"`
	Location     *Coordinate `json:"location"`
	Created      *string     `json:"record_created_at"`
	Updated      *string     `json:"record_updated_at"`
	CreatedRaw   *string     `json:"source_created_raw"`
	UpdatedRaw   *string     `json:"source_updated_raw"`
	ObservedAt   string      `json:"retrieved_at"`
	SourceURL    string      `json:"source_url"`
	Questions    []Question  `json:"questions"`
	DetailStatus string      `json:"detail_status"`
	Source       string      `json:"data_source"`
	Gaps         []string    `json:"unsupported_facts"`
}

func Normalize(spot RawSpot, observed time.Time) (Spot, error) {
	if err := validateRawSpot(spot); err != nil {
		return Spot{}, err
	}
	result := Spot{ID: spot.ID, Name: clean(spot.Name, 512), Address: clean(spot.Address, 1024), Category: spot.Category.Name,
		Created: sourceTime(spot.Created), Updated: sourceTime(spot.Updated), CreatedRaw: nonempty(spot.Created), UpdatedRaw: nonempty(spot.Updated),
		Country: nonempty(clean(spot.Country, 128)), ObservedAt: observed.UTC().Format(time.RFC3339Nano),
		SourceURL: "https://app.wheelog.com/?spotId=" + strconv.FormatInt(spot.ID, 10) + "&la=ja", Questions: make([]Question, 0),
		Source: "live", DetailStatus: "not_expanded", Gaps: []string{"measured_dimensions", "individual_report_dates", "current_open_status", "accessible_routes"}}
	if spot.Location != nil {
		lat, e1 := strconv.ParseFloat(string(spot.Location.Lat), 64)
		lon, e2 := strconv.ParseFloat(string(spot.Location.Lng), 64)
		if e1 == nil && e2 == nil && ValidCoordinate(lat, lon) {
			result.Location = &Coordinate{lat, lon}
		}
	}
	if spot.Category.Detail != nil {
		result.DetailStatus = "checked"
		seen := map[int]bool{}
		for _, raw := range spot.Category.Detail.Questions {
			if seen[raw.ID] {
				return Spot{}, fmt.Errorf("WheeLog repeated a question ID")
			}
			seen[raw.ID] = true
			category, known := QuestionCategory(raw.ID)
			if !known || category != spot.Category.Name {
				return Spot{}, fmt.Errorf("WheeLog question ID/category contract changed")
			}
			question := Question{ID: raw.ID, Label: clean(raw.Label, 512), Positive: countPointer(raw.Good), Negative: countPointer(raw.Bad)}
			question.State = ReportState(question.Positive, question.Negative)
			result.Questions = append(result.Questions, question)
		}
		sort.Slice(result.Questions, func(i, j int) bool { return result.Questions[i].ID < result.Questions[j].ID })
	}
	return result, nil
}

func clean(value string, max int) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, value)
	runes := []rune(strings.TrimSpace(value))
	if len(runes) > max {
		runes = runes[:max]
	}
	return string(runes)
}
func nonempty(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
func countPointer(value *Count) *int {
	if value == nil {
		return nil
	}
	n := int(*value)
	return &n
}
func sourceTime(value string) *string {
	for _, layout := range []string{"2006-01-02 15:04:05.999999999", "2006-01-02 15:04", "2006-01-02"} {
		parsed, err := time.ParseInLocation(layout, value, time.UTC)
		if err == nil {
			formatted := parsed.UTC().Format(time.RFC3339Nano)
			return &formatted
		}
	}
	return nil
}

func ReportState(positive, negative *int) string {
	if positive == nil || negative == nil {
		return "counts_missing"
	}
	if *positive > 0 && *negative > 0 {
		return "conflicting"
	}
	if *positive > 0 {
		return "reported_affirmative"
	}
	if *negative > 0 {
		return "reported_negative"
	}
	return "unreported"
}

type Requirement struct {
	ID       int    `json:"question_id"`
	Label    string `json:"label,omitempty"`
	State    string `json:"state"`
	Positive *int   `json:"positive_reports"`
	Negative *int   `json:"negative_reports"`
	Reason   string `json:"reason,omitempty"`
}

func Evaluate(spot Spot, questions []int) []Requirement {
	results := make([]Requirement, 0, len(questions))
	for _, id := range questions {
		r := Requirement{ID: id, State: "unreported", Reason: "question_missing"}
		if spot.DetailStatus == "unavailable" {
			r.State = "unavailable"
			r.Reason = "detail_fetch_failed"
		} else if category, known := QuestionCategory(id); !known || category != spot.Category {
			r.State = "inapplicable"
			r.Reason = "different_source_category"
		} else if spot.DetailStatus != "checked" {
			r.State = "not_checked"
			r.Reason = "detail_not_expanded"
		} else {
			for _, question := range spot.Questions {
				if question.ID == id {
					r.Label = question.Label
					r.State = question.State
					r.Positive = question.Positive
					r.Negative = question.Negative
					r.Reason = ""
					break
				}
			}
		}
		results = append(results, r)
	}
	return results
}

func Age(value *string, now time.Time) *float64 {
	if value == nil {
		return nil
	}
	stamp, err := time.Parse(time.RFC3339Nano, *value)
	if err != nil {
		return nil
	}
	days := now.Sub(stamp).Hours() / 24
	return &days
}

func RecheckReasons(spot Spot, requirements []int, maxAge time.Duration, now time.Time) []string {
	reasons := make([]string, 0)
	if spot.DetailStatus != "checked" {
		reasons = append(reasons, "missing_checked_detail")
	}
	if spot.Updated == nil {
		reasons = append(reasons, "source_update_date_unknown")
	} else if age := Age(spot.Updated, now); age != nil {
		if *age < 0 {
			reasons = append(reasons, "source_update_date_in_future")
		} else if *age > maxAge.Hours()/24 {
			reasons = append(reasons, "source_record_update_old")
		}
	}
	if age := Age(&spot.ObservedAt, now); age == nil {
		reasons = append(reasons, "retrieval_date_unknown")
	} else if *age > maxAge.Hours()/24 {
		reasons = append(reasons, "cached_observation_old")
	}
	if len(requirements) == 0 {
		for _, question := range spot.Questions {
			requirements = append(requirements, question.ID)
		}
	}
	for _, r := range Evaluate(spot, requirements) {
		switch r.State {
		case "conflicting", "unreported", "counts_missing", "inapplicable", "not_checked", "unavailable":
			reasons = append(reasons, "question_"+strconv.Itoa(r.ID)+"_"+r.State)
		}
	}
	return reasons
}

func ValidCoordinate(lat, lon float64) bool {
	return !math.IsNaN(lat) && !math.IsNaN(lon) && !math.IsInf(lat, 0) && !math.IsInf(lon, 0) && lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180
}
func Distance(origin, location Coordinate) float64 {
	const earthRadius = 6371008.8
	lat1 := origin.Latitude * math.Pi / 180
	lat2 := location.Latitude * math.Pi / 180
	dlat := lat2 - lat1
	dlon := (location.Longitude - origin.Longitude) * math.Pi / 180
	a := math.Pow(math.Sin(dlat/2), 2) + math.Cos(lat1)*math.Cos(lat2)*math.Pow(math.Sin(dlon/2), 2)
	a = math.Min(1, math.Max(0, a))
	return earthRadius * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

type Change struct {
	Field    string `json:"field"`
	Previous any    `json:"previous"`
	Current  any    `json:"current"`
}

func Changes(previous, current Spot) []Change {
	changes := make([]Change, 0)
	before := map[string]any{"name": previous.Name, "address": previous.Address, "country": previous.Country, "category": previous.Category, "location": previous.Location, "record_created_at": previous.Created, "record_updated_at": previous.Updated}
	after := map[string]any{"name": current.Name, "address": current.Address, "country": current.Country, "category": current.Category, "location": current.Location, "record_created_at": current.Created, "record_updated_at": current.Updated}
	for key, old := range before {
		if !reflect.DeepEqual(old, after[key]) {
			changes = append(changes, Change{key, old, after[key]})
		}
	}
	oldQuestions := map[int]Question{}
	newQuestions := map[int]Question{}
	for _, q := range previous.Questions {
		oldQuestions[q.ID] = q
	}
	for _, q := range current.Questions {
		newQuestions[q.ID] = q
	}
	all := map[int]bool{}
	for id := range oldQuestions {
		all[id] = true
	}
	for id := range newQuestions {
		all[id] = true
	}
	for id := range all {
		old, oldOK := oldQuestions[id]
		next, nextOK := newQuestions[id]
		if !oldOK || !nextOK || !reflect.DeepEqual(old, next) {
			var a, b any
			if oldOK {
				a = old
			}
			if nextOK {
				b = next
			}
			changes = append(changes, Change{"question_" + strconv.Itoa(id), a, b})
		}
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Field < changes[j].Field })
	return changes
}
