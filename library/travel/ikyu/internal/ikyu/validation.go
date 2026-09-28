package ikyu

import (
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"time"
)

type InputError struct{ Message string }

func (e *InputError) Error() string { return e.Message }
func invalidf(format string, args ...any) error {
	return &InputError{Message: fmt.Sprintf(format, args...)}
}

var idPattern = regexp.MustCompile(`^\d{8}$`)
var japan = time.FixedZone("JST", 9*60*60)

func ValidateStay(s Stay, now time.Time) error {
	in, err := time.ParseInLocation("2006-01-02", s.CheckIn, japan)
	if err != nil {
		return invalidf("--check-in must be an exact YYYY-MM-DD calendar date")
	}
	out, err := time.ParseInLocation("2006-01-02", s.CheckOut, japan)
	if err != nil {
		return invalidf("--check-out must be an exact YYYY-MM-DD calendar date")
	}
	today := now.In(japan)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, japan)
	if in.Before(today) {
		return invalidf("--check-in cannot be before today in Japan")
	}
	if in.After(today.AddDate(0, 0, 365)) {
		return invalidf("--check-in cannot be more than 365 days after today in Japan")
	}
	n := int(out.Sub(in) / (24 * time.Hour))
	if n < 1 || n > 30 {
		return invalidf("stay must be 1–30 nights with check-out after check-in")
	}
	if s.Adults < 1 || s.Adults > 10 {
		return invalidf("--adults must be 1–10 adults per room")
	}
	if s.Rooms < 1 || s.Rooms > 10 {
		return invalidf("--rooms must be 1–10 with identical occupancy in every room")
	}
	for i, n := range s.Children {
		if n < 0 || n > 10 {
			return invalidf("child category %c must be 0–10 per room", 'A'+i)
		}
	}
	return nil
}
func nights(s Stay) int {
	a, _ := time.ParseInLocation("2006-01-02", s.CheckIn, japan)
	b, _ := time.ParseInLocation("2006-01-02", s.CheckOut, japan)
	return int(b.Sub(a) / (24 * time.Hour))
}
func validateID(label, id string) error {
	if !idPattern.MatchString(id) {
		return invalidf("%s must be an eight-digit Ikyu ID", label)
	}
	return nil
}
func bounds(limit, offset int) error {
	if limit < 1 || limit > 50 {
		return invalidf("--limit must be 1–50")
	}
	if offset < 0 || offset > 10000 {
		return invalidf("--offset must be 0–10000")
	}
	return nil
}
func CanonicalURL(propertyID, roomID, planID string, stay *Stay) (string, error) {
	if err := validateID("property ID", propertyID); err != nil {
		return "", err
	}
	path := "/" + propertyID + "/"
	q := url.Values{}
	if roomID != "" {
		if err := validateID("room ID", roomID); err != nil {
			return "", err
		}
		if planID != "" {
			if err := validateID("plan ID", planID); err != nil {
				return "", err
			}
			path += planID + "/" + roomID + "/"
		} else {
			q.Set("rm", roomID)
			q.Set("top", "room")
		}
	} else if planID != "" {
		return "", invalidf("plan ID requires room ID")
	}
	if stay != nil {
		parsed, err := time.ParseInLocation("2006-01-02", stay.CheckIn, japan)
		if err != nil {
			return "", invalidf("invalid stay check-in")
		}
		if err := ValidateStay(*stay, parsed); err != nil {
			return "", err
		}
		q.Set("cid", dateCompact(stay.CheckIn))
		q.Set("cod", dateCompact(stay.CheckOut))
		q.Set("lc", strconv.Itoa(nights(*stay)))
		q.Set("ppc", strconv.Itoa(stay.Adults))
		q.Set("rc", strconv.Itoa(stay.Rooms))
		keys := []string{"cac", "cbc", "ccc", "cdc", "cec", "cfc"}
		for i, n := range stay.Children {
			if n > 0 {
				q.Set(keys[i], strconv.Itoa(n))
			}
		}
	}
	u := url.URL{Scheme: "https", Host: "www.ikyu.com", Path: path, RawQuery: q.Encode()}
	return u.String(), nil
}
func dateCompact(s string) string { a, _ := time.Parse("2006-01-02", s); return a.Format("20060102") }
func inputFor(s Stay, p Preferences) map[string]any {
	m := map[string]any{"checkInDate": s.CheckIn, "lodgingCount": nights(s), "peopleCount": s.Adults, "roomCount": s.Rooms, "discount": true, "sortItem": "1", "sortOrder": "1", "sortAmountTarget": "USE", "currency": "JPY"}
	keys := []string{"childACount", "childBCount", "childCCount", "childDCount", "childECount", "childFCount"}
	for i, n := range s.Children {
		m[keys[i]] = n
	}
	var attrs []string
	if p.OutdoorBath {
		attrs = append(attrs, "18")
	}
	if p.HotSpringBath {
		attrs = append(attrs, "16")
	}
	if p.Nonsmoking {
		attrs = append(attrs, "20")
	}
	if len(attrs) > 0 {
		m["roomAttributes"] = attrs
	}
	if len(p.Meals) > 0 {
		m["meals"] = p.Meals
	}
	return m
}
func validatePreferences(p Preferences) error {
	if p.MinBudget != nil && *p.MinBudget < 0 || p.MaxBudget != nil && *p.MaxBudget < 0 {
		return invalidf("budget must be nonnegative integer JPY")
	}
	if p.MinBudget != nil && p.MaxBudget != nil && *p.MinBudget > *p.MaxBudget {
		return invalidf("minimum budget must not exceed maximum budget")
	}
	if p.MinSizeM2 != nil && (math.IsNaN(*p.MinSizeM2) || math.IsInf(*p.MinSizeM2, 0) || *p.MinSizeM2 <= 0) {
		return invalidf("minimum room size must be positive")
	}
	for _, m := range p.Meals {
		if !regexp.MustCompile(`^00[0-7]$`).MatchString(m) {
			return invalidf("meal code must be 000–007")
		}
	}
	return nil
}
