// Package repark reads the public Repark time-parking website.
package repark

import (
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const Origin = "https://www.repark.jp"

var JST = time.FixedZone("JST", 9*60*60)
var idPattern = regexp.MustCompile(`^REP[0-9]{7}$`)

type Coordinates struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type Limits struct {
	HeightM    *float64 `json:"height_m"`
	LengthM    *float64 `json:"length_m"`
	WidthM     *float64 `json:"width_m"`
	WeightT    *float64 `json:"weight_t"`
	SourceText string   `json:"source_text"`
	Note       string   `json:"note,omitempty"`
	PerBay     *bool    `json:"varies_by_bay"`
}

type Vehicle struct {
	HeightM *float64 `json:"height_m,omitempty"`
	LengthM *float64 `json:"length_m,omitempty"`
	WidthM  *float64 `json:"width_m,omitempty"`
	WeightT *float64 `json:"weight_t,omitempty"`
}

type Fit struct {
	Status          string   `json:"status"`
	Conflicts       []string `json:"published_limit_conflicts"`
	UnknownFields   []string `json:"unknown_supplied_limits"`
	RemainingBayFit string   `json:"remaining_bay_fit"`
	Guaranteed      bool     `json:"guaranteed"`
}

type Occupancy struct {
	Category             string  `json:"category"`
	LabelJA              string  `json:"label_ja"`
	SourceCode           string  `json:"source_code"`
	ObservedAt           string  `json:"observed_at"`
	MeasurementTime      *string `json:"measurement_time"`
	ExactAvailableSpaces *int    `json:"exact_available_spaces"`
}

type Hours struct {
	Text       string `json:"source_text"`
	Open24H    *bool  `json:"open_24_hours"`
	Start      string `json:"start_time,omitempty"`
	End        string `json:"end_time,omitempty"`
	SourceType string `json:"source_type,omitempty"`
	Note       string `json:"note,omitempty"`
}

type Rate struct {
	DayType         string `json:"day_type"`
	Start           string `json:"start_time,omitempty"`
	End             string `json:"end_time,omitempty"`
	Overnight       bool   `json:"crosses_midnight"`
	IntervalMinutes *int   `json:"interval_minutes"`
	AmountJPY       *int   `json:"amount_jpy"`
	SourceText      string `json:"source_text"`
	SourceNote      string `json:"source_note,omitempty"`
}

type Maximum struct {
	DayType      string `json:"day_type,omitempty"`
	Kind         string `json:"kind"`
	AmountJPY    *int   `json:"amount_jpy"`
	Start        string `json:"start_time,omitempty"`
	End          string `json:"end_time,omitempty"`
	Overnight    bool   `json:"crosses_midnight"`
	ElapsedHours *int   `json:"elapsed_hours"`
	Application  string `json:"application"`
	SourceText   string `json:"source_text"`
}

type Lot struct {
	ID                 string       `json:"id"`
	Name               string       `json:"name"`
	Address            string       `json:"address"`
	Coordinates        *Coordinates `json:"coordinates"`
	DistanceM          *int         `json:"straight_line_distance_m,omitempty"`
	Capacity           *int         `json:"capacity_spaces"`
	Hours              Hours        `json:"hours"`
	Occupancy          Occupancy    `json:"occupancy"`
	Limits             Limits       `json:"vehicle_limits"`
	Fit                Fit          `json:"declared_fit"`
	Rates              []Rate       `json:"rates"`
	SourceRateText     string       `json:"source_rate_text"`
	PricingParseStatus string       `json:"pricing_parse_status"`
	Maximums           []Maximum    `json:"maximum_charges"`
	MaximumApplication string       `json:"maximum_application"`
	ChargeNote         string       `json:"source_charge_note"`
	TaxIncluded        *bool        `json:"tax_included"`
	Attention          string       `json:"source_attention,omitempty"`
	SourceURL          string       `json:"source_url"`
	CalculatorURL      string       `json:"calculator_url,omitempty"`
	SourceImportDate   string       `json:"source_import_date_raw,omitempty"`
	SourceUpdatedAt    string       `json:"source_updated_at_raw,omitempty"`
	ObservedAt         string       `json:"observed_at"`
}

func CanonicalID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if strings.Contains(value, "://") {
		u, err := url.Parse(value)
		if err != nil || u.Scheme != "https" || u.Host != "www.repark.jp" || u.Path != "/parking_user/time/result/detail/" || u.User != nil {
			return "", fmt.Errorf("lot must be a REP ID or canonical https://www.repark.jp/parking_user/time/result/detail/?park=REP... URL")
		}
		value = u.Query().Get("park")
	}
	if !idPattern.MatchString(value) {
		return "", fmt.Errorf("invalid lot ID %q: use REP followed by seven digits, for example REP0022209", value)
	}
	return value, nil
}

