package planner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type translation struct {
	Locale      string `json:"locale"`
	Translation string `json:"translation"`
}
type venueWire struct {
	ID            string        `json:"_id"`
	Slug          string        `json:"slug"`
	Names         []translation `json:"name_translations"`
	Currency      *string       `json:"currency"`
	TimeZone      *string       `json:"time_zone"`
	Country       *string       `json:"country"`
	Cuisines      []string      `json:"cuisines"`
	DinnerAverage *string       `json:"budget_dinner_avg"`
	BookingMode   *string       `json:"booking_page_mode"`
	Distance      *float64      `json:"distance"`
	Geocode       *struct {
		Lat *float64 `json:"lat"`
		Lon *float64 `json:"lon"`
	} `json:"geocode"`
	raw map[string]json.RawMessage
}

func (v *venueWire) UnmarshalJSON(data []byte) error {
	type alias venueWire
	var a alias
	if e := json.Unmarshal(data, &a); e != nil {
		return e
	}
	*v = venueWire(a)
	return json.Unmarshal(data, &v.raw)
}

type searchMeta struct {
	RecordCount *int    `json:"record_count"`
	Cursor      *string `json:"search_after"`
	LastPage    *bool   `json:"last_page"`
}
type searchWire struct {
	Shops []venueWire `json:"shops"`
	Meta  *searchMeta `json:"meta"`
}
type cuisinesWire struct {
	Cuisines []struct {
		Field string        `json:"field"`
		Texts []translation `json:"text_translations"`
	} `json:"cuisines"`
}
type menuWire struct {
	ID             string        `json:"id"`
	Names          []translation `json:"name_translations"`
	Price          *string       `json:"price"`
	PreviousPrice  *string       `json:"prev_price"`
	PriceMode      *string       `json:"price_mode"`
	TaxType        *string       `json:"tax_type"`
	ServiceFeeType *string       `json:"service_fee_type"`
	Status         *string       `json:"availability_status"`
	raw            map[string]json.RawMessage
}

func (m *menuWire) UnmarshalJSON(data []byte) error {
	type alias menuWire
	var a alias
	if e := json.Unmarshal(data, &a); e != nil {
		return e
	}
	*m = menuWire(a)
	return json.Unmarshal(data, &m.raw)
}

type menusWire struct {
	Items []menuWire `json:"menu_items"`
}
type slotWire struct {
	Available *bool `json:"is_available"`
	raw       map[string]json.RawMessage
}

func (s *slotWire) UnmarshalJSON(data []byte) error {
	type alias slotWire
	var a alias
	if e := json.Unmarshal(data, &a); e != nil {
		return e
	}
	*s = slotWire(a)
	return json.Unmarshal(data, &s.raw)
}

type calendarWire struct {
	Calendar *struct {
		Type     string                         `json:"type"`
		TimeZone string                         `json:"time_zone"`
		Closed   []string                       `json:"closed_dates"`
		Data     map[string]map[string]slotWire `json:"data"`
	} `json:"availability_calendar"`
}

