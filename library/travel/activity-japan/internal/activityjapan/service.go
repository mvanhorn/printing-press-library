// Package activityjapan interprets Activity Japan's public, read-only plan JSON.
// These website endpoints are undocumented and can change without notice.
package activityjapan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/activity-japan/internal/client"
)

var digits = regexp.MustCompile(`^[0-9]+$`)
var tokyo = time.FixedZone("Asia/Tokyo", 9*3600)

func ObservedAt() string { return time.Now().In(tokyo).Format(time.RFC3339) }
func PlanURL(id, lang string) string {
	host := "activityjapan.com"
	if lang == "en" {
		host = "en.activityjapan.com"
	}
	return "https://" + host + "/publish/plan/" + id
}
func ValidateID(id string) error {
	if !digits.MatchString(id) || len(id) > 12 {
		return fmt.Errorf("invalid plan ID %q: use the numeric Plan ID from Activity Japan search or a plan URL", id)
	}
	return nil
}
func ValidateDate(s string) error {
	d, err := time.ParseInLocation("2006-01-02", s, tokyo)
	if err != nil || d.Format("2006-01-02") != s {
		return fmt.Errorf("invalid activity date %q: use YYYY-MM-DD in Asia/Tokyo", s)
	}
	today, _ := time.ParseInLocation("2006-01-02", time.Now().In(tokyo).Format("2006-01-02"), tokyo)
	if d.Before(today) {
		return fmt.Errorf("activity date %s is in the past in Asia/Tokyo", s)
	}
	if d.After(time.Now().In(tokyo).AddDate(1, 0, 0)) {
		return fmt.Errorf("activity date %s exceeds the one-year lookup bound", s)
	}
	return nil
}
func ValidLang(lang string) error {
	if lang != "en" && lang != "ja" {
		return fmt.Errorf("invalid site language %q: use en or ja", lang)
	}
	return nil
}

type Service struct {
	Client   *client.Client
	Requests int
}

// SchemaError reports a source response that cannot be interpreted safely.
type SchemaError struct {
	Endpoint string
	Reason   string
}

func (e *SchemaError) Error() string { return "Activity Japan " + e.Endpoint + " schema: " + e.Reason }

// PartialItemsError accompanies otherwise usable items when malformed siblings
// were omitted. Callers must surface it and mark their output partial.
type PartialItemsError struct {
	Endpoint string
	Count    int
}

func (e *PartialItemsError) Error() string {
	return fmt.Sprintf("Activity Japan %s: %d malformed source item(s) omitted", e.Endpoint, e.Count)
}

func NewService(c *client.Client) *Service { return &Service{Client: c} }
func (s *Service) get(ctx context.Context, path string, q map[string]string) (map[string]any, error) {
	s.Requests++
	raw, err := s.Client.GetNoCache(ctx, path, q)
	if err != nil {
		return nil, fmt.Errorf("Activity Japan %s: %w", path, err)
	}
	var out map[string]any
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	if err := dec.Decode(&out); err != nil || out == nil {
		return nil, &SchemaError{Endpoint: path, Reason: "invalid JSON object or source denial"}
	}
	return out, nil
}
func object(v any) map[string]any { x, _ := v.(map[string]any); return x }
func list(v any) []any            { x, _ := v.([]any); return x }
func Str(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case json.Number:
		return x.String()
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	}
	return ""
}
func Text(v any) *string {
	s := strings.TrimSpace(Str(v))
	if s == "" {
		return nil
	}
	return &s
}
func Int(v any) *int {
	s := Str(v)
	if s == "" {
		return nil
	}
	n, e := strconv.Atoi(s)
	if e != nil {
		return nil
	}
	return &n
}
func PositiveInt(v any) *int {
	n := Int(v)
	if n == nil || *n <= 0 {
		return nil
	}
	return n
}
func Bool(v any) *bool {
	b, ok := v.(bool)
	if !ok {
		return nil
	}
	return &b
}
func safePeriod(v any) *string {
	s := Text(v)
	if s == nil || strings.HasPrefix(*s, "0000-00-00") {
		return nil
	}
	return s
}

