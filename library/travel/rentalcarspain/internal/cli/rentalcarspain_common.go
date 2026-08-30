// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

// Shared helpers for the hand-built Rental Car Spain commands. Kept in its own file so
// generate --force preserves it.

package cli

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/rentalcarspain/internal/carsource"
	"github.com/mvanhorn/printing-press-library/library/travel/rentalcarspain/internal/store"
)

// defaultLocationCode is the pickup location used when the caller omits an
// explicit code. The tool is Málaga-focused for now (Málaga Airport = MAL02);
// a later version will add other airports.
const defaultLocationCode = "MAL02"

// resolveSearchArgs interprets the positional args of a location-plus-dates
// command. It accepts either "<pickup> <dropoff>" (location defaults to Málaga
// Airport) or "<location-code> <pickup> <dropoff>". It returns the location
// code, pickup, dropoff, and ok=false when the argument count is unusable.
func resolveSearchArgs(args []string) (location, pickup, dropoff string, ok bool) {
	switch len(args) {
	case 2:
		return defaultLocationCode, args[0], args[1], true
	default:
		if len(args) >= 3 {
			return args[0], args[1], args[2], true
		}
		return "", "", "", false
	}
}

// searchFilters holds the shared filter/sort flags for search-shaped commands.
type searchFilters struct {
	suppliers    string // comma-separated supplier keywords
	class        string // comma-separated car-class substrings
	maxTotal     float64
	maxPerDay    float64
	driverAge    int
	transmission string
	currency     string
	base         bool // show the base (bare) rate instead of full-insurance default
	sortBy       string
	limit        int
}

// resolvedLocation carries the per-source identifiers for one airport.
type resolvedLocation struct {
	DoYouSpainCode string // e.g. MAL02
	RentalcarsIATA string // e.g. AGP
	IATA           string // canonical IATA
	Name           string // display name
}

// resolveLocationInput maps a user location argument to per-source codes. It
// accepts an IATA code (AGP), a DoYouSpain code (MAL02), or an airport name
// (Alicante). Unknown airports still work for DoYouSpain when the raw value
// looks like a DoYouSpain code; Rentalcars needs an IATA to participate.
func resolveLocationInput(location string) resolvedLocation {
	if a, ok := carsource.ResolveAirport(location); ok {
		return resolvedLocation{DoYouSpainCode: a.DoYouSpainCode, RentalcarsIATA: a.IATA, IATA: a.IATA, Name: a.Name}
	}
	loc := strings.TrimSpace(location)
	res := resolvedLocation{Name: loc}
	if len(loc) == 3 { // looks like an IATA code not in the table
		res.RentalcarsIATA = strings.ToUpper(loc)
		res.IATA = strings.ToUpper(loc)
	} else if loc != "" { // assume a DoYouSpain-style code
		res.DoYouSpainCode = loc
	}
	return res
}

// requireDateArgs validates that the pickup/dropoff positional args parse as
// dates (dd/mm/yyyy or yyyy-mm-dd). Commands like delpaso and compare take
// <pickup> <dropoff> with NO location argument, so a mistakenly-passed location
// (e.g. "delpaso AGP 15/09/2026 …") silently became a bad date and produced a
// confusing empty result; this turns that into a clear usage error.
func requireDateArgs(cmdName, pickup, dropoff string) error {
	for _, d := range []struct{ label, val string }{{"pickup", pickup}, {"dropoff", dropoff}} {
		if _, err := parseDMY(d.val); err != nil {
			return usageErr(fmt.Errorf("%q is not a valid %s date — %s takes <pickup> <dropoff> (Málaga only), "+
				"e.g. %s 15/09/2026 22/09/2026", d.val, d.label, cmdName, cmdName))
		}
	}
	return nil
}

// disabledSupplierSet parses --disable-supplier into a normalized lookup set.
// Names are matched case-insensitively against supplier ids (doyouspain,
// rentalcars, delpaso, centauro, drivalia, clickrent, goldcar, cicar,
// autoreisen).
func disabledSupplierSet(flags *rootFlags) map[string]bool {
	set := map[string]bool{}
	if flags == nil {
		return set
	}
	for _, s := range splitCSV(flags.disabledSuppliers) {
		set[strings.ToLower(s)] = true
	}
	return set
}

