// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// Public station facts are explicitly whitelisted; orders, reviews and private notes are excluded.
package carstay

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/carstay/internal/cliutil"
)

var idPattern = regexp.MustCompile(`^[0-9a-f]{24}$`)
var FacilityGroups = map[string][]string{
	"on_site": {"restroom", "water", "wifi", "playground", "petsAllowed", "smokingArea", "vendingMachine", "washingDryer", "wasteWaterDischarge", "securityCameras", "evCharger", "administrator", "campingBehaviorAllowed", "bonfireAllowed", "cookingAllowed"},
	"options": {"electricity", "nonFreeShower", "nonFreeWifi", "dustbin", "dogPark", "onsenAdult", "onsenChild", "bbqFacilitiesRental", "tentRentalLarge", "tentRentalSmall"},
	"nearby":  {"combini", "supermarket", "gasoline", "restaurant", "toilet", "onsen"},
}

type Facility struct {
	Key            string   `json:"key"`
	Group          string   `json:"group"`
	Status         string   `json:"status"`
	SourcePresent  *bool    `json:"source_present"`
	Notification   string   `json:"notification,omitempty"`
	SourcePriceJPY *float64 `json:"source_price_jpy"`
	FeeBasis       string   `json:"fee_basis"`
	SourceDistance *float64 `json:"source_distance"`
	DistanceUnit   string   `json:"distance_unit"`
	Name           string   `json:"name,omitempty"`
}
type Space struct {
	Length  *float64 `json:"length_m"`
	Width   *float64 `json:"width_m"`
	Height  *float64 `json:"height_m"`
	Meaning string   `json:"meaning"`
}
type Spot struct {
	ID                string     `json:"id"`
	Name              string     `json:"name_ja"`
	NameEN            string     `json:"name_en,omitempty"`
	Prefecture        string     `json:"prefecture_ja"`
	PrefectureEN      string     `json:"prefecture_en,omitempty"`
	Region            string     `json:"region_ja"`
	Description       string     `json:"description_ja,omitempty"`
	DescriptionEN     string     `json:"description_en,omitempty"`
	Rules             string     `json:"rules_ja,omitempty"`
	BusinessHours     string     `json:"business_hours_note_ja,omitempty"`
	Address           string     `json:"address_ja,omitempty"`
	EnglishApproved   *bool      `json:"english_approved"`
	ActivityOnly      *bool      `json:"activity_only"`
	SourceURL         string     `json:"source_url"`
	EnglishURL        string     `json:"english_url,omitempty"`
	ObservedAt        string     `json:"observed_at"`
	SourceLanguage    string     `json:"source_language"`
	StartingPriceJPY  *float64   `json:"starting_price_jpy"`
	PriceBasis        string     `json:"price_basis"`
	Availability      string     `json:"availability"`
	VehicleAcceptance string     `json:"vehicle_acceptance"`
	Latitude          *float64   `json:"latitude"`
	Longitude         *float64   `json:"longitude"`
	DistanceKM        *float64   `json:"straight_line_distance_km,omitempty"`
	ParkingSpace      Space      `json:"parking_space"`
	Facilities        []Facility `json:"facilities,omitempty"`
	Warnings          []string   `json:"warnings"`
}

