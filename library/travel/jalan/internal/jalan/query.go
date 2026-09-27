package jalan

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// Query describes identical occupancy in every requested room. Children are,
// in order, elementary, infant meals+bed, infant meals, infant bed, infant neither.
type Query struct {
	Destination     string `json:"destination,omitempty"`
	AreaCode        string `json:"area_code,omitempty"`
	CheckIn         string `json:"check_in"`
	Nights          int    `json:"nights"`
	Rooms           int    `json:"rooms"`
	Adults          int    `json:"adults_per_room"`
	Children        [5]int `json:"children_per_room"`
	Meals           string `json:"meals,omitempty"`
	LodgingType     string `json:"lodging_type,omitempty"`
	Onsen           bool   `json:"onsen"`
	OutdoorBath     bool   `json:"outdoor_bath"`
	PrivateBath     bool   `json:"private_bath"`
	RoomOutdoorBath bool   `json:"room_outdoor_bath"`
	NonSmoking      bool   `json:"non_smoking"`
	Page            int    `json:"page"`
	Limit           int    `json:"limit"`
}

type PlanRef struct {
	PlanID string `json:"plan_id"`
	RoomID string `json:"room_id"`
}

var tokyo = time.FixedZone("Asia/Tokyo", 9*60*60)
var idPattern = regexp.MustCompile(`^[0-9]+$`)

// ValidateQuery performs the same no-I/O preflight used by inventory commands.
// The date window is a conservative client bound, not a source inventory promise.
func ValidateQuery(q Query, requireDestination bool) (Query, error) {
	return normalizeQuery(q, requireDestination, time.Now())
}

