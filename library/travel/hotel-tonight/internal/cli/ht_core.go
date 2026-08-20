// HotelTonight novel-feature core: shared types, the /v6/inventory fetch path,
// and the date / category / discount helpers the watch, history, verdict,
// compare-neighborhoods, datescan, and daily-drop commands build on. Hand
// authored (survives `generate --force`); not a generator-emitted file.
package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hotel-tonight/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/hotel-tonight/internal/cliutil"
)

// htCategories is HotelTonight's hotel quality tier set, as observed on the
// hotel.category field of the live /v6/inventory feed.
var htCategories = []string{"basic", "solid", "hip", "luxe", "charming", "crashpad"}

// timeNow is the clock used for date resolution; a package var so tests can
// pin "now" without touching the wall clock.
var timeNow = time.Now

// categoryFlagHelp is the --category flag description, built from htCategories
// so the four commands that expose the filter can't list a stale tier set.
var categoryFlagHelp = "Filter by hotel tier(s), comma-separated: " + strings.Join(htCategories, ", ")

// dailyDropDealType is the deal_type HotelTonight assigns to its once-a-day
// flash deal.
const dailyDropDealType = "daily_drop"

// htHotel maps the hotel fields the novel commands consume. Numeric fields use
// json.Number so a market that quotes them as strings (HotelTonight is
// inconsistent across markets) does not break unmarshaling.
type htHotel struct {
	ID           json.Number   `json:"id"`
	Name         string        `json:"name"`
	Neighborhood string        `json:"neighborhood"`
	Address      string        `json:"address"`
	City         string        `json:"city"`
	State        string        `json:"state"`
	Latitude     json.Number   `json:"latitude"`
	Longitude    json.Number   `json:"longitude"`
	Category     string        `json:"category"`
	WhyWeLikeIt  htWhyWeLikeIt `json:"why_we_like_it"`
}

// htWhyWeLikeIt is HotelTonight's editorial blurb for a hotel: a titled set of
// line items. The /v6/inventory feed returns this as an object, not a string.
type htWhyWeLikeIt struct {
	Title     string `json:"title"`
	LineItems []struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	} `json:"line_items"`
}

// text flattens the editorial line items into one space-joined sentence.
func (w htWhyWeLikeIt) text() string {
	var parts []string
	for _, li := range w.LineItems {
		if d := strings.TrimSpace(li.Description); d != "" {
			parts = append(parts, d)
		}
	}
	return strings.Join(parts, " ")
}

// htRoom maps only the room/deal fields the novel commands consume. Fields
// HotelTonight types inconsistently across markets (e.g. id/deal_id appear as
// both string and number) are intentionally omitted to keep unmarshaling
// robust; numeric prices use json.Number so a market that quotes them as
// strings does not break parsing.
type htRoom struct {
	DealType              string      `json:"deal_type"`
	CustomerPricePerNight json.Number `json:"customer_price_per_night"`
	StrikethroughPrice    json.Number `json:"strikethrough_price"`
	TotalCustomerPrice    json.Number `json:"total_customer_price"`
	NumRemaining          json.Number `json:"num_remaining"`
	Available             bool        `json:"available"`
	SoldOut               bool        `json:"sold_out"`
	Hotel                 htHotel     `json:"hotel"`
}

type htMarket struct {
	ID          json.Number `json:"id"`
	CityName    string      `json:"city_name"`
	DisplayName string      `json:"display_name"`
	Name        string      `json:"name"`
	Latitude    json.Number `json:"latitude"`
	Longitude   json.Number `json:"longitude"`
	SeoSlug     string      `json:"seo_slug"`
	State       string      `json:"state"`
	CountryCode string      `json:"country_code"`
}

type htInventory struct {
	PrimaryMarket htMarket    `json:"primary_market"`
	CurrentDay    string      `json:"current_day"`
	NumNights     json.Number `json:"num_nights"`
	Rooms         []htRoom    `json:"rooms"`
}

