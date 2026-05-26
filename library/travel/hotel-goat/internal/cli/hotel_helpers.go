// Copyright 2026 kothari-nikunj. Licensed under Apache-2.0. See LICENSE.

// Hand-written helpers shared by all hotel-goat commands. NEW FILE — does
// not exist in the generator's emit set, so subsequent regenerations
// preserve these utilities verbatim.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/hotel-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/hotel-goat/internal/parser"
	"github.com/mvanhorn/printing-press-library/library/travel/hotel-goat/internal/store"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

// hotelSearchOpts is the shared filter set used by `hotels`, `near`,
// `family`, `brand-loyal`, `compare`, `compare-cities`, `cheapest-window`,
// `bundle`. Most fields map 1:1 to Google's Hotels URL params; brand and
// amenities are applied client-side after the parse because Google's URL
// surface doesn't expose every filter the SerpAPI param set has.
type hotelSearchOpts struct {
	Adults           int
	Children         int
	ChildAges        []int
	Rooms            int
	Currency         string
	Sort             string // cheapest/best/rating/reviews
	MinPrice         float64
	MaxPrice         float64
	MinRating        float64
	HotelClass       []int
	Brand            []string
	Amenities        []string
	FreeCancellation bool
	SpecialOffers    bool
	EcoCertified     bool
	Type             string // hotel | rental
	MinBedrooms      int
	MinBathrooms     int
	Limit            int
	Page             int
	Locale           string
}

// buildHotelsURLParams converts the typed opts to the query-string
// map Google Hotels accepts. Only fields the SSR honours are encoded;
// brand and amenities are filtered client-side in filterHotels.
func buildHotelsURLParams(opts hotelSearchOpts) map[string]string {
	p := map[string]string{}
	if opts.Currency != "" {
		p["currency"] = opts.Currency
	}
	if opts.Locale != "" {
		p["hl"] = opts.Locale
	}
	// Google encodes adults/children inside the q param via a structured
	// hint string. For v1, surfacing the count in q (e.g. "for 2 adults")
	// is sufficient. The narrative agent rarely needs sub-adult precision
	// at the SSR level.
	return p
}