func ValidID(id string) bool { return idPattern.MatchString(id) }
func number(m map[string]json.RawMessage, key string) (*float64, error) {
	b, ok := m[key]
	if !ok || string(b) == "null" {
		return nil, nil
	}
	var n float64
	if err := json.Unmarshal(b, &n); err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
		return nil, fmt.Errorf("invalid source numeric field %s", key)
	}
	return &n, nil
}
func text(m map[string]json.RawMessage, key string) string {
	var s string
	_ = json.Unmarshal(m[key], &s)
	if key == "_id" || key == "name" || key == "nameEn" || key == "prefecture" || key == "prefectureEn" || key == "region" || key == "address" {
		return s
	}
	return cliutil.CleanText(s)
}
func boolean(m map[string]json.RawMessage, key string) (*bool, error) {
	b, ok := m[key]
	if !ok || string(b) == "null" {
		return nil, nil
	}
	var v bool
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, fmt.Errorf("invalid source boolean field %s", key)
	}
	return &v, nil
}
func Normalize(data json.RawMessage, observed string) (Spot, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return Spot{}, fmt.Errorf("invalid station object: %w", err)
	}
	s := Spot{ID: text(m, "_id"), Name: text(m, "name"), NameEN: text(m, "nameEn"), Prefecture: text(m, "prefecture"), PrefectureEN: text(m, "prefectureEn"), Region: text(m, "region"), Description: text(m, "description"), DescriptionEN: text(m, "descriptionEn"), Rules: text(m, "note"), BusinessHours: text(m, "businessHourNote"), Address: text(m, "address"), ObservedAt: observed, SourceLanguage: "ja", PriceBasis: "provider_starting_reference_per_night_not_dated_total", Availability: "unknown", VehicleAcceptance: "unknown", Warnings: []string{}, ParkingSpace: Space{Meaning: "parking_area_dimensions_not_vehicle_acceptance"}}
	if !ValidID(s.ID) || s.Name == "" {
		return Spot{}, fmt.Errorf("source station lacks a valid stable ID or Japanese name")
	}
	var err error
	s.ActivityOnly, err = boolean(m, "activityOnly")
	if err != nil {
		return Spot{}, err
	}
	en, err := boolean(m, "approvedEn")
	if err != nil {
		return Spot{}, err
	}
	s.EnglishApproved = en
	s.StartingPriceJPY, err = number(m, "price")
	if err != nil {
		return Spot{}, err
	}
	s.ParkingSpace.Length, err = number(m, "length")
	if err != nil {
		return Spot{}, err
	}
	s.ParkingSpace.Width, err = number(m, "breadth")
	if err != nil {
		return Spot{}, err
	}
	s.ParkingSpace.Height, err = number(m, "height")
	if err != nil {
		return Spot{}, err
	}
	for _, p := range []*float64{s.StartingPriceJPY, s.ParkingSpace.Length, s.ParkingSpace.Width, s.ParkingSpace.Height} {
		if p != nil && *p < 0 {
			return Spot{}, fmt.Errorf("source contains a negative price or parking dimension")
		}
	}
	var location []float64
	if json.Unmarshal(m["location"], &location) == nil && len(location) == 2 && location[0] >= -180 && location[0] <= 180 && location[1] >= -90 && location[1] <= 90 {
		lon, lat := location[0], location[1]
		s.Longitude = &lon
		s.Latitude = &lat
	}
	area := AreaFor(s.Prefecture)
	if area == "" {
		return Spot{}, fmt.Errorf("source prefecture %q has no verified canonical area", s.Prefecture)
	}
	s.SourceURL = fmt.Sprintf("https://carstay.jp/ja/stations/%s/station/%s/", area, s.ID)
	if EnglishPublished(s) {
		s.EnglishURL = strings.Replace(s.SourceURL, "/ja/", "/en/", 1)
	}
	if EnglishPublished(s) && s.NameEN == "" {
		s.Warnings = append(s.Warnings, "English publication flag is true but translated name is absent")
	}
	for _, group := range []string{"on_site", "options", "nearby"} {
		for _, key := range FacilityGroups[group] {
			f := Facility{Key: key, Group: group, Status: "unknown", FeeBasis: "unknown_source_option_unit", DistanceUnit: "unverified_source_unit"}
			if b, ok := m[key]; ok && string(b) != "null" {
				var obj map[string]json.RawMessage
				if err := json.Unmarshal(b, &obj); err != nil {
					return Spot{}, fmt.Errorf("invalid source facility %s", key)
				}
				f.SourcePresent, err = boolean(obj, "exist")
				if err != nil {
					return Spot{}, fmt.Errorf("facility %s: %w", key, err)
				}
				f.Notification = text(obj, "notification")
				f.Name = text(obj, "name")
				f.SourcePriceJPY, err = number(obj, "price")
				if err != nil {
					return Spot{}, err
				}
				f.SourceDistance, err = number(obj, "distance")
				if err != nil {
					return Spot{}, err
				}
				if f.SourcePresent != nil {
					if *f.SourcePresent {
						f.Status = "reported_present"
					} else if f.Notification != "" {
						f.Status = "requires_confirmation"
					} else {
						f.Status = "reported_absent"
					}
				}
			}
			s.Facilities = append(s.Facilities, f)
		}
	}
	return s, nil
}
func Overnight(s Spot) bool { return s.ActivityOnly != nil && !*s.ActivityOnly }
func Summary(s Spot) Spot {
	s.Description = ""
	s.DescriptionEN = ""
	s.Rules = ""
	s.BusinessHours = ""
	s.Address = ""
	s.Facilities = nil
	return s
}
func AreaFor(pref string) string { return prefectureAreas[pref] }