func DetailURL(id string) string {
	return Origin + "/parking_user/time/result/detail/?park=" + url.QueryEscape(id)
}

func ValidateCoordinates(p Coordinates) error {
	if math.IsNaN(p.Latitude) || math.IsNaN(p.Longitude) || math.IsInf(p.Latitude, 0) || math.IsInf(p.Longitude, 0) || p.Latitude < -90 || p.Latitude > 90 || p.Longitude < -180 || p.Longitude > 180 {
		return fmt.Errorf("--lat and --lon must be finite WGS84 coordinates within [-90,90] and [-180,180]")
	}
	// The provider covers Japan. Avoid unsupported polar/dateline range windows.
	if p.Latitude < 20 || p.Latitude > 46 || p.Longitude < 122 || p.Longitude > 154 {
		return fmt.Errorf("Repark search coordinates must be within Japan's supported bounding region (lat 20..46, lon 122..154)")
	}
	return nil
}

func DistanceM(a, b Coordinates) int {
	r := math.Pi / 180
	x := math.Sin((b.Latitude - a.Latitude) * r / 2)
	y := math.Sin((b.Longitude - a.Longitude) * r / 2)
	h := x*x + math.Cos(a.Latitude*r)*math.Cos(b.Latitude*r)*y*y
	return int(math.Round(6371008.8 * 2 * math.Asin(math.Sqrt(math.Min(1, math.Max(0, h))))))
}

func AssessFit(l Limits, v Vehicle) Fit {
	f := Fit{Status: "not_assessed", Conflicts: []string{}, UnknownFields: []string{}, RemainingBayFit: "unknown"}
	for _, p := range []struct {
		name            string
		supplied, limit *float64
	}{{"height_m", v.HeightM, l.HeightM}, {"length_m", v.LengthM, l.LengthM}, {"width_m", v.WidthM, l.WidthM}, {"weight_t", v.WeightT, l.WeightT}} {
		if p.supplied == nil {
			continue
		}
		f.Status = "within_supplied_published_limits"
		if p.limit == nil {
			f.UnknownFields = append(f.UnknownFields, p.name)
		} else if *p.supplied > *p.limit {
			f.Conflicts = append(f.Conflicts, p.name)
		}
	}
	if len(f.Conflicts) > 0 {
		f.Status = "exceeds_published_limits"
	} else if len(f.UnknownFields) > 0 {
		f.Status = "unknown"
	}
	return f
}

func ValidateVehicle(v Vehicle) error {
	for _, p := range []*float64{v.HeightM, v.LengthM, v.WidthM, v.WeightT} {
		if p != nil && (math.IsNaN(*p) || math.IsInf(*p, 0) || *p <= 0 || *p > 100) {
			return fmt.Errorf("vehicle dimensions in metres and weight in tonnes must be finite, positive, and no greater than 100")
		}
	}
	return nil
}

func ParseJST(value string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02 15:04"} {
		if t, err := time.ParseInLocation(layout, value, JST); err == nil {
			return t, nil
		}
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil && t.Second() == 0 && t.Nanosecond() == 0 {
		return t.In(JST), nil
	}
	return time.Time{}, fmt.Errorf("invalid date/time %q: use YYYY-MM-DDTHH:MM in JST or RFC3339 with zero seconds and an explicit offset", value)
}