func normalizeQuery(q Query, requireDestination bool, now time.Time) (Query, error) {
	if q.Nights == 0 {
		q.Nights = 1
	}
	if q.Rooms == 0 {
		q.Rooms = 1
	}
	if q.Adults == 0 {
		q.Adults = 2
	}
	if q.Page == 0 {
		q.Page = 1
	}
	if q.Limit == 0 {
		q.Limit = 5
	}
	q.Destination = strings.TrimSpace(q.Destination)
	q.AreaCode = strings.TrimSpace(q.AreaCode)
	if q.CheckIn == "" {
		return q, usage("check_in_required", "check-in is required for dated inventory", "Pass --check-in YYYY-MM-DD.")
	}
	date, err := time.ParseInLocation("2006-01-02", q.CheckIn, tokyo)
	if err != nil || date.Format("2006-01-02") != q.CheckIn {
		return q, usage("invalid_query", "check-in must be a valid YYYY-MM-DD calendar date", "Pass --check-in YYYY-MM-DD.")
	}
	today := now.In(tokyo)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, tokyo)
	if date.Before(today) || date.After(today.AddDate(0, 0, 365)) {
		return q, usage("invalid_query", "check-in must be today or within the next 365 days in Asia/Tokyo", "Choose a date within the client search window; source inventory may cover less.")
	}
	if q.Nights < 1 || q.Nights > 9 {
		return q, usage("invalid_query", "nights must be between 1 and 9", "Pass --nights 1..9.")
	}
	if q.Rooms < 1 || q.Rooms > 10 {
		return q, usage("invalid_query", "rooms must be between 1 and 10", "Pass --rooms 1..10; each room has the same occupancy.")
	}
	if q.Adults < 1 || q.Adults > 8 {
		return q, usage("unsupported", "adults per room must be between 1 and 8", "The source value 9 is a nine-or-more bucket, not an exact party count; use --adults 1..8.")
	}
	for _, child := range q.Children {
		if child < 0 || child > 5 {
			return q, usage("unsupported", "each child category per room must be between 0 and 5", "Use the five explicit child category flags with counts 0..5.")
		}
	}
	if q.Page < 1 || q.Page > 10000 {
		return q, usage("invalid_query", "page must be between 1 and 10000", "Pass --page 1..10000.")
	}
	if q.Limit < 1 || q.Limit > 30 {
		return q, usage("invalid_query", "limit must be between 1 and 30", "Pass --limit 1..30.")
	}
	switch strings.ToLower(strings.TrimSpace(q.Meals)) {
	case "", "any":
		q.Meals = ""
	case "none", "breakfast", "dinner", "breakfast_dinner":
		q.Meals = strings.ToLower(strings.TrimSpace(q.Meals))
	case "both":
		q.Meals = "breakfast_dinner"
	default:
		return q, usage("unsupported", "unsupported meals filter", "Use none, breakfast, dinner, or breakfast_dinner.")
	}
	switch strings.ToLower(strings.TrimSpace(q.LodgingType)) {
	case "", "any":
		q.LodgingType = ""
	case "hotel", "ryokan", "pension", "vacation_rental", "public_lodging":
		q.LodgingType = strings.ToLower(strings.TrimSpace(q.LodgingType))
	default:
		return q, usage("unsupported", "unsupported lodging type", "Use hotel, ryokan, pension, vacation_rental, or public_lodging.")
	}
	if q.AreaCode != "" && strings.HasSuffix(q.AreaCode, "0000") {
		known := false
		for _, location := range locationCatalogue {
			if location.Kind == "prefecture" && location.AreaCode == q.AreaCode {
				known = true
				break
			}
		}
		if !known {
			return q, usage("unsupported", "prefecture code is not in the verified source catalogue", "Use stay locations to choose a supported prefecture or an explicit large-area code.")
		}
	}
	if q.AreaCode != "" && (!idPattern.MatchString(q.AreaCode) || len(q.AreaCode) != 6) {
		return q, usage("invalid_query", "area-code must be a six-digit source large-area or prefecture code", "Use stay locations, or an explicit Jalan large-area/prefecture code.")
	}
	if requireDestination {
		if q.Destination != "" && q.AreaCode != "" {
			return q, usage("invalid_query", "destination and area-code are mutually exclusive", "Pass either --destination or --area-code.")
		}
		if q.Destination != "" {
			location, err := resolveLocation(q.Destination)
			if err != nil {
				return q, err
			}
			q.AreaCode = location.AreaCode
		} else if q.AreaCode == "" {
			return q, usage("destination_required", "a destination or area-code is required", "Use stay locations to inspect supported aliases, then pass --destination or --area-code.")
		}
	} else if q.Destination != "" || q.AreaCode != "" {
		return q, usage("unsupported", "a property ID already fixes the destination", "Omit --destination and --area-code for property offers or exact plans.")
	}
	return q, nil
}

func validateID(value, kind string) error {
	length := map[string]int{"property": 6, "plan": 8, "room": 7}[kind]
	if !idPattern.MatchString(value) || len(value) != length {
		return usage("invalid_query", fmt.Sprintf("%s must be a %d-digit source ID", kind, length), "Copy the exact ID from a search, offer, or plan result.")
	}
	return nil
}