var prefectureAreas = map[string]string{
	"北海道": "hokkaido", "青森県": "tohoku", "岩手県": "tohoku", "宮城県": "tohoku", "秋田県": "tohoku", "山形県": "tohoku", "福島県": "tohoku",
	"茨城県": "kanto", "栃木県": "kanto", "群馬県": "kanto", "埼玉県": "kanto", "千葉県": "kanto", "東京都": "kanto", "神奈川県": "kanto",
	"新潟県": "hokuriku", "富山県": "hokuriku", "石川県": "hokuriku", "福井県": "hokuriku",
	"山梨県": "chubu", "長野県": "chubu", "岐阜県": "chubu", "静岡県": "chubu", "愛知県": "chubu",
	"三重県": "kinki", "滋賀県": "kinki", "京都府": "kinki", "大阪府": "kinki", "兵庫県": "kinki", "奈良県": "kinki", "和歌山県": "kinki",
	"鳥取県": "chugoku", "島根県": "chugoku", "岡山県": "chugoku", "広島県": "chugoku", "山口県": "chugoku",
	"徳島県": "shikoku", "香川県": "shikoku", "愛媛県": "shikoku", "高知県": "shikoku",
	"福岡県": "kyushu", "佐賀県": "kyushu", "長崎県": "kyushu", "熊本県": "kyushu", "大分県": "kyushu", "宮崎県": "kyushu", "鹿児島県": "kyushu", "沖縄県": "kyushu",
}
var prefectureEnglish = map[string]string{"hokkaido": "北海道", "aomori": "青森県", "iwate": "岩手県", "miyagi": "宮城県", "akita": "秋田県", "yamagata": "山形県", "fukushima": "福島県", "ibaraki": "茨城県", "tochigi": "栃木県", "gunma": "群馬県", "saitama": "埼玉県", "chiba": "千葉県", "tokyo": "東京都", "kanagawa": "神奈川県", "niigata": "新潟県", "toyama": "富山県", "ishikawa": "石川県", "fukui": "福井県", "yamanashi": "山梨県", "nagano": "長野県", "gifu": "岐阜県", "shizuoka": "静岡県", "aichi": "愛知県", "mie": "三重県", "shiga": "滋賀県", "kyoto": "京都府", "osaka": "大阪府", "hyogo": "兵庫県", "nara": "奈良県", "wakayama": "和歌山県", "tottori": "鳥取県", "shimane": "島根県", "okayama": "岡山県", "hiroshima": "広島県", "yamaguchi": "山口県", "tokushima": "徳島県", "kagawa": "香川県", "ehime": "愛媛県", "kochi": "高知県", "fukuoka": "福岡県", "saga": "佐賀県", "nagasaki": "長崎県", "kumamoto": "熊本県", "oita": "大分県", "miyazaki": "宮崎県", "kagoshima": "鹿児島県", "okinawa": "沖縄県"}

