package pocket

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type Label struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	NameJA *string `json:"name_ja"`
}
type PriceRange struct {
	Min         *int   `json:"min"`
	Max         *int   `json:"max"`
	ServiceType string `json:"serviceType"`
}
type Venue struct {
	ID                  string           `json:"id"`
	Name                string           `json:"name"`
	RealTimeBooking     *bool            `json:"realTimeBooking"`
	Area                *Label           `json:"area"`
	Cuisines            []Label          `json:"cuisines"`
	Blurb               *string          `json:"blurb"`
	PriceRanges         []PriceRange     `json:"priceRanges"`
	LocalizedAddress    *string          `json:"localizedAddress"`
	AddressHidden       *bool            `json:"addressHidden"`
	BusinessHours       *string          `json:"businessHours"`
	Holidays            *string          `json:"holidays"`
	LongDescription     *string          `json:"longDescription"`
	NearestStations     []string         `json:"nearestStations"`
	ReservationTerms    *string          `json:"reservationTerms"`
	Services            []string         `json:"services"`
	TransactionsAllowed *bool            `json:"transactionsAllowed"`
	Courses             []Course         `json:"courses"`
	FAQ                 []map[string]any `json:"frequentlyAskedQuestions"`
	Calendar            *Calendar        `json:"availabilityCalendar"`
}
type Course struct {
	ID                       string  `json:"id"`
	Name                     string  `json:"name"`
	FixedPrice               *int    `json:"fixedPrice"`
	FixedTitle               *string `json:"fixedTitle"`
	CostPerGuest             *int    `json:"costPerGuest"`
	ServiceType              string  `json:"serviceType"`
	Summary                  *string `json:"summary"`
	SupplementaryInformation *string `json:"supplementaryInformation"`
}
type Calendar struct {
	ReservationDates []string `json:"reservationDates"`
	WaitlistDates    []string `json:"waitlistDates"`
}
type Slot struct {
	Type         string  `json:"__typename"`
	ID           *string `json:"id"`
	StartTime    string  `json:"startTime"`
	EndTime      *string `json:"endTime"`
	SeatingType  *string `json:"seatingType"`
	MinPartySize *int    `json:"minPartySize"`
	MaxPartySize *int    `json:"maxPartySize"`
	Course       *Course `json:"course"`
}

var digits = regexp.MustCompile(`^[0-9]{1,12}$`)

func ValidateID(id string) error {
	if !digits.MatchString(id) {
		return Fail("usage", "--id/course/session must be a numeric source ID (1–12 digits)")
	}
	return nil
}
func ValidateDate(s string) error {
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return Fail("usage", "--date must be a real date in YYYY-MM-DD (Japan time)")
	}
	return nil
}
func BookingMode(b *bool) any {
	if b == nil {
		return nil
	}
	if *b {
		return "instant_confirmation"
	}
	return "reservation_request"
}
func URL(id, lang string) string {
	prefix := ""
	if lang == "en" {
		prefix = "/en"
	}
	return "https://www.pocket-concierge.jp" + prefix + "/restaurants/" + id
}
func ptr(s string) *string { return &s }
func Summary(v Venue, jp *Venue, lang string) map[string]any {
	var ja *string
	if lang == "ja" {
		ja = ptr(v.Name)
	} else if jp != nil {
		ja = ptr(jp.Name)
	}
	var prices []map[string]any
	if v.PriceRanges != nil {
		prices = []map[string]any{}
	}
	for _, p := range v.PriceRanges {
		prices = append(prices, map[string]any{"min": p.Min, "max": p.Max, "service_type": p.ServiceType, "currency": "JPY", "unit": "per_guest", "inclusions": nil})
	}
	var cuisines []Label
	if v.Cuisines != nil {
		cuisines = []Label{}
	}
	for _, c := range v.Cuisines {
		if lang == "ja" {
			c.NameJA = ptr(c.Name)
		} else if jp != nil {
			for _, x := range jp.Cuisines {
				if x.ID == c.ID {
					c.NameJA = ptr(x.Name)
				}
			}
		}
		cuisines = append(cuisines, c)
	}
	var area *Label
	if v.Area != nil {
		x := *v.Area
		if lang == "ja" {
			x.NameJA = ptr(x.Name)
		} else if jp != nil && jp.Area != nil && jp.Area.ID == x.ID {
			x.NameJA = ptr(jp.Area.Name)
		}
		area = &x
	}
	return map[string]any{"id": v.ID, "name": v.Name, "name_ja": ja, "url": URL(v.ID, lang), "area": area, "cuisines": cuisines, "blurb": v.Blurb, "price_ranges": prices, "booking_mode": BookingMode(v.RealTimeBooking), "real_time_booking": v.RealTimeBooking}
}
func FeeStatements(scope string, fields map[string]*string) []map[string]string {
	result := []map[string]string{}
	for _, field := range []string{"summary", "supplementary_information", "reservation_terms"} {
		text := fields[field]
		if text == nil {
			continue
		}
		for _, line := range strings.Split(*text, "\n") {
			line = strings.TrimSpace(line)
			lower := strings.ToLower(line)
			if strings.Contains(lower, "tax") || strings.Contains(lower, "service charge") || strings.Contains(lower, "additional") || strings.Contains(line, "税込") || strings.Contains(line, "税金") || strings.Contains(line, "サービス料") || strings.Contains(line, "別途") {
				result = append(result, map[string]string{"scope": scope, "field": field, "text": line})
			}
		}
	}
	return result
}
func CourseView(c Course, jp *Course, venueID, lang string) map[string]any {
	var ja *string
	if lang == "ja" {
		ja = ptr(c.Name)
	} else if jp != nil {
		ja = ptr(jp.Name)
	}
	return map[string]any{"id": c.ID, "restaurant_id": venueID, "name": c.Name, "name_ja": ja, "service_type": c.ServiceType, "price": map[string]any{"currency": "JPY", "per_guest": c.CostPerGuest, "fixed_per_group": c.FixedPrice, "fixed_title": c.FixedTitle, "all_in_total": nil}, "summary": c.Summary, "supplementary_information": c.SupplementaryInformation, "fee_statements": FeeStatements("course", map[string]*string{"summary": c.Summary, "supplementary_information": c.SupplementaryInformation}), "url": URL(venueID, lang)}
}
func PartyEligible(s Slot, party int) (bool, error) {
	if s.MinPartySize != nil && *s.MinPartySize < 1 || s.MaxPartySize != nil && *s.MaxPartySize < 1 || s.MinPartySize != nil && s.MaxPartySize != nil && *s.MaxPartySize < *s.MinPartySize {
		return false, Fail("source_schema", "invalid session party-size bounds")
	}
	if party == 0 {
		return true, nil
	}
	if s.MinPartySize != nil && party < *s.MinPartySize || s.MaxPartySize != nil && party > *s.MaxPartySize {
		return false, nil
	}
	return true, nil
}