// supplierEnabled reports whether a supplier (by id) has not been turned off via
// --disable-supplier.
func supplierEnabled(flags *rootFlags, id string) bool {
	return !disabledSupplierSet(flags)[strings.ToLower(id)]
}

// fetchOffers dispatches a search to one or both aggregator sources and returns
// the merged offers plus a per-source error map (empty on full success).
// source is "doyouspain" (default), "rentalcars", or "all". location may be an
// IATA code, a DoYouSpain code, or an airport name.
func fetchOffers(ctx context.Context, flags *rootFlags, source, location, pickup, dropoff, pickupTime, dropoffTime string, age int) ([]carsource.Offer, map[string]error) {
	source = strings.ToLower(strings.TrimSpace(source))
	if source == "" {
		source = "doyouspain"
	}
	wantDYS := (source == "doyouspain" || source == "all") && supplierEnabled(flags, "doyouspain")
	wantRC := (source == "rentalcars" || source == "all") && supplierEnabled(flags, "rentalcars")

	loc := resolveLocationInput(location)
	var merged []carsource.Offer
	errs := map[string]error{}

	if wantDYS {
		if loc.DoYouSpainCode == "" {
			errs["doyouspain"] = fmt.Errorf("no DoYouSpain code known for location %q", location)
		} else {
			dys := carsource.NewDoYouSpain(carHTTPClient(flags))
			q := carsource.SearchQuery{LocationCode: loc.DoYouSpainCode, Pickup: pickup, Dropoff: dropoff, PickupTime: pickupTime, DropoffTime: dropoffTime, DriverAge: age}
			if offers, err := dys.Search(ctx, q); err != nil { // pp:client-call
				errs["doyouspain"] = err
			} else {
				merged = append(merged, offers...)
			}
		}
	}
	if wantRC {
		if loc.RentalcarsIATA == "" {
			errs["rentalcars"] = fmt.Errorf("no Rentalcars IATA known for location %q (pass an IATA code)", location)
		} else {
			rc := carsource.NewRentalcars(carHTTPClient(flags))
			if offers, err := rc.Search(ctx, loc.RentalcarsIATA, pickup, dropoff, pickupTime, dropoffTime, age); err != nil { // pp:client-call
				errs["rentalcars"] = err
			} else {
				merged = append(merged, offers...)
			}
		}
	}
	return merged, errs
}

// sourceErrorsError turns a set of per-source failures into a single classified
// error: a throttling error (exit code 7) when any source was rate-limited, an
// API error (exit code 5) otherwise, or nil when there were none.
func sourceErrorsError(errs map[string]error) error {
	if len(errs) == 0 {
		return nil
	}
	parts := make([]string, 0, len(errs))
	throttled := false
	for src, e := range errs {
		if carsource.IsRateLimit(e) {
			throttled = true
		}
		parts = append(parts, fmt.Sprintf("%s: %s", src, e.Error()))
	}
	sort.Strings(parts)
	agg := fmt.Errorf("all sources failed: %s", strings.Join(parts, "; "))
	if throttled {
		return rateLimitErr(agg)
	}
	return apiErr(agg)
}

// ratingCacheTTL is how long a cached supplier rating is trusted before it is
// treated as stale: expired entries are purged and never override a rating seen
// live in the current search.
const ratingCacheTTL = 14 * 24 * time.Hour

// reviewInfo is a supplier's customer rating for an airport.
type reviewInfo struct {
	Score  float64 `json:"score"`
	Count  int     `json:"count"`
	Source string  `json:"source"` // "rentalcars" | "doyouspain"
	// Stale marks a rating that came only from a cache entry older than
	// ratingCacheTTL (surfaced so callers can flag it).
	Stale bool `json:"stale,omitempty"`
}