func Prefecture(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	if prefectureAreas[s] != "" {
		return s, nil
	}
	if ja := prefectureEnglish[strings.ToLower(s)]; ja != "" {
		return ja, nil
	}
	return "", fmt.Errorf("unknown --prefecture %q; use a Japanese prefecture or English name such as Yamanashi", s)
}
func FacilityKnown(key string) bool {
	for _, keys := range FacilityGroups {
		for _, k := range keys {
			if k == key {
				return true
			}
		}
	}
	return false
}
func Match(s Spot, query, pref, language string) bool {
	if !Overnight(s) || pref != "" && s.Prefecture != pref || language == "en" && !EnglishPublished(s) {
		return false
	}
	hay := strings.ToLower(strings.Join([]string{s.Name, s.NameEN, s.Prefecture, s.PrefectureEN, s.Region, s.Description, s.DescriptionEN}, " "))
	for _, term := range strings.Fields(strings.ToLower(query)) {
		if !strings.Contains(hay, term) {
			return false
		}
	}
	return true
}
func DistanceKM(lat1, lon1, lat2, lon2 float64) float64 {
	r := math.Pi / 180
	dlat := (lat2 - lat1) * r
	dlon := (lon2 - lon1) * r
	a := math.Sin(dlat/2)*math.Sin(dlat/2) + math.Cos(lat1*r)*math.Cos(lat2*r)*math.Sin(dlon/2)*math.Sin(dlon/2)
	a = math.Min(1, math.Max(0, a))
	return 6371.0088 * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
func RankNear(spots []Spot, lat, lon, radius float64) []Spot {
	out := []Spot{}
	for _, s := range spots {
		if !Overnight(s) || s.Latitude == nil || s.Longitude == nil {
			continue
		}
		d := DistanceKM(lat, lon, *s.Latitude, *s.Longitude)
		if d <= radius {
			s.DistanceKM = &d
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if *out[i].DistanceKM == *out[j].DistanceKM {
			return out[i].ID < out[j].ID
		}
		return *out[i].DistanceKM < *out[j].DistanceKM
	})
	return out
}
func Dates(in, out string) error {
	if in == "" && out == "" {
		return nil
	}
	if in == "" || out == "" {
		return fmt.Errorf("--check-in and --check-out must be supplied together as YYYY-MM-DD JST dates")
	}
	loc := time.FixedZone("JST", 9*3600)
	a, e := time.ParseInLocation("2006-01-02", in, loc)
	if e != nil {
		return fmt.Errorf("invalid --check-in: use YYYY-MM-DD")
	}
	b, e := time.ParseInLocation("2006-01-02", out, loc)
	if e != nil {
		return fmt.Errorf("invalid --check-out: use YYYY-MM-DD")
	}
	if !b.After(a) {
		return fmt.Errorf("--check-out must be later than --check-in")
	}
	return nil
}
func Handoff(s Spot, language, in, out string) (string, error) {
	if err := Dates(in, out); err != nil {
		return "", err
	}
	if !Overnight(s) {
		return "", fmt.Errorf("activity-only or unknown station is not an overnight booking handoff")
	}
	u := s.SourceURL
	if language == "en" && s.EnglishURL != "" {
		u = s.EnglishURL
	}
	parsed, err := url.Parse(u)
	if err != nil {
		return "", err
	}
	q := parsed.Query()
	if in != "" {
		q.Set("checkIn", in)
		q.Set("checkOut", out)
	}
	parsed.RawQuery = q.Encode()
	return parsed.String(), nil
}

type Constraints struct {
	Length, Width, Height float64
	Require               []string
}
type Check struct {
	Field     string   `json:"field"`
	State     string   `json:"state"`
	Requested *float64 `json:"requested_m,omitempty"`
	Source    *float64 `json:"source_m,omitempty"`
	Note      string   `json:"note,omitempty"`
}
type Assessment struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name_ja"`
	SourceURL          string  `json:"source_url"`
	ObservedAt         string  `json:"observed_at"`
	SpaceAndFacilities string  `json:"space_and_facilities"`
	VehicleAcceptance  string  `json:"vehicle_acceptance"`
	Checks             []Check `json:"checks"`
}

