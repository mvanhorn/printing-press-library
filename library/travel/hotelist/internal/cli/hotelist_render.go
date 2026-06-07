// Hand-authored shared query runner, output rendering, and amenity vocabulary
// for the Hotelist CLI. Not generated.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/hotelist/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/hotelist/internal/store"
)

// amenityAliases maps friendly user input to the exact photo-verified amenity
// labels hotelist.com filters on. Unknown inputs pass through Title-cased.
var amenityAliases = map[string]string{
	"gym":               "Gym",
	"weights":           "Weightlifting gym",
	"weightlifting":     "Weightlifting gym",
	"weightlifting gym": "Weightlifting gym",
	"squat rack":        "Squat rack",
	"squat":             "Squat rack",
	"pool":              "Pool",
	"tennis":            "Tennis court",
	"tennis court":      "Tennis court",
	"sauna":             "Sauna",
	"infrared sauna":    "Infrared sauna",
	"jacuzzi":           "Jacuzzi",
	"coworking":         "Coworking",
	"desk":              "Working desk",
	"working desk":      "Working desk",
	"kitchen":           "Kitchen",
	"bath":              "Bath",
	"bathtub":           "Bath",
	"nespresso":         "Nespresso",
	"coffee":            "Coffee maker",
	"coffee maker":      "Coffee maker",
	"kettle":            "Kettle",
	"pets":              "Pets allowed",
	"pets allowed":      "Pets allowed",
	"ocean view":        "Ocean view",
	"beach view":        "Beach view",
	"mountain view":     "Mountain view",
	"ski":               "Ski hotel",
	"parking":           "Parking",
	"beach":             "Beach",
	"adults only":       "Adults only",
	"massage":           "Massage",
	"business center":   "Business center",
	"iron":              "Iron",
	"restaurant":        "Restaurant",
	"blackout blinds":   "Blackout blinds",
	"modern":            "Modern interior",
}

func amenityLabel(input string) string {
	in := strings.ToLower(strings.TrimSpace(input))
	if in == "" {
		return ""
	}
	if label, ok := amenityAliases[in]; ok {
		return label
	}
	return strings.Title(in)
}

// amenityKeywords maps a normalized amenity term to the case-insensitive
// substrings that signal its presence in a hotel's pros/cons free-text. The
// list endpoint does not return a structured amenity array, and the upstream
// `filters[][target]=amenities` param is silently ignored by the scraped
// backend, so this keyword set is how the CLI enforces --amenities locally.
var amenityKeywords = map[string][]string{
	"gym":           {"gym"},
	"weights":       {"gym", "weight"},
	"weightlifting": {"gym", "weight"},
	"squat":         {"gym", "squat rack", "weight"},
	"pool":          {"pool"},
	"tennis":        {"tennis"},
	"sauna":         {"sauna"},
	"jacuzzi":       {"jacuzzi", "hot tub"},
	"coworking":     {"cowork", "work", "desk", "business"},
	"desk":          {"desk", "work"},
	"kitchen":       {"kitchen", "kitchenette"},
	"bath":          {"bath", "bathtub", "tub"},
	"bathtub":       {"bath", "bathtub", "tub"},
	"coffee":        {"coffee", "nespresso", "espresso"},
	"nespresso":     {"nespresso", "coffee"},
	"kettle":        {"kettle", "tea"},
	"pets":          {"pet", "dog", "animal"},
	"ocean view":    {"ocean", "sea view", "beach"},
	"beach view":    {"beach", "ocean", "sea"},
	"mountain view": {"mountain"},
	"breakfast":     {"breakfast"},
	"parking":       {"parking", "park"},
	"spa":           {"spa"},
	"rooftop":       {"rooftop", "roof top"},
	"balcony":       {"balcony", "terrace"},
}

// amenityMatchTerms returns the lowercase substrings used to detect one
// requested amenity in pros/cons text. Falls back to the bare term when the
// amenity is not in the keyword vocabulary so unknown inputs still narrow
// rather than silently pass everything through.
func amenityMatchTerms(input string) []string {
	in := strings.ToLower(strings.TrimSpace(input))
	if in == "" {
		return nil
	}
	if kws, ok := amenityKeywords[in]; ok {
		return kws
	}
	return []string{in}
}