// buildReviewIndex collects the best available rating per canonical supplier
// from a merged offer set, preferring Rentalcars over DoYouSpain.
func buildReviewIndex(offers []carsource.Offer) map[string]reviewInfo {
	idx := map[string]reviewInfo{}
	for _, o := range offers {
		if o.SupplierScore <= 0 {
			continue
		}
		key := carsource.CanonicalSupplier(o.Supplier)
		cur, ok := idx[key]
		// Prefer Rentalcars; otherwise keep the entry with more reviews.
		better := !ok ||
			(o.Source == "rentalcars" && cur.Source != "rentalcars") ||
			(o.Source == cur.Source && o.Reviews > cur.Count)
		if better {
			idx[key] = reviewInfo{Score: o.SupplierScore, Count: o.Reviews, Source: o.Source}
		}
	}
	return idx
}

// reviewIndexCached builds the review index from the current offers, persists
// any ratings seen to the per-airport cache, then overlays the cache so ratings
// that a given search's HTML omitted (DoYouSpain renders supplier scores
// inconsistently) still appear. Falls back to the in-memory index on any store
// error.
func reviewIndexCached(ctx context.Context, flags *rootFlags, airport, pickup, dropoff, pickupTime, dropoffTime string, age int, offers []carsource.Offer) map[string]reviewInfo {
	idx := buildReviewIndex(offers)
	// Enrich with Rentalcars' fuller product-cards ratings (covers more
	// companies — e.g. Goldcar, Hertz — than the depot ratings on offers).
	if iata := resolveLocationInput(airport).RentalcarsIATA; iata != "" && pickup != "" {
		rc := carsource.NewRentalcars(carHTTPClient(flags))
		for _, sr := range rc.SupplierRatingsAt(ctx, iata, pickup, dropoff, pickupTime, dropoffTime, age) { // pp:client-call
			cur, ok := idx[sr.Supplier]
			if !ok || sr.Reviews >= cur.Count {
				idx[sr.Supplier] = reviewInfo{Score: sr.Score, Count: sr.Reviews, Source: "rentalcars"}
			}
		}
	}
	if airport == "" {
		return idx
	}
	db, err := store.OpenWithContext(ctx, defaultDBPath("rentalcarspain-pp-cli"))
	if err != nil {
		return idx
	}
	defer db.Close()
	// Expire stale entries so they neither override fresh data nor accumulate.
	_, _ = db.PurgeStaleSupplierRatings(ctx, ratingCacheTTL)
	// Persist what we saw live this run (always fresh).
	toStore := make([]store.SupplierRating, 0, len(idx))
	for sup, ri := range idx {
		toStore = append(toStore, store.SupplierRating{Supplier: sup, Score: ri.Score, Reviews: ri.Count, Source: ri.Source})
	}
	_ = db.UpsertSupplierRatings(ctx, airport, toStore)
	// Overlay the cache: fill gaps with entries still within the freshness
	// window. A live rating from this run always wins over cache; a cache entry
	// only fills a gap or replaces a lower-confidence one, and is flagged stale
	// if it is older than the TTL (it survived the purge only when the purge
	// failed, so mark it defensively).
	cached, err := db.GetSupplierRatings(ctx, airport)
	if err != nil {
		return idx
	}
	live := map[string]bool{}
	for sup := range idx {
		live[sup] = true
	}
	for _, c := range cached {
		stale := !c.UpdatedAt.IsZero() && time.Since(c.UpdatedAt) > ratingCacheTTL
		if stale {
			continue // never surface a rating past its freshness window
		}
		if live[c.Supplier] {
			continue // a rating seen live this run already wins
		}
		cur, ok := idx[c.Supplier]
		if !ok || c.Reviews > cur.Count {
			idx[c.Supplier] = reviewInfo{Score: c.Score, Count: c.Reviews, Source: c.Source, Stale: false}
		}
	}
	return idx
}

