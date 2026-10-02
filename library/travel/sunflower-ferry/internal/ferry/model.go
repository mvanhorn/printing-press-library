// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package ferry

import (
	"fmt"
	"strings"
	"time"
)

const PublicBase = "https://www.ferry-sunflower.co.jp"
const BookingBase = "https://booking.ferry-sunflower.co.jp"
const BookingURL = BookingBase + "/web/yoyaku/Reserve0000/IndexEnglish"
const ConditionsURL = "https://www.sunflower.co.jp/stipulate/passenger/"

var JST = time.FixedZone("JST", 9*60*60)

type Port struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	NameJA string `json:"name_ja"`
}
type Route struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	NameJA       string `json:"name_ja"`
	OutboundLine string `json:"outbound_line"`
	InboundLine  string `json:"inbound_line"`
	Origin       Port   `json:"origin"`
	Destination  Port   `json:"destination"`
	SourceURL    string `json:"source_url"`
	BookingURL   string `json:"booking_url"`
}

var Registry = []Route{
	{"osaka-beppu", "Osaka–Beppu", "大阪～別府", "21", "22", Port{"osaka-terminal1", "Osaka Sunflower Terminal 1", "さんふらわあターミナル（大阪）第1ターミナル"}, Port{"beppu", "Beppu Kanko-ko", "別府観光港"}, PublicBase + "/en/route/osaka-beppu/time/", BookingURL},
	{"kobe-oita", "Kobe–Oita", "神戸～大分", "11", "12", Port{"kobe", "Kobe Rokko Island", "神戸六甲アイランド港"}, Port{"oita", "Oita", "大分港"}, PublicBase + "/en/route/kobe-oita/time/", BookingURL},
	{"osaka-shibushi", "Osaka–Shibushi (Kagoshima)", "大阪～志布志（鹿児島）", "31", "32", Port{"osaka-terminal2", "Osaka Sunflower Terminal 2", "さんふらわあターミナル（大阪）第2ターミナル"}, Port{"shibushi", "Shibushi", "志布志港"}, PublicBase + "/en/route/osaka-shibushi/time/", BookingURL},
}

func ResolveRoute(id, direction string) (Route, string, error) {
	if direction != "outbound" && direction != "inbound" {
		return Route{}, "", fmt.Errorf("--direction must be outbound or inbound")
	}
	for _, r := range Registry {
		if id == r.ID || id == r.OutboundLine || id == r.InboundLine {
			line := r.OutboundLine
			if direction == "inbound" {
				line = r.InboundLine
			}
			if id == r.OutboundLine || id == r.InboundLine {
				line = id
				if direction == "inbound" && id == r.OutboundLine {
					return Route{}, "", fmt.Errorf("line %s is outbound; use its route slug with --direction inbound", id)
				}
			}
			return r, line, nil
		}
	}
	return Route{}, "", fmt.Errorf("unknown route %q; use routes list", id)
}
func ParseDate(s string) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02", s, JST)
	if err != nil || t.Format("2006-01-02") != s {
		return time.Time{}, fmt.Errorf("date must be an exact YYYY-MM-DD Japan service date")
	}
	return t, nil
}
func SearchRoutes(query string) []Route {
	var out = make([]Route, 0)
	q := strings.ToLower(strings.TrimSpace(query))
	for _, r := range Registry {
		if strings.Contains(strings.ToLower(r.ID+" "+r.Name+" "+r.NameJA+" "+r.OutboundLine+" "+r.InboundLine+" "+r.Origin.ID+" "+r.Origin.Name+" "+r.Origin.NameJA+" "+r.Destination.ID+" "+r.Destination.Name+" "+r.Destination.NameJA), q) {
			out = append(out, r)
		}
	}
	return out
}

type Party struct {
	Adults       int    `json:"adults"`
	Children     int    `json:"children"`
	Toddlers     int    `json:"toddlers"`
	Infants      int    `json:"infants"`
	Mode         string `json:"mode"`
	CarCategory  string `json:"car_category,omitempty"`
	BikeCategory string `json:"bike_category,omitempty"`
	Bikes        int    `json:"bikes,omitempty"`
	PetCages     int    `json:"pet_cages,omitempty"`
}

func (p Party) Validate() error {
	for _, n := range []int{p.Adults, p.Children, p.Toddlers, p.Infants, p.Bikes, p.PetCages} {
		if n < 0 || n > 14 {
			return fmt.Errorf("party/category counts must be between 0 and 14")
		}
	}
	if p.Adults < 1 || p.Adults+p.Children > 14 {
		return fmt.Errorf("online lookup supports at least one adult and at most 14 adults plus children; other parties require the reservation center")
	}
	if p.PetCages > 2 {
		return fmt.Errorf("online lookup supports at most two medium pet cages; contact the reservation center")
	}
	switch p.Mode {
	case "foot":
		if p.CarCategory != "" || p.BikeCategory != "" || p.Bikes != 0 {
			return fmt.Errorf("foot mode cannot include car or bike categories")
		}
	case "car":
		if _, ok := carCodes[p.CarCategory]; !ok {
			return fmt.Errorf("--car-category must be lt3m, lt4m, lt5m or lt6m (strictly less than that length)")
		}
		if p.BikeCategory != "" || p.Bikes != 0 {
			return fmt.Errorf("one lookup cannot combine car and two-wheeled vehicle modes")
		}
	case "bike":
		if _, ok := bikeFields[p.BikeCategory]; !ok {
			return fmt.Errorf("--bike-category must be over750cc, le750cc, le125cc or bicycle")
		}
		if p.Bikes < 1 || p.Bikes > p.Adults {
			return fmt.Errorf("bike mode requires 1..adults two-wheeled vehicles")
		}
		if p.CarCategory != "" {
			return fmt.Errorf("bike mode cannot include a car category")
		}
	default:
		return fmt.Errorf("--mode must be foot, car or bike")
	}
	return nil
}