// matchesAmenities reports whether a hotel's pros/cons free-text mentions every
// requested amenity (AND semantics, matching the upstream filter contract). It
// is a best-effort local predicate because the list payload carries no
// structured amenity field; a hotel whose pros never mention a required amenity
// is excluded. Empty `wanted` keeps every hotel.
func matchesAmenities(h hlHotel, wanted []string) bool {
	if len(wanted) == 0 {
		return true
	}
	hay := strings.ToLower(h.Pros + " " + h.Cons)
	for _, w := range wanted {
		terms := amenityMatchTerms(w)
		if len(terms) == 0 {
			continue
		}
		hit := false
		for _, t := range terms {
			if strings.Contains(hay, t) {
				hit = true
				break
			}
		}
		if !hit {
			return false
		}
	}
	return true
}

// filterByAmenities returns only the hotels whose pros/cons mention every
// requested amenity. `wanted` is the raw user amenity tokens (pre-label).
func filterByAmenities(hotels []hlHotel, wanted []string) []hlHotel {
	if len(wanted) == 0 {
		return hotels
	}
	kept := hotels[:0:0]
	for _, h := range hotels {
		if matchesAmenities(h, wanted) {
			kept = append(kept, h)
		}
	}
	return kept
}

// sortKeyMap maps user --sort values to (api key, order).
var sortKeyMap = map[string][2]string{
	"score":      {"hotellist_rating", "desc"},
	"rating":     {"hotellist_rating", "desc"},
	"score-asc":  {"hotellist_rating", "asc"},
	"price":      {"price", "asc"},
	"price-desc": {"price", "desc"},
	"newest":     {"year_built", "desc"},
	"oldest":     {"year_built", "asc"},
	"value":      {"best-value", "desc"},
	"best-value": {"best-value", "desc"},
}

func resolveSort(input string) (apiFilter, bool) {
	in := strings.ToLower(strings.TrimSpace(input))
	if in == "" {
		return apiFilter{}, false
	}
	if kv, ok := sortKeyMap[in]; ok {
		return filterSort(kv[0], kv[1]), true
	}
	return apiFilter{}, false
}

// hasPriceFilter reports whether any extra filter constrains price, so the
// command can locally drop unknown-price hotels the server's price filter
// would otherwise let through (price 0 / NULL satisfies a "less-than" bound).
func hasPriceFilter(extra []apiFilter) bool {
	for _, f := range extra {
		if f.Target == "price" {
			return true
		}
	}
	return false
}

// storeHotels upserts fetched hotels into the local mirror for offline reuse
// and the watch/diff drift tracker. Best-effort; storage errors are ignored so
// a transient DB issue never fails a live query.
func storeHotels(db *store.Store, hotels []hlHotel) {
	if db == nil {
		return
	}
	for _, h := range hotels {
		if h.HotelID == "" {
			continue
		}
		raw, err := json.Marshal(h)
		if err != nil {
			continue
		}
		_ = db.Upsert("hotel", h.HotelID, raw)
	}
}

// checkinCheckoutNote returns the honest disclaimer when the user passes date
// flags Hotelist cannot honor.
func checkinCheckoutNote(checkin, checkout string) string {
	if strings.TrimSpace(checkin) == "" && strings.TrimSpace(checkout) == "" {
		return ""
	}
	return "Hotelist has no date-based pricing; --checkin/--checkout are recorded as context only and do not change results. Prices are AI-estimated nightly figures."
}

// buildView turns raw hotels into the output view, applying an optional limit.
func buildView(label string, hotels []hlHotel, limit int, note string) hotelListView {
	if limit > 0 && len(hotels) > limit {
		hotels = hotels[:limit]
	}
	outs := make([]hotelOut, 0, len(hotels))
	for _, h := range hotels {
		outs = append(outs, toHotelOut(h))
	}
	return hotelListView{
		Source:     hotelistSource,
		Disclaimer: hotelistDisclaimer,
		Location:   label,
		Count:      len(outs),
		Hotels:     outs,
		Note:       note,
	}
}

// printHotelView prints the view as JSON (machine/piped) or a human table.
func printHotelView(out io.Writer, flags *rootFlags, view hotelListView) error {
	if !wantsHumanTable(out, flags) {
		return printJSONFiltered(out, view, flags)
	}
	if view.Location != "" {
		fmt.Fprintf(out, "%s — %d hotels\n", view.Location, view.Count)
	} else {
		fmt.Fprintf(out, "%d hotels\n", view.Count)
	}
	fmt.Fprintln(out, strings.Repeat("-", 72))
	for i, h := range view.Hotels {
		badge := ""
		if h.Exceptional {
			badge = " ✨exceptional"
		}
		chain := ""
		if h.Chain != "" {
			chain = "  [" + h.Chain + "]"
		}
		val := ""
		if h.ValuePer100USD > 0 {
			val = fmt.Sprintf("  value %.1f/100$", h.ValuePer100USD)
		}
		fmt.Fprintf(out, "%2d. %-40s  ⭐%.1f  $%.0f%s%s%s\n",
			i+1, truncate(h.Name, 40), h.Rating, h.Price, val, chain, badge)
		if len(h.Pros) > 0 {
			fmt.Fprintf(out, "    %s\n", truncate(strings.TrimSpace(h.Pros[0]), 66))
		}
	}
	fmt.Fprintln(out, strings.Repeat("-", 72))
	if view.Note != "" {
		fmt.Fprintf(out, "note: %s\n", view.Note)
	}
	fmt.Fprintf(out, "%s\n", view.Disclaimer)
	return nil
}