// filterHotels applies client-side filters that Google's SSR doesn't
// directly expose: brand match, amenity intersection, hotel-class set,
// min-rating, price band, type (hotel vs rental — vacation-rental
// records have no hotel_class), limit/page slicing.
func filterHotels(hotels []parser.Hotel, opts hotelSearchOpts) []parser.Hotel {
	// Type filter
	if strings.EqualFold(opts.Type, "rental") {
		out := hotels[:0]
		for _, h := range hotels {
			if h.HotelClass == 0 {
				out = append(out, h)
			}
		}
		hotels = out
	} else if strings.EqualFold(opts.Type, "hotel") {
		out := hotels[:0]
		for _, h := range hotels {
			if h.HotelClass > 0 {
				out = append(out, h)
			}
		}
		hotels = out
	}
	// hotel-class set
	if len(opts.HotelClass) > 0 {
		want := map[int]bool{}
		for _, c := range opts.HotelClass {
			want[c] = true
		}
		out := hotels[:0]
		for _, h := range hotels {
			if want[h.HotelClass] {
				out = append(out, h)
			}
		}
		hotels = out
	}
	// min-rating
	if opts.MinRating > 0 {
		out := hotels[:0]
		for _, h := range hotels {
			if h.Rating >= opts.MinRating {
				out = append(out, h)
			}
		}
		hotels = out
	}
	// price band
	if opts.MinPrice > 0 || opts.MaxPrice > 0 {
		out := hotels[:0]
		for _, h := range hotels {
			if opts.MinPrice > 0 && h.PricePerNight < opts.MinPrice {
				continue
			}
			if opts.MaxPrice > 0 && h.PricePerNight > opts.MaxPrice {
				continue
			}
			out = append(out, h)
		}
		hotels = out
	}
	// brand
	if len(opts.Brand) > 0 {
		brandSet := map[string]bool{}
		for _, b := range opts.Brand {
			brandSet[strings.ToLower(strings.TrimSpace(b))] = true
		}
		out := hotels[:0]
		for _, h := range hotels {
			name := strings.ToLower(h.Name)
			brand := strings.ToLower(h.Brand)
			match := false
			for b := range brandSet {
				if b == "" {
					continue
				}
				if strings.Contains(brand, b) || strings.Contains(name, b) {
					match = true
					break
				}
			}
			if match {
				out = append(out, h)
			}
		}
		hotels = out
	}
	// amenity intersection (best-effort; Google's amenity codes aren't
	// always surfaced on the list page, so this is a name/description
	// substring check.)
	if len(opts.Amenities) > 0 {
		wantedAny := false
		for _, a := range opts.Amenities {
			if strings.TrimSpace(a) != "" {
				wantedAny = true
				break
			}
		}
		if wantedAny {
			out := hotels[:0]
			for _, h := range hotels {
				hay := strings.ToLower(h.Name + " " + h.Description + " " + strings.Join(h.Amenities, " "))
				ok := true
				for _, a := range opts.Amenities {
					a = strings.ToLower(strings.TrimSpace(a))
					if a == "" {
						continue
					}
					if !strings.Contains(hay, a) {
						ok = false
						break
					}
				}
				if ok {
					out = append(out, h)
				}
			}
			hotels = out
		}
	}
	// sort
	switch strings.ToLower(opts.Sort) {
	case "cheapest", "price":
		sort.SliceStable(hotels, func(i, j int) bool {
			pi, pj := hotels[i].PricePerNight, hotels[j].PricePerNight
			if pi == 0 {
				return false
			}
			if pj == 0 {
				return true
			}
			return pi < pj
		})
	case "rating":
		sort.SliceStable(hotels, func(i, j int) bool { return hotels[i].Rating > hotels[j].Rating })
	case "reviews":
		sort.SliceStable(hotels, func(i, j int) bool { return hotels[i].Reviews > hotels[j].Reviews })
	}
	// page / limit
	if opts.Page > 1 && opts.Limit > 0 {
		start := (opts.Page - 1) * opts.Limit
		if start >= len(hotels) {
			return nil
		}
		hotels = hotels[start:]
	}
	if opts.Limit > 0 && len(hotels) > opts.Limit {
		hotels = hotels[:opts.Limit]
	}
	return hotels
}

// fetchAndParseHotels runs the full search flow: build URL, fetch HTML,
// parse, filter. Honours VERIFY (print-would-call) and DOGFOOD (force
// small limit) gates.
func fetchAndParseHotels(ctx context.Context, location, checkin, checkout string, opts hotelSearchOpts) ([]parser.Hotel, string, error) {
	extras := buildHotelsURLParams(opts)
	wouldURL := buildHotelsRequestURL(location, checkin, checkout, extras)
	if cliutil.IsVerifyEnv() {
		return nil, wouldURL, nil
	}
	if cliutil.IsDogfoodEnv() && (opts.Limit == 0 || opts.Limit > 5) {
		opts.Limit = 5
	}
	client := &http.Client{Timeout: 45 * time.Second}
	html, err := parser.FetchHotelsHTML(ctx, client, location, checkin, checkout, extras)
	if err != nil {
		return nil, wouldURL, err
	}
	hotels, err := parser.ParseSearchPage(html)
	if err != nil {
		return nil, wouldURL, fmt.Errorf("parse: %w", err)
	}
	return filterHotels(hotels, opts), wouldURL, nil
}