func SlotView(s Slot, venue Venue, jp *Course, party int, lang string) (map[string]any, error) {
	if s.Course == nil || s.Course.ID == "" || s.StartTime == "" {
		return nil, Fail("source_schema", "session missing course identity or start time")
	}
	if _, err := time.Parse(time.RFC3339, s.StartTime); err != nil {
		return nil, Fail("source_schema", "session start time is not an offset timestamp")
	}
	var status any
	switch s.Type {
	case "ReservableAvailability":
		if s.ID == nil || *s.ID == "" {
			return nil, Fail("source_schema", "reservable session has no ID")
		}
		status = BookingMode(venue.RealTimeBooking)
	case "WaitlistableAvailability":
		status = "waitlist"
	default:
		return nil, Fail("source_schema", "unknown availability type: "+s.Type)
	}
	var eligible any
	if s.MinPartySize != nil && s.MaxPartySize != nil && party > 0 {
		eligible, _ = PartyEligible(s, party)
	}
	return map[string]any{"session_id": s.ID, "source_type": s.Type, "restaurant_id": venue.ID, "course_id": s.Course.ID, "course": CourseView(*s.Course, jp, venue.ID, lang), "status": status, "confirmed_reservation": false, "start_time": s.StartTime, "end_time": s.EndTime, "timezone": "Asia/Tokyo", "seating_type": s.SeatingType, "min_party_size": s.MinPartySize, "max_party_size": s.MaxPartySize, "party_eligible": eligible, "url": URL(venue.ID, lang)}, nil
}
func MatchCourse(v *Venue, id string) *Course {
	if v != nil {
		for i := range v.Courses {
			if v.Courses[i].ID == id {
				return &v.Courses[i]
			}
		}
	}
	return nil
}
func ValidateVenue(v *Venue, id string) error {
	if v == nil {
		return Fail("not_found", "restaurant not found in public Pocket Concierge inventory")
	}
	if v.ID != id || v.Name == "" {
		return Fail("source_schema", fmt.Sprintf("restaurant identity mismatch for %s", id))
	}
	return nil
}
func (s Slot) CourseID() string {
	if s.Course == nil {
		return ""
	}
	return s.Course.ID
}

// ConditionEvidence returns source statements, not inferred eligibility.
func ConditionEvidence(terms *string, services []string) map[string]any {
	r := map[string]any{"dietary": nil, "children": nil, "language": nil}
	termsByKind := map[string][]map[string]string{"dietary": {}, "children": {}, "language": {}}
	if terms != nil {
		for _, line := range strings.Split(*terms, "\n") {
			line = strings.TrimSpace(line)
			low := strings.ToLower(line)
			for kind, keywords := range map[string][]string{"dietary": {"allerg", "diet", "vegetarian", "vegan", "food preference", "アレルギー", "苦手", "食事制限"}, "children": {"child", "aged ", "years old", "お子様", "お子さま", "子供", "歳以上", "才以上"}, "language": {"english", "language", "英語", "通訳"}} {
				for _, keyword := range keywords {
					if strings.Contains(low, keyword) {
						termsByKind[kind] = append(termsByKind[kind], map[string]string{"field": "reservation_terms", "text": line})
						break
					}
				}
			}
		}
	}
	for _, service := range services {
		if strings.Contains(service, "english") || strings.Contains(service, "language") {
			termsByKind["language"] = append(termsByKind["language"], map[string]string{"field": "source_services", "text": service})
		}
	}
	for kind, statements := range termsByKind {
		if len(statements) > 0 {
			r[kind] = statements
		}
	}
	r["interpretation"] = "Verbatim source evidence only; do not infer guaranteed accommodation, child eligibility or staff language."
	return r
}
