package traveloka

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var marketPattern = regexp.MustCompile(`^[A-Z]{2}$`)
var localePattern = regexp.MustCompile(`^[a-z]{2}-[A-Z]{2}$`)
var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
var airportPattern = regexp.MustCompile(`^[A-Z]{3}$`)

func ValidateShopper(s Shopper) error {
	if !marketPattern.MatchString(s.Market) || !localePattern.MatchString(s.Locale) || !currencyPattern.MatchString(s.Currency) || s.Locale[3:] != s.Market {
		return apiError("INVALID_INPUT", "market, locale and currency must use matching forms SG, en-SG and SGD", 0, false)
	}
	return nil
}
func ValidateQuery(q Query) error { return ValidateQueryAt(q, time.Now()) }
func ValidateQueryAt(q Query, now time.Time) error {
	if err := ValidateShopper(q.Shopper()); err != nil {
		return err
	}
	if q.Adults <= 0 || q.Children < 0 || q.Infants < 0 || q.Limit < 0 || q.Offset < 0 || q.MaxCandidates < 0 {
		return apiError("INVALID_INPUT", "adults must be positive; passenger counts and output caps must be nonnegative", 0, false)
	}
	switch q.Kind {
	case "flight", "flights":
		if !airportPattern.MatchString(q.Origin) || !airportPattern.MatchString(q.Destination) || q.Origin == q.Destination {
			return apiError("INVALID_INPUT", "distinct three-letter source airport codes are required", 0, false)
		}
		switch q.Cabin {
		case "ECONOMY", "PREMIUM_ECONOMY", "BUSINESS", "FIRST":
		default:
			return apiError("UNSUPPORTED_OPERATION", "unsupported cabin", 0, false)
		}
		depart, err := futureDate(q.Depart, now)
		if err != nil {
			return err
		}
		if q.ReturnDate != "" {
			ret, err := futureDate(q.ReturnDate, now)
			if err != nil {
				return err
			}
			if ret.Before(depart) {
				return apiError("INVALID_INPUT", "return date must not precede departure", 0, false)
			}
		}
	case "hotel", "hotels", "rooms":
		if q.Rooms <= 0 {
			return apiError("INVALID_INPUT", "rooms must be positive", 0, false)
		}
		if q.PropertyID == "" && q.GeoID == "" {
			return apiError("INVALID_INPUT", "a source property or geographic ID is required", 0, false)
		}
		if len(q.ChildAges) != q.Children {
			return apiError("INVALID_INPUT", "one child age is required for every child", 0, false)
		}
		for _, age := range q.ChildAges {
			if age < 0 {
				return apiError("INVALID_INPUT", "child ages must be nonnegative", 0, false)
			}
		}
		in, err := futureDate(q.CheckIn, now)
		if err != nil {
			return err
		}
		out, err := futureDate(q.CheckOut, now)
		if err != nil {
			return err
		}
		if !out.After(in) {
			return apiError("INVALID_INPUT", "check-out must follow check-in", 0, false)
		}
	default:
		return apiError("UNSUPPORTED_OPERATION", "unsupported query kind", 0, false)
	}
	return nil
}
func futureDate(value string, now time.Time) (time.Time, error) {
	d, err := time.Parse("2006-01-02", value)
	if err != nil || d.Format("2006-01-02") != value {
		return time.Time{}, apiError("INVALID_INPUT", "travel dates must be valid ISO calendar dates", 0, false)
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if d.Before(today) {
		return time.Time{}, apiError("INVALID_INPUT", "travel dates must be today or in the future", 0, false)
	}
	return d, nil
}
func FlightURL(q Query) (string, error) {
	if err := ValidateQuery(q); err != nil {
		return "", err
	}
	if q.Kind != "flight" && q.Kind != "flights" {
		return "", apiError("INVALID_INPUT", "flight query required", 0, false)
	}
	dep, _ := time.Parse("2006-01-02", q.Depart)
	ret := "NA"
	path := "/flight/fullsearch"
	if q.ReturnDate != "" {
		d, _ := time.Parse("2006-01-02", q.ReturnDate)
		ret = d.Format("02-01-2006")
		path = "/flight/fulltwosearch"
	}
	args := url.Values{"ap": {q.Origin + "." + q.Destination}, "dt": {dep.Format("02-01-2006") + "." + ret}, "ps": {fmt.Sprintf("%d.%d.%d", q.Adults, q.Children, q.Infants)}, "sc": {q.Cabin}}
	return "https://www.traveloka.com/" + strings.ToLower(q.Locale) + path + "?" + args.Encode(), nil
}
func HotelURL(q Query) (string, error) {
	if err := ValidateQuery(q); err != nil {
		return "", err
	}
	if q.Kind != "hotel" && q.Kind != "hotels" && q.Kind != "rooms" {
		return "", apiError("INVALID_INPUT", "hotel query required", 0, false)
	}
	name := q.PropertyName
	if name == "" {
		return "", apiError("INVALID_INPUT", "source destination or property name is required for hotel handoff", 0, false)
	}
	in, _ := time.Parse("2006-01-02", q.CheckIn)
	out, _ := time.Parse("2006-01-02", q.CheckOut)
	kind, id, path := "HOTEL_GEO", q.GeoID, "/hotel/search"
	if q.PropertyID != "" {
		kind, id, path = "HOTEL", q.PropertyID, "/hotel/detail"
	}
	spec := fmt.Sprintf("%s.%s.%d.%d.%s.%s.%s.%d", in.Format("02-01-2006"), out.Format("02-01-2006"), int(out.Sub(in).Hours()/24), q.Rooms, kind, id, name, q.Adults)
	args := url.Values{"spec": {spec}}
	if len(q.ChildAges) > 0 {
		ages := make([]string, len(q.ChildAges))
		for i, age := range q.ChildAges {
			ages[i] = strconv.Itoa(age)
		}
		args.Set("childSpec", strings.Join(ages, ","))
	}
	return "https://www.traveloka.com/" + strings.ToLower(q.Locale) + path + "?" + args.Encode(), nil
}