// carSize classifies an offer as "small" or "bigger" using the ACRISS class
// (Rentalcars/Drivalia), DoYouSpain/Centauro text categories, and a keyword/
// seat fallback. Used to pick one small + one bigger car in `recommend`.
func carSize(o carsource.Offer) string {
	hay := strings.ToLower(o.CarClass + " " + o.Car)
	switch {
	case strings.Contains(hay, "suv"), strings.Contains(hay, "estate"),
		strings.Contains(hay, "people carrier"), strings.Contains(hay, "minivan"),
		strings.Contains(hay, "7 seat"), strings.Contains(hay, "9 seat"),
		strings.Contains(hay, "cabrio"), strings.Contains(hay, "premium"),
		strings.Contains(hay, "van"):
		return "bigger"
	case strings.Contains(hay, "small"), strings.Contains(hay, "mini"),
		strings.Contains(hay, "economy"):
		return "small"
	}
	if o.Seats >= 7 {
		return "bigger"
	}
	// ACRISS: first char is the category (M/N mini, E economy, H, C compact →
	// small; I/J/R/S/F/P/L/X/U → bigger). Second char W/V/S/F/P/X → SUV/estate.
	class := strings.ToUpper(strings.TrimSpace(o.CarClass))
	if len(class) >= 4 { // looks like an ACRISS code (e.g. EDMR)
		switch class[0] {
		case 'M', 'N', 'E', 'H', 'C':
			if len(class) >= 2 {
				switch class[1] {
				case 'W', 'V', 'F', 'P', 'X', 'S':
					return "bigger"
				}
			}
			return "small"
		case 'I', 'J', 'R', 'S', 'F', 'P', 'L', 'X', 'U', 'D', 'W', 'G':
			return "bigger"
		}
	}
	return "small"
}

// classFilterMatch reports whether an offer matches the --class filter: a
// comma-separated list of brand/type/size substrings (e.g. "bmw,cabrio",
// "suv", "alfa romeo"), matched against the car's model and class, with
// ACRISS-code decoding for body/size so Rentalcars offers that carry only a
// code still match. An empty filter matches every offer.
func classFilterMatch(o carsource.Offer, classCSV string) bool {
	keys := splitCSV(classCSV)
	if len(keys) == 0 {
		return true
	}
	return anyClass(o.CarClass, o.Car, keys)
}

// carHTTPClient builds an HTTP client bounded by the root --timeout and paced by
// a per-host adaptive rate limiter (honoring --rate-limit as a hard per-host
// ceiling), so fanning out across suppliers never hammers any single site. The
// source clients copy this client and set their own cookie jar, which preserves
// the injected transport.
func carHTTPClient(flags *rootFlags) *http.Client {
	to := time.Duration(0)
	maxRate := 0.0
	noCache := false
	if flags != nil {
		to = flags.timeout
		maxRate = flags.rateLimit
		noCache = flags.noCache
	}
	if to <= 0 {
		to = 60 * time.Second // matching the root default
	}
	// Transport chain: static-lookup cache → per-host rate limit → network. A
	// cache hit skips both the rate limiter and the network; prices are never
	// cached (see lookup_cache.go), only office/location lookups.
	var rt http.RoundTripper = &perHostRateLimitTransport{base: http.DefaultTransport, reg: sharedHostRegistry(maxRate)}
	rt = wrapLookupCache(rt, noCache)
	return &http.Client{Timeout: to, Transport: rt}
}

// applyFilters filters and sorts offers per the given flags.
func (f *searchFilters) apply(offers []carsource.Offer) []carsource.Offer {
	out := make([]carsource.Offer, 0, len(offers))
	supKeys := splitCSV(f.suppliers)
	classKeys := splitCSV(f.class)
	for _, o := range offers {
		if len(supKeys) > 0 && !anySupplier(o.Supplier, supKeys) {
			continue
		}
		if len(classKeys) > 0 && !anyClass(o.CarClass, o.Car, classKeys) {
			continue
		}
		price := offerPrice(o, f.base)
		if f.maxTotal > 0 && o.Total > f.maxTotal {
			continue
		}
		if f.maxPerDay > 0 && o.PerDay > f.maxPerDay {
			continue
		}
		if f.transmission != "" && !strings.EqualFold(o.Transmission, normTransmission(f.transmission)) {
			continue
		}
		_ = price
		out = append(out, o)
	}
	sortOffers(out, f.sortBy, f.base)
	if f.limit > 0 && len(out) > f.limit {
		out = out[:f.limit]
	}
	return out
}