func buildHotelsRequestURL(location, checkin, checkout string, extras map[string]string) string {
	v := url.Values{}
	v.Set("q", "hotels in "+location)
	v.Set("checkin", checkin)
	v.Set("checkout", checkout)
	if _, has := extras["hl"]; !has {
		v.Set("hl", "en")
	}
	for k, val := range extras {
		if val != "" {
			v.Set(k, val)
		}
	}
	return "https://www.google.com/travel/search?" + v.Encode()
}

// validateYYYYMMDD returns a usage-friendly error when s isn't YYYY-MM-DD.
func validateYYYYMMDD(label, s string) error {
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return usageErr(fmt.Errorf("%s must be YYYY-MM-DD, got %q", label, s))
	}
	return nil
}

var ymdRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func looksLikeDate(s string) bool { return ymdRE.MatchString(s) }

// parseChildAges accepts "5,8,12" -> [5,8,12].
func parseChildAges(s string) []int {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []int
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		var n int
		fmt.Sscanf(part, "%d", &n)
		if n > 0 || part == "0" {
			out = append(out, n)
		}
	}
	return out
}

// haversineMiles is the great-circle distance in statute miles between
// two (lat, lng) pairs. ~10 LoC — used by the `near` command.
func haversineMiles(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 3958.7613 // earth radius in miles
	rad := math.Pi / 180.0
	dLat := (lat2 - lat1) * rad
	dLon := (lon2 - lon1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return r * c
}

// hotelsEnvelope is the standard response wrapper: {meta:{source,...}, results:[...]}.
// Mirrors flight-goat's shape so a future travel-pp-cli orchestrator can
// stack flights and hotels without dispatch-specific code.
type hotelsEnvelope struct {
	Meta    map[string]any  `json:"meta"`
	Results []parser.Hotel  `json:"results"`
	Extra   json.RawMessage `json:"extra,omitempty"`
}

func newEnvelope(source string, results []parser.Hotel) hotelsEnvelope {
	return hotelsEnvelope{
		Meta:    map[string]any{"source": source, "fetched_at": time.Now().UTC().Format(time.RFC3339), "parser_version": parser.ParserVersion, "count": len(results)},
		Results: results,
	}
}

// recordSnapshotsForResults writes one price_snapshots row per priced
// hotel in `results`. Wrapped in a single transaction. Errors don't fail
// the user-facing command — we log to stderr and continue.
func recordSnapshotsForResults(ctx context.Context, results []parser.Hotel, checkin, checkout string, stderr writer) {
	if len(results) == 0 {
		return
	}
	dbPath := defaultDBPath("hotel-goat-pp-cli")
	s, err := openStoreEnsured(ctx, dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "warning: open store for snapshot: %v\n", err)
		return
	}
	defer s.Close()
	failures := 0
	for _, h := range results {
		if h.PricePerNight <= 0 {
			continue
		}
		if err := s.RecordPriceSnapshot(ctx, h.PropertyToken, h.Name, checkin, checkout, h.Currency, h.PricePerNight); err != nil {
			failures++
			// "Log and continue" per the function's contract — a single
			// row-level write failure shouldn't silently abandon the
			// rest of the batch and leave drift history quietly
			// incomplete. Aggregate the count at the end so the
			// stderr surface stays one line per call.
			if failures == 1 {
				fmt.Fprintf(stderr, "warning: record snapshot: %v\n", err)
			}
			continue
		}
	}
	if failures > 1 {
		fmt.Fprintf(stderr, "warning: %d additional snapshot writes failed in this batch\n", failures-1)
	}
}

// writer is a small interface so callers can pass cmd.ErrOrStderr() without
// pulling in io.
type writer interface {
	Write(p []byte) (n int, err error)
}

// openStoreEnsured opens the SQLite store at dbPath and runs the
// hotel-goat hand-built table migrations. Safe to call before every
// hand-written command that touches price_snapshots / brand_aliases /
// wishlist.
func openStoreEnsured(ctx context.Context, dbPath string) (*store.Store, error) {
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	if err := s.EnsureHotelGoatTables(ctx); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}