var carCodes = map[string]string{"lt3m": "201", "lt4m": "202", "lt5m": "203", "lt6m": "204"}
var bikeFields = map[string]string{"over750cc": "RiyoNaiyo.Number_Of_Bike_Over750cc", "le750cc": "RiyoNaiyo.Number_Of_Bike_Less750cc", "le125cc": "RiyoNaiyo.Number_Of_Scooter", "bicycle": "RiyoNaiyo.Number_Of_cycle"}

type Metadata struct {
	ObservedAt     string   `json:"observed_at"`
	SourceURLs     []string `json:"source_urls"`
	SourceLanguage string   `json:"source_language"`
	RequestCount   int      `json:"request_count"`
	ResponseBytes  int      `json:"response_bytes"`
	ElapsedMS      int64    `json:"elapsed_ms"`
}
type Envelope struct {
	Data     any      `json:"data"`
	Meta     Metadata `json:"meta"`
	Warnings []string `json:"warnings,omitempty"`
}
type Rule struct {
	Weekdays         []string `json:"weekdays"`
	WeekdayNumbers   []int    `json:"-"`
	SourceDayLabel   string   `json:"source_day_label"`
	Departure        string   `json:"departure_time"`
	Arrival          string   `json:"arrival_time"`
	ArrivalDayOffset int      `json:"arrival_day_offset"`
}
type Timetable struct {
	Route                  Route  `json:"route"`
	Line                   string `json:"line"`
	Direction              string `json:"direction"`
	SourceEffectiveCaption string `json:"source_effective_caption"`
	Rules                  []Rule `json:"weekday_rules"`
	DateValidity           string `json:"date_validity"`
}
type SeasonDate struct {
	Date          string `json:"date"`
	Weekday       string `json:"weekday"`
	Band          string `json:"band"`
	DaytimeCruise bool   `json:"daytime_cruise"`
}
type Calendar struct {
	RouteID            string       `json:"route_id"`
	Line               string       `json:"line"`
	Dates              []SeasonDate `json:"dates"`
	PublishedFrom      string       `json:"published_from"`
	PublishedTo        string       `json:"published_to"`
	RequestedFrom      string       `json:"requested_from"`
	RequestedTo        string       `json:"requested_to"`
	MissingDates       []string     `json:"missing_dates"`
	Complete           bool         `json:"complete"`
	DiscountConditions []string     `json:"discount_conditions"`
}
type CabinFare struct {
	ID                 string `json:"id"`
	SourceClassCode    string `json:"source_class_code,omitempty"`
	Name               string `json:"name"`
	RoomType           string `json:"room_type"`
	DisplayedPriceJPY  int    `json:"displayed_price_jpy"`
	AvailabilitySymbol string `json:"availability_symbol"`
	Availability       string `json:"availability"`
	Info               string `json:"info,omitempty"`
}
type Sailing struct {
	ID                         string                `json:"id"`
	Ship                       string                `json:"ship"`
	From                       string                `json:"from"`
	To                         string                `json:"to"`
	Departure                  string                `json:"departure"`
	Arrival                    string                `json:"arrival"`
	ArrivalDayOffset           int                   `json:"arrival_day_offset"`
	RecommendedTerminalArrival string                `json:"recommended_terminal_arrival"`
	CabinFares                 []CabinFare           `json:"cabin_fares,omitempty"`
	VehicleAvailability        *AvailabilitySnapshot `json:"vehicle_availability,omitempty"`
}

type AvailabilitySnapshot struct {
	SourceLabel string `json:"source_label"`
	Symbol      string `json:"symbol"`
	Status      string `json:"status"`
	Guaranteed  bool   `json:"guaranteed"`
}
type Quote struct {
	RouteID             string    `json:"route_id"`
	Line                string    `json:"line"`
	BoardingDate        string    `json:"boarding_date"`
	Party               Party     `json:"party"`
	Season              string    `json:"season"`
	DiscountLabel       string    `json:"discount_label"`
	Currency            string    `json:"currency"`
	Sailings            []Sailing `json:"sailings"`
	SourceNotes         []string  `json:"source_notes"`
	PriceBasis          string    `json:"price_basis"`
	Unknowns            []string  `json:"unknowns"`
	InventoryGuaranteed bool      `json:"inventory_guaranteed"`
}