func (q Query) values() url.Values {
	date, _ := time.ParseInLocation("2006-01-02", q.CheckIn, tokyo)
	v := url.Values{"stayYear": {strconv.Itoa(date.Year())}, "stayMonth": {strconv.Itoa(int(date.Month()))}, "stayDay": {strconv.Itoa(date.Day())}, "stayCount": {strconv.Itoa(q.Nights)}, "roomCount": {strconv.Itoa(q.Rooms)}, "adultNum": {strconv.Itoa(q.Adults)}, "distCd": {"01"}}
	occupancy := strconv.Itoa(q.Adults)
	for i, child := range q.Children {
		occupancy += strconv.Itoa(child)
		if child > 0 {
			v.Set(fmt.Sprintf("child%dNum", i+1), strconv.Itoa(child))
		}
	}
	rooms := make([]string, q.Rooms)
	for i := range rooms {
		rooms[i] = occupancy
	}
	v.Set("roomCrack", strings.Join(rooms, ","))
	if q.AreaCode != "" {
		if strings.HasSuffix(q.AreaCode, "0000") {
			v.Set("kenCd", q.AreaCode)
		} else {
			v.Set("lrgCd", q.AreaCode)
		}
	}
	meals := map[string]string{"none": "0", "breakfast": "1", "dinner": "2", "breakfast_dinner": "3"}
	if value, ok := meals[q.Meals]; ok {
		v.Set("mealType", value)
	}
	types := map[string]string{"hotel": "yadHb", "ryokan": "yadRk", "pension": "yadPm", "vacation_rental": "yadKc", "public_lodging": "yadKy"}
	if value, ok := types[q.LodgingType]; ok {
		v.Set(value, "1")
	}
	for name, enabled := range map[string]bool{"careOnsen": q.Onsen, "careOpenbath": q.OutdoorBath, "careBathRent": q.PrivateBath, "carePribateBath": q.RoomOutdoorBath, "careNsmr": q.NonSmoking} {
		if enabled {
			v.Set(name, "1")
		}
	}
	return v
}

func validateOfferFilters(q Query) error {
	if q.LodgingType != "" || q.Onsen || q.OutdoorBath || q.PrivateBath {
		return usage("unsupported", "property offers do not support lodging type or property bath facility filters", "Use stay search for lodging-type, onsen, outdoor-bath, or private-bath filters; offers support meals, non-smoking and room-outdoor-bath.")
	}
	return nil
}
func validatePlanFilters(q Query) error {
	if q.Meals != "" || q.LodgingType != "" || q.Onsen || q.OutdoorBath || q.PrivateBath || q.RoomOutdoorBath || q.NonSmoking {
		return usage("unsupported", "exact plan inspection does not apply search filters", "Choose the exact plan/room pair from filtered search or offers, then omit filters when inspecting its terms.")
	}
	return nil
}

// ValidateOffersQuery and ValidatePlanQuery add command-specific no-I/O checks.
func ValidateOffersQuery(q Query) (Query, error) {
	q, err := ValidateQuery(q, false)
	if err != nil {
		return q, err
	}
	return q, validateOfferFilters(q)
}
func ValidatePlanQuery(q Query) (Query, error) {
	q, err := ValidateQuery(q, false)
	if err != nil {
		return q, err
	}
	return q, validatePlanFilters(q)
}

// verifyQueryEcho rejects a source fallback that dropped dated occupancy or an
// active native filter. Only hidden or checked controls count as echoed input.
func verifyQueryEcho(doc string, q Query, sourceURL string) error {
	values := map[string]map[string]bool{}
	tokenizer := html.NewTokenizer(strings.NewReader(doc))
	for {
		tokenType := tokenizer.Next()
		if tokenType == html.ErrorToken {
			break
		}
		if tokenType != html.StartTagToken && tokenType != html.SelfClosingTagToken {
			continue
		}
		token := tokenizer.Token()
		if token.Data != "input" {
			continue
		}
		attrs := map[string]string{}
		for _, attr := range token.Attr {
			attrs[attr.Key] = attr.Val
		}
		name, value := attrs["name"], attrs["value"]
		if name == "" {
			continue
		}
		if attrs["type"] == "checkbox" || attrs["type"] == "radio" {
			if _, checked := attrs["checked"]; !checked {
				continue
			}
		}
		if values[name] == nil {
			values[name] = map[string]bool{}
		}
		values[name][value] = true
	}
	for name, expected := range q.values() {
		if name == "distCd" {
			continue
		}
		if !values[name][expected[0]] {
			return &Error{Code: "unsupported", Message: "source did not preserve requested " + name + " in its dated search controls", Hint: "Open the source URL to inspect source occupancy or filter restrictions; this response is not a verified match for the request.", URL: sourceURL}
		}
	}
	return nil
}