func offerPrice(o carsource.Offer, base bool) float64 {
	if base && o.BaseTotal > 0 {
		return o.BaseTotal
	}
	return o.Total
}

func sortOffers(offers []carsource.Offer, by string, base bool) {
	switch by {
	case "per-day":
		sort.SliceStable(offers, func(i, j int) bool { return offers[i].PerDay < offers[j].PerDay })
	case "supplier":
		sort.SliceStable(offers, func(i, j int) bool { return offers[i].Supplier < offers[j].Supplier })
	case "total", "cheapest", "":
		fallthrough
	default:
		sort.SliceStable(offers, func(i, j int) bool { return offers[i].Total < offers[j].Total })
	}
}

func anySupplier(supplier string, keys []string) bool {
	for _, k := range keys {
		if carsource.MatchesSupplier(supplier, k) {
			return true
		}
	}
	return false
}

// acrissBody maps a friendly class keyword to the ACRISS type/body codes it
// covers (the 2nd character of a code like IFAR).
var acrissBody = map[string]string{
	"suv": "FGJ", "crossover": "G", "4x4": "FJ", "off-road": "J",
	"estate": "W", "wagon": "W",
	"van": "VMK", "minivan": "V", "people carrier": "VM", "monospace": "M",
	"convertible": "T", "cabrio": "T", "cabriolet": "T",
	"coupe": "E", "pickup": "PQ", "sport": "S",
}

// acrissSize maps a friendly size keyword to the ACRISS category codes it
// covers (the 1st character of a code like IFAR).
var acrissSize = map[string]string{
	"mini": "MN", "small": "MNEH", "economy": "EH",
	"compact": "CD", "intermediate": "IJ", "medium": "IJ",
	"standard": "SR", "large": "FG", "fullsize": "FG", "full size": "FG",
	"premium": "PU", "luxury": "LW",
}

// acrissTransmission / acrissFuel are the valid 3rd and 4th characters of an
// ACRISS code. They are what distinguish a real code ("CFMR") from an ordinary
// word that happens to be four letters ("LARGe cars", "AUTOmatic").
const (
	acrissTransmission = "MNCABD"
	acrissFuel         = "RNDQHIECLSABMFVZUX"
)

// looksACRISS reports whether a car-class string is an ACRISS code (as
// Rentalcars emits) rather than a human label (as DoYouSpain emits). Codes are
// a single token whose first four characters are A–Z with a valid transmission
// and fuel/AC character in positions 3 and 4.
func looksACRISS(s string) bool {
	s = strings.ToUpper(strings.TrimSpace(s))
	if len(s) < 4 || strings.ContainsAny(s, " \t-/") {
		return false
	}
	for i := 0; i < 4; i++ {
		if s[i] < 'A' || s[i] > 'Z' {
			return false
		}
	}
	return strings.IndexByte(acrissTransmission, s[2]) >= 0 &&
		strings.IndexByte(acrissFuel, s[3]) >= 0
}

// acrissMatches reports whether an ACRISS code satisfies a friendly class
// keyword, so "--class suv" also selects Rentalcars offers (IFAR, CFMR…) whose
// code carries no human-readable category text.
func acrissMatches(class, key string) bool {
	c := strings.ToUpper(strings.TrimSpace(class))
	if !looksACRISS(c) {
		return false
	}
	if bodies, ok := acrissBody[key]; ok && strings.IndexByte(bodies, c[1]) >= 0 {
		return true
	}
	if sizes, ok := acrissSize[key]; ok && strings.IndexByte(sizes, c[0]) >= 0 {
		return true
	}
	return false
}

// anyClass reports whether an offer matches any of the requested class keys.
// It matches DoYouSpain's human categories ("SUVs", "Small Cars") and model
// names by substring, and Rentalcars' ACRISS codes by decoding them — without
// the latter, "--class suv" silently dropped every Rentalcars offer.
func anyClass(class, car string, keys []string) bool {
	hay := strings.ToLower(class + " " + car)
	for _, k := range keys {
		kl := strings.ToLower(strings.TrimSpace(k))
		if kl == "" {
			continue
		}
		if strings.Contains(hay, kl) {
			return true
		}
		if acrissMatches(class, kl) {
			return true
		}
	}
	return false
}