var exactMinutes = regexp.MustCompile(`^(\d{1,3})\s+minutes?$`)
var exactHours = regexp.MustCompile(`^(\d{1,2})\s+hours?$`)
var exactJapaneseMinutes = regexp.MustCompile(`^(?:約)?(\d{1,3})分$`)
var exactJapaneseHours = regexp.MustCompile(`^(?:約)?(\d{1,2})時間$`)

// ParseExactDuration only interprets a whole-field duration. Mixed total and
// activity prose, ranges, and machine-translation errors remain source text.
func ParseExactDuration(v *string) *int {
	if v == nil {
		return nil
	}
	s := strings.ToLower(strings.TrimSpace(*v))
	for _, re := range []*regexp.Regexp{exactMinutes, exactHours, exactJapaneseMinutes, exactJapaneseHours} {
		match := re.FindStringSubmatch(s)
		if len(match) != 2 {
			continue
		}
		n, e := strconv.Atoi(match[1])
		if e != nil || n <= 0 {
			return nil
		}
		if re == exactHours || re == exactJapaneseHours {
			n *= 60
		}
		return &n
	}
	return nil
}

// Plan keeps source facts and derived values in distinct fields. Unknown facts are null.
type Plan struct {
	PlanID                  string            `json:"plan_id"`
	OperatorID              *string           `json:"operator_id"`
	OperatorNameOriginal    *string           `json:"operator_name_original"`
	NameOriginalJA          *string           `json:"name_original_ja"`
	NameLocalized           *string           `json:"name_localized"`
	SiteLanguage            string            `json:"site_language"`
	SourceURL               string            `json:"source_url"`
	CanonicalURL            *string           `json:"canonical_url"`
	URLs                    map[string]string `json:"language_urls"`
	HeadlineBasePriceJPY    *int              `json:"headline_base_price_jpy"`
	HeadlineDiscountJPY     *int              `json:"headline_discount_jpy"`
	DerivedHeadlineFromJPY  *int              `json:"derived_headline_from_jpy"`
	VenueAddress            *string           `json:"venue_address"`
	MeetingPoint            *string           `json:"meeting_point"`
	PickupLocation          *string           `json:"pickup_location"`
	DurationTotalText       *string           `json:"duration_total_text"`
	DerivedTotalMinutes     *int              `json:"derived_total_minutes"`
	ActivityMinutes         *int              `json:"activity_minutes"`
	MeetingTimeText         *string           `json:"meeting_time_text"`
	CheckInLeadMinutes      *int              `json:"check_in_lead_minutes"`
	AgeMinYears             *int              `json:"age_min_years"`
	AgeMaxYears             *int              `json:"age_max_years"`
	PartyMin                *int              `json:"party_min"`
	PartyMax                *int              `json:"party_max"`
	PartyMinSource          *int              `json:"party_min_source"`
	PartyMaxSource          *int              `json:"party_max_source"`
	BasicMinPassengers      *int              `json:"basic_min_passenger_count_source"`
	InclusionText           *string           `json:"inclusion_text"`
	ExclusionText           *string           `json:"exclusion_text"`
	EquipmentToBring        *string           `json:"equipment_to_bring"`
	RentalEquipment         *string           `json:"rental_equipment"`
	RestrictionText         *string           `json:"restriction_text"`
	AdditionalAttentionText *string           `json:"additional_attention_text"`
	OperatorMessageText     *string           `json:"operator_message_text"`
	WeatherDependence       *string           `json:"weather_dependence"`
	CancellationText        *string           `json:"cancellation_text"`
	PaymentText             *string           `json:"payment_text"`
	PeriodStart             *string           `json:"operating_period_start"`
	PeriodEnd               *string           `json:"operating_period_end"`
	SpokenLanguages         []string          `json:"spoken_languages"`
	LocaleSupportFlag       *bool             `json:"locale_support_flag"`
	Options                 []Option          `json:"options"`
	Missing                 []string          `json:"missing"`
	Derived                 map[string]any    `json:"derived"`
	ObservedAt              string            `json:"observed_at"`
	Partial                 bool              `json:"partial"`
	Errors                  []string          `json:"errors,omitempty"`
}
type Option struct {
	OptionID           string  `json:"option_id"`
	PlanType           string  `json:"plan_type"`
	NameOriginalJA     *string `json:"name_original_ja"`
	NameLocalized      *string `json:"name_localized"`
	UnitTextOriginalJA *string `json:"unit_text_original_ja"`
	UnitTextLocalized  *string `json:"unit_text_localized"`
	BasePriceJPY       *int    `json:"base_price_jpy"`
	DiscountJPY        *int    `json:"discount_jpy"`
	UndatedUnitJPY     *int    `json:"derived_undated_unit_jpy"`
	Basis              string  `json:"basis"`
	GroupSize          *int    `json:"group_size"`
	AgeBandText        *string `json:"age_band_text"`
	AgeClass           string  `json:"age_class"`
	MinUnits           *int    `json:"min_units"`
	MaxUnits           *int    `json:"max_units"`
}