// runHotelQuery is the shared path behind search/filter/value: resolve location,
// fetch from /api, store, render. extra holds command-specific filters; sort is
// an optional sort filter; limit caps output (not the upstream fetch).
func runHotelQuery(ctx context.Context, c *client.Client, db *store.Store, flags *rootFlags,
	out io.Writer,
	loc *resolvedLocation, extra []apiFilter, sort *apiFilter, search string, limit int, extraNote string) error {

	filters := append([]apiFilter{}, loc.Filters...)
	filters = append(filters, extra...)
	if sort != nil {
		filters = append(filters, *sort)
	}

	hotels, err := adaptiveFetch(ctx, c, filters, search)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	hotels = dedupeHotels(hotels)
	storeHotels(db, hotels)
	if hasPriceFilter(extra) {
		hotels = dropUnpriced(hotels) // a price bound cannot be met by unknown-price hotels
	}

	note := loc.Note
	if extraNote != "" {
		if note != "" {
			note += " "
		}
		note += extraNote
	}
	view := buildView(loc.Label, hotels, limit, note)
	return printHotelView(out, flags, view)
}

// runValueQuery fetches, then locally ranks by Hotelist rating-per-dollar so
// priced hotels lead and price-unknown hotels sink to the bottom (the upstream
// best-value sort alone can float price-0 hotels to the top).
func runValueQuery(ctx context.Context, c *client.Client, db *store.Store, flags *rootFlags,
	out io.Writer, loc *resolvedLocation, extra []apiFilter, limit int, note string) error {

	filters := append([]apiFilter{}, loc.Filters...)
	filters = append(filters, extra...)
	filters = append(filters, filterSort("best-value", "desc"))

	hotels, err := adaptiveFetch(ctx, c, filters, "")
	if err != nil {
		return classifyAPIError(err, flags)
	}
	hotels = dedupeHotels(hotels)
	storeHotels(db, hotels)
	hotels = dropUnpriced(hotels) // value ranking requires a real price
	sortHotelsByValue(hotels)

	full := note
	if loc.Note != "" {
		full = strings.TrimSpace(loc.Note + " " + note)
	}
	view := buildView(loc.Label, hotels, limit, full)
	return printHotelView(out, flags, view)
}

// runHotelQueryFiltered is runHotelQuery plus a local post-fetch predicate
// (used for filters the upstream /api cannot express, e.g. --exceptional).
func runHotelQueryFiltered(ctx context.Context, c *client.Client, db *store.Store, flags *rootFlags,
	out io.Writer,
	loc *resolvedLocation, extra []apiFilter, sort *apiFilter, search string, limit int, extraNote string,
	keep func(hlHotel) bool) error {

	filters := append([]apiFilter{}, loc.Filters...)
	filters = append(filters, extra...)
	if sort != nil {
		filters = append(filters, *sort)
	}
	hotels, err := adaptiveFetch(ctx, c, filters, search)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	hotels = dedupeHotels(hotels)
	storeHotels(db, hotels)
	if hasPriceFilter(extra) {
		hotels = dropUnpriced(hotels) // a price bound cannot be met by unknown-price hotels
	}

	if keep != nil {
		kept := hotels[:0:0]
		for _, h := range hotels {
			if keep(h) {
				kept = append(kept, h)
			}
		}
		hotels = kept
	}

	note := loc.Note
	if extraNote != "" {
		if note != "" {
			note += " "
		}
		note += extraNote
	}
	view := buildView(loc.Label, hotels, limit, note)
	return printHotelView(out, flags, view)
}

// openHotelStore opens the local SQLite mirror at the default path.
func openHotelStore(ctx context.Context, flags *rootFlags) (*store.Store, error) {
	return store.OpenWithContext(ctx, defaultDBPath("hotelist-pp-cli"))
}