func normTransmission(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if strings.HasPrefix(s, "a") {
		return "Automatic"
	}
	return "Manual"
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// cheapest returns the lowest-total offer, or false when the slice is empty.
func cheapest(offers []carsource.Offer) (carsource.Offer, bool) {
	if len(offers) == 0 {
		return carsource.Offer{}, false
	}
	best := offers[0]
	for _, o := range offers[1:] {
		if o.Total > 0 && (best.Total == 0 || o.Total < best.Total) {
			best = o
		}
	}
	return best, true
}

// searchKey builds a stable snapshot key from search parameters. The drop-off
// location code is part of the identity: a one-way rental (different drop-off
// office) is a different route with a different price than the round-trip, so
// their price histories must not share a key. Round-trip callers pass "".
func searchKey(locationCode, dropoffCode, pickup, dropoff string, driverAge int) string {
	return fmt.Sprintf("%s|%s|%s|%s|%d", locationCode, dropoffCode, pickup, dropoff, driverAge)
}

// youngDriverAgeThreshold is the age below which suppliers typically apply a
// young-driver surcharge. Those surcharges are collected at the rental counter
// and are NOT included in the online zero-excess quotes this tool reports —
// verified against Clickrent, Centauro, Drivalia and Goldcar, whose online
// prices do not change between ages 23 and 35.
const youngDriverAgeThreshold = 25

// youngDriverNotice returns a caveat when the driver age is set below the
// young-driver threshold, warning that under-25 surcharges are charged at the
// counter and are not reflected in the quoted totals. It returns "" for
// standard-age drivers (age 0 means "unspecified", treated as standard).
func youngDriverNotice(age int) string {
	if age <= 0 || age >= youngDriverAgeThreshold {
		return ""
	}
	return fmt.Sprintf("Note: for drivers under %d, totals now include each supplier's "+
		"obligatory young-driver surcharge — read live where the supplier prices it online "+
		"(Centauro, Drivalia) or from its published rate otherwise (Delpaso: €12/day). "+
		"Others instead collect it at the rental counter, not shown here (e.g. Clickrent "+
		"+€7.95/day), and some suppliers may decline very young drivers or limit them to "+
		"certain car groups.", youngDriverAgeThreshold)
}

// ageEligibilityNote returns a short flag when the driver's age is below an
// offer's stated minimum (e.g. "min age 21"), or "" when the age is unspecified,
// the offer states no minimum, or the driver meets it. It lets views show an
// offer the driver cannot actually rent while flagging the restriction, rather
// than hiding it or quoting it as if bookable.
func ageEligibilityNote(o carsource.Offer, age int) string {
	if age <= 0 || o.MinAge <= 0 || age >= o.MinAge {
		return ""
	}
	return fmt.Sprintf("min age %d", o.MinAge)
}

// carCellWithAge renders a car name for a table cell, appending an age-eligibility
// flag when the driver is below the offer's minimum age.
func carCellWithAge(o carsource.Offer, age, width int) string {
	cell := truncate(o.Car, width)
	if note := ageEligibilityNote(o, age); note != "" {
		cell += " [" + note + "]"
	}
	return cell
}

// recordSnapshotFor opens the store and appends a snapshot of the cheapest
// prices for a search. Failures are non-fatal (best-effort local memory).
func recordSnapshotFor(ctx context.Context, flags *rootFlags, key string, offers []carsource.Offer) {
	if len(offers) == 0 {
		return
	}
	db, err := store.OpenWithContext(ctx, defaultDBPath("rentalcarspain-pp-cli"))
	if err != nil {
		return
	}
	defer db.Close()
	best, _ := cheapest(offers)
	snap := store.PriceSnapshot{
		SearchKey:       key,
		CheapestTotal:   best.Total,
		CheapestFITotal: best.Total, // full-insurance is the default price basis
		Currency:        "EUR",
		OfferCount:      len(offers),
	}
	_ = db.RecordSnapshot(ctx, snap)
}