func classifyBasis(s string) (string, *int) {
	folded := strings.ToLower(strings.TrimSpace(s))
	if folded == "人" || folded == "名" || folded == "person" || folded == "people" || folded == "participant" || folded == "participants" {
		return "per_person", nil
	}
	if strings.Contains(folded, "pair") || strings.Contains(folded, "ペア") || strings.Contains(folded, "2 people") || strings.Contains(folded, "2 persons") {
		n := 2
		return "per_group", &n
	}
	if strings.Contains(folded, "group") || strings.Contains(folded, "組") || strings.Contains(folded, "set") {
		return "per_group", nil
	}
	return "unknown", nil
}

func ageClass(prefix, suffix string) (string, *string) {
	label := strings.TrimSpace(prefix + " " + suffix)
	folded := strings.ToLower(label)
	switch {
	case strings.Contains(folded, "幼児"), strings.Contains(folded, "乳児"), strings.Contains(folded, "infant"), strings.Contains(folded, "baby"):
		return "infant", Text(prefix)
	case strings.Contains(folded, "小人"), strings.Contains(folded, "子供"), strings.Contains(folded, "子ども"), strings.Contains(folded, "child"), strings.Contains(folded, "kid"):
		return "child", Text(prefix)
	case strings.Contains(folded, "大人"), strings.Contains(folded, "成人"), strings.Contains(folded, "adult"):
		if strings.Contains(strings.ToLower(prefix), "大人") || strings.Contains(strings.ToLower(prefix), "adult") {
			return "adult", Text(prefix)
		}
		return "adult", Text(suffix)
	case strings.Contains(folded, "参加者"), strings.Contains(folded, "participant"):
		return "participant", Text(prefix)
	default:
		return "unknown", nil
	}
}
func parseOptions(local, ja map[string]any, separateJA bool) ([]Option, int) {
	jaByID := map[string]map[string]any{}
	invalid := 0
	for _, v := range list(ja["price_items"]) {
		x := object(v)
		id := Str(x["plan_price_id"])
		if id == "" {
			if separateJA {
				invalid++
			}
			continue
		}
		jaByID[id] = x
	}
	result := []Option{}
	for _, v := range list(local["price_items"]) {
		x := object(v)
		id := Str(x["plan_price_id"])
		if id == "" {
			invalid++
			continue
		}
		j := jaByID[id]
		basis, size := classifyBasis(Str(j["price_suffix"]))
		if basis == "unknown" {
			basis, size = classifyBasis(Str(x["price_suffix"]))
		}
		// Only the original Japanese option label establishes an age class.
		// English translations can add "Adult" to an otherwise generic option.
		age, band := ageClass(Str(j["price_prefix"]), Str(j["price_suffix"]))
		base, disc := Int(x["base_price"]), Int(x["discount_price"])
		var headline *int
		if base != nil && disc != nil && *base >= *disc {
			n := *base - *disc
			headline = &n
		}
		result = append(result, Option{OptionID: id, PlanType: Str(x["plan_type"]), NameOriginalJA: Text(j["price_prefix"]), NameLocalized: Text(x["price_prefix"]), UnitTextOriginalJA: Text(j["price_suffix"]), UnitTextLocalized: Text(x["price_suffix"]), BasePriceJPY: base, DiscountJPY: disc, UndatedUnitJPY: headline, Basis: basis, GroupSize: size, AgeBandText: band, AgeClass: age, MinUnits: PositiveInt(x["item_people_min"]), MaxUnits: PositiveInt(x["item_people_max"])})
	}
	return result, invalid
}
func (s *Service) Detail(ctx context.Context, id, lang string) (Plan, error) {
	if err := ValidateID(id); err != nil {
		return Plan{}, err
	}
	if err := ValidLang(lang); err != nil {
		return Plan{}, err
	}
	q := map[string]string{"plan_id": id, "lang_flag": lang, "url": PlanURL(id, lang)}
	local, err := s.get(ctx, "/get_plan_price_info", q)
	if err != nil {
		return Plan{}, err
	}
	p := object(local["plan_data"])
	if Str(p["plan_id"]) != id {
		return Plan{}, &SchemaError{Endpoint: "/get_plan_price_info", Reason: fmt.Sprintf("requested plan %s, returned %q", id, Str(p["plan_id"]))}
	}
	ja := local
	var partial []string
	jaVerified := lang == "ja"
	if lang != "ja" {
		qja := map[string]string{"plan_id": id, "lang_flag": "ja", "url": PlanURL(id, "ja")}
		if v, e := s.get(ctx, "/get_plan_price_info", qja); e == nil && Str(object(v["plan_data"])["plan_id"]) == id {
			ja = v
			jaVerified = true
		} else if e != nil {
			partial = append(partial, "Japanese source detail: "+e.Error())
		} else {
			partial = append(partial, "Japanese source detail: plan identity mismatch")
		}
	}
	jp := object(ja["plan_data"])
	var jaName *string
	if jaVerified {
		jaName = Text(jp["plan_name"])
	}
	operatorID := Text(p["partner_id"])
	jaOptions := ja
	if !jaVerified {
		jaOptions = nil
	} else if _, ok := jaOptions["price_items"].([]any); !ok && lang != "ja" {
		partial = append(partial, "Japanese source detail: missing price_items array")
		jaOptions = nil
	}
	rawOptions, ok := local["price_items"].([]any)
	if !ok {
		return Plan{}, &SchemaError{Endpoint: "/get_plan_price_info", Reason: "missing price_items array"}
	}
	options, invalidOptions := parseOptions(local, jaOptions, lang != "ja" && jaOptions != nil)
	if invalidOptions > 0 {
		if len(options) == 0 && len(rawOptions) > 0 {
			return Plan{}, &SchemaError{Endpoint: "/get_plan_price_info", Reason: fmt.Sprintf("all %d local price options malformed", len(rawOptions))}
		}
		partial = append(partial, fmt.Sprintf("%d malformed detail price option record(s) omitted or untranslated", invalidOptions))
	}
	plan := Plan{PlanID: id, OperatorID: operatorID, NameOriginalJA: jaName, NameLocalized: Text(p["plan_name"]), SiteLanguage: lang, SourceURL: PlanURL(id, lang), URLs: map[string]string{"en": PlanURL(id, "en"), "ja": PlanURL(id, "ja")}, VenueAddress: Text(p["address"]), MeetingPoint: Text(p["basic_meeting_place"]), DurationTotalText: Text(p["necessary_time_comment"]), MeetingTimeText: Text(p["basic_meeting_time"]), AgeMinYears: Int(p["age_start"]), AgeMaxYears: Int(p["age_end"]), PartyMin: PositiveInt(p["people_min"]), PartyMax: PositiveInt(p["people_max"]), PartyMinSource: Int(p["people_min"]), PartyMaxSource: Int(p["people_max"]), BasicMinPassengers: Int(p["basic_min_passenger_count"]), InclusionText: Text(p["include_charge"]), ExclusionText: Text(p["not_include_charge"]), EquipmentToBring: Text(p["clothes_belongings"]), RentalEquipment: Text(p["clothes_rental"]), RestrictionText: Text(p["other_attention"]), AdditionalAttentionText: Text(p["other_comment"]), OperatorMessageText: Text(p["default_message_note"]), CancellationText: Text(p["payment_cancel"]), PaymentText: Text(p["payment_method"]), PeriodStart: safePeriod(p["period_start"]), PeriodEnd: safePeriod(p["period_end"]), LocaleSupportFlag: Bool(p["support_language"]), Options: options, Derived: map[string]any{"source_url_basis": "constructed from plan ID and locale; index membership unverified", "canonical_url_basis": "null until the selected locale is checked against its plan sitemap", "headline_price_method": "plan_data base_price minus discount_price; selected-date price requires experience price", "zero_party_bound": "source zero is treated as unknown, not a traveler limit", "basic_min_passenger_count_source": "unverified semantics; not used as minimum party size"}, ObservedAt: ObservedAt(), Partial: len(partial) > 0, Errors: partial}
	plan.HeadlineBasePriceJPY = Int(p["base_price"])
	plan.DerivedTotalMinutes = ParseExactDuration(plan.DurationTotalText)
	plan.HeadlineDiscountJPY = Int(p["discount_price"])
	if plan.HeadlineBasePriceJPY != nil && plan.HeadlineDiscountJPY != nil && *plan.HeadlineBasePriceJPY >= *plan.HeadlineDiscountJPY {
		n := *plan.HeadlineBasePriceJPY - *plan.HeadlineDiscountJPY
		plan.DerivedHeadlineFromJPY = &n
	}
	plan.Missing = []string{}
	if plan.OperatorNameOriginal == nil {
		plan.Missing = append(plan.Missing, "operator_name_original")
	}
	if plan.PartyMax == nil {
		plan.Missing = append(plan.Missing, "party_max")
	}
	if plan.PickupLocation == nil {
		plan.Missing = append(plan.Missing, "pickup_location")
	}
	if plan.SpokenLanguages == nil {
		plan.Missing = append(plan.Missing, "spoken_languages")
	}
	if plan.WeatherDependence == nil {
		plan.Missing = append(plan.Missing, "weather_dependence")
	}
	if plan.ActivityMinutes == nil {
		plan.Missing = append(plan.Missing, "activity_minutes")
	}
	if plan.CheckInLeadMinutes == nil {
		plan.Missing = append(plan.Missing, "check_in_lead_minutes")
	}
	return plan, nil
}