func Assess(s Spot, c Constraints) Assessment {
	a := Assessment{ID: s.ID, Name: s.Name, SourceURL: s.SourceURL, ObservedAt: s.ObservedAt, SpaceAndFacilities: "fits_reported_constraints_only", VehicleAcceptance: "unknown", Checks: []Check{}}
	for _, d := range []struct {
		name string
		want float64
		have *float64
	}{{"length", c.Length, s.ParkingSpace.Length}, {"width", c.Width, s.ParkingSpace.Width}, {"height", c.Height, s.ParkingSpace.Height}} {
		if d.want == 0 {
			continue
		}
		w := d.want
		ch := Check{Field: "parking_space_" + d.name, Requested: &w, Source: d.have, State: "unknown"}
		if d.have != nil && *d.have > 0 {
			if w <= *d.have {
				ch.State = "fits_reported_space"
			} else {
				ch.State = "exceeds_reported_space"
				a.SpaceAndFacilities = "fails_reported_constraint"
			}
		} else if a.SpaceAndFacilities != "fails_reported_constraint" {
			a.SpaceAndFacilities = "requires_confirmation"
		}
		a.Checks = append(a.Checks, ch)
	}
	for _, key := range c.Require {
		ch := Check{Field: key, State: "unknown"}
		for _, f := range s.Facilities {
			if f.Key == key {
				ch.State = f.Status
				ch.Note = f.Notification
				break
			}
		}
		if ch.State == "reported_absent" {
			a.SpaceAndFacilities = "fails_reported_constraint"
		} else if ch.State != "reported_present" && a.SpaceAndFacilities != "fails_reported_constraint" {
			a.SpaceAndFacilities = "requires_confirmation"
		}
		a.Checks = append(a.Checks, ch)
	}
	return a
}

type EvidenceAudit struct {
	ID                    string     `json:"id"`
	Name                  string     `json:"name_ja"`
	SourceURL             string     `json:"source_url"`
	ObservedAt            string     `json:"observed_at"`
	ObservationAgeSeconds float64    `json:"observation_age_seconds"`
	PriceBasis            string     `json:"price_basis"`
	Availability          string     `json:"availability"`
	DateCandidacy         string     `json:"date_candidacy"`
	UnknownFields         []string   `json:"unknown_fields"`
	QualifiedFacilities   []Facility `json:"qualified_facilities"`
	UnresolvedFees        []Facility `json:"unresolved_fees"`
}

func Audit(s Spot, now time.Time, candidacy string) EvidenceAudit {
	a := EvidenceAudit{ID: s.ID, Name: s.Name, SourceURL: s.SourceURL, ObservedAt: s.ObservedAt, PriceBasis: s.PriceBasis, Availability: "unknown", DateCandidacy: candidacy, UnknownFields: []string{"complete_dated_total", "remaining_availability", "vehicle_acceptance"}, QualifiedFacilities: []Facility{}, UnresolvedFees: []Facility{}}
	if t, err := time.Parse(time.RFC3339, s.ObservedAt); err == nil {
		a.ObservationAgeSeconds = math.Max(0, now.Sub(t).Seconds())
	}
	if s.StartingPriceJPY == nil {
		a.UnknownFields = append(a.UnknownFields, "starting_price_jpy")
	}
	for _, d := range []struct {
		name  string
		value *float64
	}{{"length_m", s.ParkingSpace.Length}, {"width_m", s.ParkingSpace.Width}, {"height_m", s.ParkingSpace.Height}} {
		if d.value == nil || *d.value == 0 {
			a.UnknownFields = append(a.UnknownFields, "parking_space."+d.name)
		}
	}
	for _, f := range s.Facilities {
		if f.Status == "unknown" {
			a.UnknownFields = append(a.UnknownFields, "facility."+f.Key)
		}
		if f.Status == "requires_confirmation" {
			a.QualifiedFacilities = append(a.QualifiedFacilities, f)
		}
		if f.Group == "options" && (f.SourcePriceJPY == nil || f.SourcePresent == nil || !*f.SourcePresent || f.Notification != "") {
			a.UnresolvedFees = append(a.UnresolvedFees, f)
		}
	}
	return a
}

func EnglishPublished(s Spot) bool { return s.EnglishApproved != nil && *s.EnglishApproved }