func requireArray(raw []byte, key string, target any) error {
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(raw, &fields); e != nil {
		return e
	}
	v, ok := fields[key]
	if !ok || len(v) == 0 || v[0] != '[' {
		return fmt.Errorf("missing or non-array %s", key)
	}
	return json.Unmarshal(raw, target)
}
func validateSearch(raw []byte) error {
	var w searchWire
	if e := requireArray(raw, "shops", &w); e != nil {
		return e
	}
	if w.Meta == nil {
		return fmt.Errorf("missing search meta")
	}
	for _, v := range w.Shops {
		if v.ID == "" || v.Slug == "" {
			return fmt.Errorf("search shop missing stable ID or slug")
		}
	}
	return nil
}
func validateVenue(raw []byte) error {
	var w searchWire
	if e := requireArray(raw, "shops", &w); e != nil {
		return e
	}
	for _, v := range w.Shops {
		if v.ID == "" || v.Slug == "" {
			return fmt.Errorf("venue missing stable ID or slug")
		}
	}
	return nil
}
func validateCuisines(raw []byte) error { var w cuisinesWire; return requireArray(raw, "cuisines", &w) }
func validateMenus(raw []byte) error {
	var w menusWire
	if e := requireArray(raw, "menu_items", &w); e != nil {
		return e
	}
	for _, m := range w.Items {
		if m.ID == "" {
			return fmt.Errorf("menu item missing stable ID")
		}
		for _, p := range []*string{m.Price, m.PreviousPrice} {
			if p != nil && !decimalPattern.MatchString(*p) {
				return fmt.Errorf("menu price must be decimal text")
			}
		}
	}
	return nil
}
func text(t []translation, locale string) any {
	for _, v := range t {
		if v.Locale == locale && strings.TrimSpace(v.Translation) != "" {
			return v.Translation
		}
	}
	return nil
}
func displayName(t []translation) any {
	if v := text(t, "en"); v != nil {
		return v
	}
	if v := text(t, "ja"); v != nil {
		return v
	}
	for _, v := range t {
		if v.Translation != "" {
			return v.Translation
		}
	}
	return nil
}
func decodeAny(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var v any
	if d.Decode(&v) != nil {
		return nil
	}
	return v
}
func rawField(m map[string]json.RawMessage, key string) any { return decodeAny(m[key]) }
func optionalString(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}
func canonical(slug, mode string) (string, string) {
	venueURL := "https://www.tablecheck.com/en/" + url.PathEscape(slug)
	switch mode {
	case "v2":
		return venueURL, "https://www.tablecheck.com/en/" + url.PathEscape(slug) + "/reserve/landing"
	case "v1":
		return venueURL, "https://www.tablecheck.com/en/shops/" + url.PathEscape(slug) + "/reserve"
	default:
		return venueURL, ""
	}
}
func venueSummary(v venueWire) map[string]any {
	mode := ""
	if v.BookingMode != nil {
		mode = *v.BookingMode
	}
	venueURL, bookingURL := canonical(v.Slug, mode)
	var booking any
	if bookingURL != "" {
		booking = bookingURL
	}
	var lat, lon any
	if v.Geocode != nil {
		if v.Geocode.Lat != nil {
			lat = *v.Geocode.Lat
		}
		if v.Geocode.Lon != nil {
			lon = *v.Geocode.Lon
		}
	}
	var dist any
	if v.Distance != nil {
		dist = *v.Distance
	}
	return map[string]any{
		"id": v.ID, "slug": v.Slug, "name": displayName(v.Names), "name_ja": text(v.Names, "ja"), "name_en": text(v.Names, "en"), "currency": optionalString(v.Currency), "time_zone": optionalString(v.TimeZone), "country": optionalString(v.Country), "cuisines": v.Cuisines, "latitude": lat, "longitude": lon, "distance_m": dist, "dinner_average_budget": optionalString(v.DinnerAverage), "budget_basis": "venue_dinner_average", "venue_url": venueURL, "booking_url": booking, "booking_page_mode": optionalString(v.BookingMode),
	}
}
func courseSummary(m menuWire, v venueWire) map[string]any {
	return map[string]any{"id": m.ID, "name": displayName(m.Names), "name_ja": text(m.Names, "ja"), "name_en": text(m.Names, "en"), "price": optionalString(m.Price), "previous_price": optionalString(m.PreviousPrice), "currency": optionalString(v.Currency), "price_mode": optionalString(m.PriceMode), "price_basis": rawField(m.raw, "price_basis"), "tax_type": optionalString(m.TaxType), "service_fee_type": optionalString(m.ServiceFeeType), "service_fee_rate": rawField(m.raw, "service_fee_rate"), "availability_status": optionalString(m.Status), "availability_scope": "course_listing_eligibility", "qty_remaining": rawField(m.raw, "qty_remaining"), "is_active": rawField(m.raw, "is_active"), "is_hidden": rawField(m.raw, "is_hidden")}
}

// An unresolved lookup still preserves the requested slug without invented identity.
func unknownVenue(slug string) map[string]any {
	summary := venueSummary(venueWire{Slug: slug})
	summary["id"] = nil
	summary["venue_url"] = nil
	summary["booking_url"] = nil
	return summary
}