// htDeal is the flattened, agent-friendly view of one room/deal that the novel
// commands emit and persist. The verbose nested room/hotel payload is
// collapsed to the high-gravity fields with a derived percent-off.
type htDeal struct {
	HotelID      int64   `json:"hotel_id"`
	HotelName    string  `json:"hotel_name"`
	Neighborhood string  `json:"neighborhood"`
	City         string  `json:"city"`
	State        string  `json:"state"`
	Category     string  `json:"category"`
	DealType     string  `json:"deal_type"`
	Price        float64 `json:"price"`
	Was          float64 `json:"was,omitempty"`
	PctOff       int     `json:"pct_off"`
	NumRemaining int64   `json:"num_remaining"`
	SoldOut      bool    `json:"sold_out"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	WhyWeLikeIt  string  `json:"why_we_like_it,omitempty"`
}

// numF converts a json.Number to float64, returning 0 for empty/unparseable
// values so a missing field reads as "no data" rather than crashing.
func numF(n json.Number) float64 {
	f, err := n.Float64()
	if err != nil {
		return 0
	}
	return f
}

// numI converts a json.Number to int64, returning 0 for empty/unparseable
// values.
func numI(n json.Number) int64 {
	i, err := n.Int64()
	if err != nil {
		// Fall back through float (e.g. "2.0") then truncate.
		return int64(numF(n))
	}
	return i
}

// toDeal flattens a room into the agent-friendly deal view, deriving percent
// off from the strikethrough (highest rate in the last 30 days) and cleaning
// any HTML entities out of the editorial "why we like it" blurb.
func (r htRoom) toDeal() htDeal {
	price := numF(r.CustomerPricePerNight)
	strike := numF(r.StrikethroughPrice)
	return htDeal{
		HotelID:      numI(r.Hotel.ID),
		HotelName:    cliutil.CleanText(r.Hotel.Name),
		Neighborhood: cliutil.CleanText(r.Hotel.Neighborhood),
		City:         r.Hotel.City,
		State:        r.Hotel.State,
		Category:     r.Hotel.Category,
		DealType:     r.DealType,
		Price:        price,
		Was:          strike,
		PctOff:       pctOff(strike, price),
		NumRemaining: numI(r.NumRemaining),
		SoldOut:      r.SoldOut,
		Latitude:     numF(r.Hotel.Latitude),
		Longitude:    numF(r.Hotel.Longitude),
		WhyWeLikeIt:  cliutil.CleanText(r.Hotel.WhyWeLikeIt.text()),
	}
}

// pctOff returns the integer percent saved versus the strikethrough price.
// HotelTonight defines strikethrough_price as the highest rate seen for the
// same hotel and dates in the last 30 days, so this is a real discount, not a
// marketing figure. Returns 0 when inputs are missing or non-discounting.
func pctOff(strike, price float64) int {
	if strike <= 0 || price <= 0 || price >= strike {
		return 0
	}
	return int(math.Round((strike - price) / strike * 100))
}

// normalizeCategory lowercases and strips a category label so "CrashPad",
// "crash pad", and "crashpad" all compare equal.
func normalizeCategory(s string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), " ", "")
}

// parseCategories splits a comma-separated --category value into normalized
// tiers, returning an error naming the offending value if any is unknown.
func parseCategories(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		n := normalizeCategory(part)
		if n == "" {
			continue
		}
		valid := false
		for _, c := range htCategories {
			if c == n {
				valid = true
				break
			}
		}
		if !valid {
			return nil, fmt.Errorf("unknown --category %q: valid tiers are %s", strings.TrimSpace(part), strings.Join(htCategories, ", "))
		}
		out = append(out, n)
	}
	return out, nil
}

// matchesCategories reports whether a hotel's category is in the requested
// set. An empty want list matches everything.
func matchesCategories(hotelCategory string, want []string) bool {
	if len(want) == 0 {
		return true
	}
	hc := normalizeCategory(hotelCategory)
	for _, w := range want {
		if w == hc {
			return true
		}
	}
	return false
}

// computeDates resolves a "when" keyword (tonight, tomorrow, weekend) into a
// check-in/check-out date pair relative to now. An empty or unrecognized
// "when" returns empty strings, letting the server apply its tonight default.
// Pure (takes now) so it is unit-testable.
func computeDates(when string, now time.Time) (checkIn, checkOut string) {
	const layout = "2006-01-02"
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	switch strings.ToLower(strings.TrimSpace(when)) {
	case "", "tonight":
		return "", ""
	case "tomorrow":
		ci := day.AddDate(0, 0, 1)
		return ci.Format(layout), ci.AddDate(0, 0, 1).Format(layout)
	case "weekend":
		// Upcoming Friday -> Sunday (2 nights). If today is already Fri/Sat,
		// use this weekend; otherwise the next one.
		offset := (int(time.Friday) - int(day.Weekday()) + 7) % 7
		fri := day.AddDate(0, 0, offset)
		return fri.Format(layout), fri.AddDate(0, 0, 2).Format(layout)
	default:
		return "", ""
	}
}

// fetchInventory pulls the live /v6/inventory deal feed for a coordinate and
// optional date range. It always bypasses the response cache: deal prices are
// time-sensitive and a cached snapshot would silently record stale rates.
func fetchInventory(c *client.Client, lat, lng, checkIn, checkOut string, rooms int) (*htInventory, error) {
	params := map[string]string{"latitude": lat, "longitude": lng}
	if checkIn != "" {
		params["check_in"] = checkIn
	}
	if checkOut != "" {
		params["check_out"] = checkOut
	}
	if rooms > 0 {
		params["rooms"] = strconv.Itoa(rooms)
	}
	raw, err := c.GetNoCache("/v6/inventory", params)
	if err != nil {
		return nil, err
	}
	var inv htInventory
	if err := json.Unmarshal(raw, &inv); err != nil {
		return nil, fmt.Errorf("parsing inventory response: %w", err)
	}
	return &inv, nil
}

// validateCoord checks that a coordinate string parses as a number in the
// given inclusive range, returning an actionable error otherwise. Catches the
// common "passed a placeholder / city name / one coord missing" mistakes
// before the API rejects them with an opaque 400.
func validateCoord(name, v string, min, max float64) error {
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil {
		return fmt.Errorf("--%s must be a number (got %q); e.g. --lat 37.7749 --lng -122.4194, or use --metro <id> (see 'markets list')", name, v)
	}
	if f < min || f > max {
		return fmt.Errorf("--%s %s is out of range [%g, %g]", name, v, min, max)
	}
	return nil
}

// resolveGeo turns either an explicit lat/lng or a numeric market id into a
// coordinate pair. When a market id is given it fetches /v2/market_cities/{id}
// and returns the market's center plus its display name. Explicit coordinates
// are validated as numbers in range before any API call.
func resolveGeo(c *client.Client, metroID int, lat, lng string) (string, string, *htMarket, error) {
	// Exactly-one-coord is a common mistake; treat it as a usage error rather
	// than silently falling through to the --metro path.
	if (lat != "") != (lng != "") {
		return "", "", nil, fmt.Errorf("--lat and --lng must be given together; e.g. --lat 37.7749 --lng -122.4194")
	}
	if lat != "" && lng != "" {
		if err := validateCoord("lat", lat, -90, 90); err != nil {
			return "", "", nil, err
		}
		if err := validateCoord("lng", lng, -180, 180); err != nil {
			return "", "", nil, err
		}
		return lat, lng, nil, nil
	}
	if metroID <= 0 {
		return "", "", nil, fmt.Errorf("provide --lat and --lng, or --metro <market-id> (see 'markets list')")
	}
	raw, err := c.Get(fmt.Sprintf("/v2/market_cities/%d", metroID), nil)
	if err != nil {
		return "", "", nil, fmt.Errorf("resolving market %d: %w", metroID, err)
	}
	var m htMarket
	if err := json.Unmarshal(raw, &m); err != nil {
		return "", "", nil, fmt.Errorf("parsing market %d: %w", metroID, err)
	}
	mlat, mlng := numF(m.Latitude), numF(m.Longitude)
	if mlat == 0 && mlng == 0 {
		return "", "", nil, fmt.Errorf("market %d has no coordinates", metroID)
	}
	return strconv.FormatFloat(mlat, 'f', -1, 64),
		strconv.FormatFloat(mlng, 'f', -1, 64), &m, nil
}

// dealsFrom flattens an inventory's rooms into deals, applying the category
// filter and an optional minimum percent-off, sorted by the requested key.
func dealsFrom(inv *htInventory, cats []string, minPctOff int, sortKey string) []htDeal {
	var out []htDeal
	for _, r := range inv.Rooms {
		if !matchesCategories(r.Hotel.Category, cats) {
			continue
		}
		d := r.toDeal()
		if d.PctOff < minPctOff {
			continue
		}
		out = append(out, d)
	}
	sortDeals(out, sortKey)
	return out
}

// isDiscountSort reports whether a --sort value asks for biggest-discount-first
// ordering. Owns the accepted synonym set so the deal and neighborhood sorts
// can't drift apart.
func isDiscountSort(sortKey string) bool {
	switch strings.ToLower(strings.TrimSpace(sortKey)) {
	case "discount", "pct-off", "pctoff":
		return true
	default:
		return false
	}
}

// sortDeals orders deals in place: discount by percent off descending, else
// price ascending.
func sortDeals(deals []htDeal, sortKey string) {
	if isDiscountSort(sortKey) {
		sort.SliceStable(deals, func(i, j int) bool { return deals[i].PctOff > deals[j].PctOff })
		return
	}
	sort.SliceStable(deals, func(i, j int) bool { return deals[i].Price < deals[j].Price })
}