type Quote struct {
	OptionID             string  `json:"option_id"`
	PlanType             string  `json:"plan_type"`
	SelectedDate         string  `json:"selected_date"`
	PriceJPY             *int    `json:"selected_date_unit_price_jpy"`
	BasePriceJPY         *int    `json:"base_price_jpy"`
	Basis                string  `json:"basis"`
	GroupSize            *int    `json:"group_size"`
	UnitTextOriginalJA   *string `json:"unit_text_original_ja"`
	OptionNameOriginalJA *string `json:"option_name_original_ja"`
	AgeBandText          *string `json:"age_band_text"`
	AgeClass             string  `json:"age_class"`
	MinUnits             *int    `json:"min_units"`
	MaxUnits             *int    `json:"max_units"`
	DependID             string  `json:"depend_id"`
	DerivedSubtotalJPY   *int    `json:"derived_subtotal_jpy"`
	SubtotalNote         *string `json:"subtotal_note"`
}

// Subtotal only prices an unambiguous unit count. Fees and extras remain unknown.
func Subtotal(price *int, basis string, groupSize *int, participants int) *int {
	if price == nil || participants <= 0 || *price < 0 {
		return nil
	}
	n := 0
	switch basis {
	case "per_person":
		n = participants
	case "per_group":
		if groupSize == nil || participants != *groupSize {
			return nil
		}
		n = 1
	default:
		return nil
	}
	if n > 100 || *price > 100000000/n {
		return nil
	}
	total := *price * n
	return &total
}

func stricterMin(a, b *int) *int {
	if a == nil || (b != nil && *b > *a) {
		return b
	}
	return a
}
func stricterMax(a, b *int) *int {
	if a == nil || (b != nil && *b < *a) {
		return b
	}
	return a
}
func subtotalUnits(basis string, groupSize *int, participants int) int {
	if basis == "per_person" {
		return participants
	}
	if basis == "per_group" && groupSize != nil && participants == *groupSize {
		return 1
	}
	return 0
}
func (s *Service) Price(ctx context.Context, id, date string, participants int, options []Option) ([]Quote, error) {
	if err := ValidateID(id); err != nil {
		return nil, err
	}
	if err := ValidateDate(date); err != nil {
		return nil, err
	}
	raw, err := s.get(ctx, "/plan/get_plan_price", map[string]string{"plan_id": id, "date": date})
	if err != nil {
		return nil, err
	}
	types := object(raw["planPriceList"])
	if types == nil {
		return nil, &SchemaError{Endpoint: "/plan/get_plan_price", Reason: "missing planPriceList"}
	}
	keys := make([]string, 0, len(types))
	for k := range types {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	quotes := []Quote{}
	invalid := 0
	optionByID := map[string]Option{}
	for _, option := range options {
		optionByID[option.OptionID] = option
	}
	for _, typ := range keys {
		if _, ok := types[typ].([]any); !ok {
			return nil, &SchemaError{Endpoint: "/plan/get_plan_price", Reason: "planPriceList entry is not an array"}
		}
		for _, v := range list(types[typ]) {
			x := object(v)
			id2 := Str(x["price_id"])
			if id2 == "" {
				invalid++
				continue
			}
			basis, size := classifyBasis(Str(x["price_suffix"]))
			age, band := ageClass(Str(x["price_prefix"]), Str(x["price_suffix"]))
			var option Option
			if matched, ok := optionByID[id2]; ok {
				option = matched
				if age == "unknown" {
					age = matched.AgeClass
					band = matched.AgeBandText
				}
				if basis == "unknown" {
					basis = matched.Basis
					size = matched.GroupSize
				}
			}
			p := Int(x["price"])
			minUnits := stricterMin(PositiveInt(x["price_item_min"]), option.MinUnits)
			maxUnits := stricterMax(PositiveInt(x["price_item_max"]), option.MaxUnits)
			var sub *int
			var note *string
			if age == "adult" || age == "participant" {
				units := subtotalUnits(basis, size, participants)
				if units > 0 && ((minUnits != nil && units < *minUnits) || (maxUnits != nil && units > *maxUnits)) {
					t := "Requested party falls outside source option unit bounds; no subtotal derived"
					note = &t
				} else {
					sub = Subtotal(p, basis, size, participants)
				}
			}
			if sub != nil {
				t := "Derived adult-party subtotal for this option only; excludes unknown fees, extras and traveler age validation"
				note = &t
			}
			quotes = append(quotes, Quote{OptionID: id2, PlanType: typ, SelectedDate: date, PriceJPY: p, BasePriceJPY: Int(x["base_price"]), Basis: basis, GroupSize: size, UnitTextOriginalJA: Text(x["price_suffix"]), OptionNameOriginalJA: Text(x["price_prefix"]), AgeBandText: band, AgeClass: age, MinUnits: minUnits, MaxUnits: maxUnits, DependID: Str(x["depend_id"]), DerivedSubtotalJPY: sub, SubtotalNote: note})
		}
	}
	if invalid > 0 {
		if len(quotes) == 0 {
			return nil, &SchemaError{Endpoint: "/plan/get_plan_price", Reason: fmt.Sprintf("all %d source items malformed", invalid)}
		}
		return quotes, &PartialItemsError{Endpoint: "/plan/get_plan_price", Count: invalid}
	}
	return quotes, nil
}

type Session struct {
	SessionID    string `json:"session_id"`
	StartLocal   string `json:"start_local"`
	SourceTime   string `json:"source_time"`
	SourceStatus string `json:"source_status"`
	Availability string `json:"availability"`
	ObservedAt   string `json:"observed_at"`
}

func Status(code string) string {
	switch code {
	case "1":
		return "instant_confirmable"
	case "3":
		return "reservation_request"
	case "4":
		return "closed"
	case "5":
		return "not_accepted"
	default:
		return "unknown"
	}
}
func (s *Service) Sessions(ctx context.Context, id, date string) ([]Session, error) {
	if err := ValidateID(id); err != nil {
		return nil, err
	}
	if err := ValidateDate(date); err != nil {
		return nil, err
	}
	raw, err := s.get(ctx, "/select_plan_course", map[string]string{"plan_id": id, "selected_date": date})
	if err != nil {
		return nil, err
	}
	names := object(raw["course_name"])
	statuses := object(raw["course_status"])
	if names == nil || statuses == nil {
		return nil, &SchemaError{Endpoint: "/select_plan_course", Reason: "missing course_name or course_status"}
	}
	times := make([]string, 0, len(names))
	for k := range names {
		times = append(times, k)
	}
	sort.Strings(times)
	out := make([]Session, 0, len(times))
	invalid := 0
	observed := ObservedAt()
	for _, t := range times {
		id2 := Str(names[t])
		if id2 == "" {
			invalid++
			continue
		}
		clock, e := time.ParseInLocation("2006-01-02 15:04", date+" "+t, tokyo)
		if e != nil {
			invalid++
			continue
		}
		code := Str(statuses[id2])
		out = append(out, Session{SessionID: id2, StartLocal: clock.Format(time.RFC3339), SourceTime: t, SourceStatus: code, Availability: Status(code), ObservedAt: observed})
	}
	if invalid > 0 {
		if len(out) == 0 {
			return nil, &SchemaError{Endpoint: "/select_plan_course", Reason: fmt.Sprintf("all %d source items malformed", invalid)}
		}
		return out, &PartialItemsError{Endpoint: "/select_plan_course", Count: invalid}
	}
	return out, nil
}

type Check struct {
	PlanID               string `json:"plan_id"`
	SessionID            string `json:"session_id"`
	Date                 string `json:"date"`
	Participants         int    `json:"participants"`
	SourceResult         string `json:"source_result"`
	SourceStock          *int   `json:"source_stock"`
	Availability         string `json:"availability"`
	ObservedAt           string `json:"observed_at"`
	ReservationConfirmed bool   `json:"reservation_confirmed"`
}

func CheckAvailability(initial string, result string) string {
	switch result {
	case "1":
		return Status(initial)
	case "2":
		return "instant_confirmable"
	case "3":
		return "reservation_request"
	case "4":
		return "closed"
	case "5":
		return "sold_out_for_party"
	default:
		return "unknown"
	}
}
func (s *Service) Check(ctx context.Context, id, date, sessionID string, participants int) (Check, error) {
	if participants < 1 || participants > 50 {
		return Check{}, fmt.Errorf("participants must be 1..50 for bounded stock checks")
	}
	plan, err := s.Detail(ctx, id, "ja")
	if err != nil {
		return Check{}, err
	}
	if len(plan.Options) == 0 {
		return Check{}, errors.New("stock quantity basis is unknown without price options")
	}
	for _, option := range plan.Options {
		if option.Basis != "per_person" {
			return Check{}, errors.New("stock quantity basis is unverified for group or unknown-unit options; use experience sessions and handoff")
		}
	}
	min := plan.PartyMin
	if min != nil && participants < *min {
		return Check{}, fmt.Errorf("plan %s requires at least %d participants", id, *min)
	}
	if plan.PartyMax != nil && participants > *plan.PartyMax {
		return Check{}, fmt.Errorf("plan %s allows at most %d participants", id, *plan.PartyMax)
	}
	sessions, err := s.Sessions(ctx, id, date)
	if err != nil {
		return Check{}, err
	}
	var selected *Session
	for i := range sessions {
		if sessions[i].SessionID == sessionID {
			selected = &sessions[i]
			break
		}
	}
	if selected == nil {
		return Check{}, fmt.Errorf("session ID %s was not returned for plan %s on %s", sessionID, id, date)
	}
	if selected.SourceStatus != "1" && selected.SourceStatus != "3" {
		return Check{PlanID: id, SessionID: sessionID, Date: date, Participants: participants, SourceResult: "", Availability: selected.Availability, ObservedAt: selected.ObservedAt}, nil
	}
	raw, err := s.get(ctx, "/plan/check_calendar_data", map[string]string{"plan_id": id, "c_id": sessionID, "date": date, "status": selected.SourceStatus, "count": strconv.Itoa(participants), "type": "2"})
	if err != nil {
		return Check{}, err
	}
	result := Str(raw["result"])
	if result == "" {
		return Check{}, errors.New("Activity Japan stock schema: missing result")
	}
	return Check{PlanID: id, SessionID: sessionID, Date: date, Participants: participants, SourceResult: result, SourceStock: Int(raw["stock"]), Availability: CheckAvailability(selected.SourceStatus, result), ObservedAt: ObservedAt()}, nil
}
func URLFromPlanInput(v string) (string, error) {
	if digits.MatchString(v) {
		return v, ValidateID(v)
	}
	parsed, err := url.Parse(v)
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "https" || (parsed.Host != "activityjapan.com" && parsed.Host != "en.activityjapan.com") {
		return "", fmt.Errorf("use an Activity Japan plan URL")
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 3 || parts[0] != "publish" || parts[1] != "plan" {
		return "", fmt.Errorf("use a /publish/plan/<id> URL")
	}
	return parts[2], ValidateID(parts[2])
}
