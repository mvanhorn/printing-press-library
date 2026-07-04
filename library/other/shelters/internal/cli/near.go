// Copyright 2026 Abe Diaz (@abe238) and contributors. Licensed under Apache-2.0. See LICENSE.
//
// The `near <location>` command: closest open shelters to a point, with optional
// pet / accessibility filters. Coordinates are frequently null in the feed even
// for open shelters, so any shelter missing lat/lon is geocoded from its street
// address (US Census, free, cached, bounded concurrency). A shelter that cannot
// be located is reported in a count, never silently dropped and never errored.

package cli

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

// maxGeocode bounds how many distinct addresses `near` will geocode in one run,
// so an active-event feed of hundreds cannot stall behind unbounded network
// calls. When the cap is hit it is reported, never hidden.
const maxGeocode = 400

// geocodeWorkers is the bounded concurrency for address geocoding.
const geocodeWorkers = 8

var latLonRe = regexp.MustCompile(`^\s*(-?\d+(?:\.\d+)?)\s*,\s*(-?\d+(?:\.\d+)?)\s*$`)

// shelterDistance is a shelter plus its computed distance from the origin.
type shelterDistance struct {
	Shelter
	DistanceMiles  float64 `json:"distance_miles"`
	CoordsGeocoded bool    `json:"coords_geocoded"`
}

// nearData is the near command payload.
type nearData struct {
	Origin          originInfo        `json:"origin"`
	LocatedCount    int               `json:"located_count"`
	ReturnedCount   int               `json:"returned_count"`
	UnlocatedCount  int               `json:"unlocated_count"`
	Unlocated       []unlocatedInfo   `json:"unlocated,omitempty"`
	GeocodeCapped   bool              `json:"geocode_capped"`
	GeocodeTimedOut bool              `json:"geocode_timed_out"`
	Note            string            `json:"note"`
	Shelters        []shelterDistance `json:"shelters"`
}

type originInfo struct {
	Query     string  `json:"query"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Geocoded  bool    `json:"geocoded"`
}

type unlocatedInfo struct {
	ShelterID int    `json:"shelter_id"`
	Name      string `json:"shelter_name"`
	Address   string `json:"address"`
}

// pp:data-source auto
func newNovelNearCmd(flags *rootFlags) *cobra.Command {
	var sf shelterFilter
	var flagLimit int
	var flagMaxMiles float64
	var flagFixture string

	cmd := &cobra.Command{
		Use:   "near <location>",
		Short: "Closest open shelters to a location, with optional pet / accessibility filters",
		Long: "Find the open shelters closest to a location. <location> may be 'lat,lon', a ZIP, or a " +
			"full street address. Shelters missing coordinates (common in the feed) are geocoded from " +
			"their street address via the free US Census geocoder; any that cannot be located are " +
			"counted, never dropped. Distances are straight-line miles, not driving distance.\n\n" +
			"Use case: \"the closest shelter to me that allows pets\" -> add --pets.",
		Example: "  shelters-pp-cli near 78566 --pets\n" +
			"  shelters-pp-cli near \"2400 W Bradley Ave, Champaign, IL\" --limit 3\n" +
			"  shelters-pp-cli near 29.76,-95.37 --ada --max-miles 50",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("near: a location is required (lat,lon | ZIP | street address)"))
			}
			loc := strings.TrimSpace(args[0])

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			origin, err := resolveOrigin(ctx, loc)
			if err != nil {
				return err
			}

			source, shelters, err := loadShelterFeed(cmd, flags, flagFixture)
			if err != nil {
				return err
			}
			shelters = sf.apply(shelters)

			data := buildNear(ctx, origin, shelters, flagMaxMiles, flagLimit)
			return emitEnvelopeHuman(cmd, flags, source, data, func() string {
				return renderNear(data)
			})
		},
	}
	cmd.Flags().BoolVar(&sf.pets, "pets", false, "Only shelters that allow pets (pet code COHABIT or ONSITE)")
	cmd.Flags().BoolVar(&sf.ada, "ada", false, "Only shelters confirmed ADA compliant")
	cmd.Flags().BoolVar(&sf.wheelchair, "wheelchair", false, "Only shelters confirmed wheelchair accessible")
	cmd.Flags().IntVar(&flagLimit, "limit", 5, "Number of closest shelters to return (0 = all)")
	cmd.Flags().Float64Var(&flagMaxMiles, "max-miles", 0, "Only shelters within this many straight-line miles (0 = no cap)")
	cmd.Flags().StringVar(&flagFixture, "fixture", "", "Parse a saved feed JSON (path or - for stdin) instead of fetching live")
	return cmd
}

// resolveOrigin turns the location arg into coordinates. "lat,lon" is parsed
// directly (no network); anything else is geocoded.
func resolveOrigin(ctx context.Context, loc string) (originInfo, error) {
	if m := latLonRe.FindStringSubmatch(loc); m != nil {
		lat, _ := strconv.ParseFloat(m[1], 64)
		lon, _ := strconv.ParseFloat(m[2], 64)
		if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
			return originInfo{}, usageErr(fmt.Errorf("near: lat,lon out of range: %q", loc))
		}
		return originInfo{Query: loc, Latitude: lat, Longitude: lon, Geocoded: false}, nil
	}
	ll, ok, err := geocodeOneLine(ctx, loc)
	if err != nil {
		return originInfo{}, apiErr(fmt.Errorf("geocoding origin %q: %w", loc, err))
	}
	if !ok {
		return originInfo{}, usageErr(fmt.Errorf("near: could not geocode location %q; try 'lat,lon' or a full street address", loc))
	}
	return originInfo{Query: loc, Latitude: ll.Lat, Longitude: ll.Lon, Geocoded: true}, nil
}

// buildNear resolves shelter coordinates (geocoding those with null lat/lon),
// computes distances, sorts ascending, and applies max-miles + limit.
func buildNear(ctx context.Context, origin originInfo, shelters []Shelter, maxMiles float64, limit int) nearData {
	coords, geocoded, unlocated, capped := resolveShelterCoords(ctx, shelters)
	timedOut := ctx.Err() != nil

	located := make([]shelterDistance, 0, len(coords))
	for i, s := range shelters {
		// Keyed by slice index, never shelter_id: shelter_id is a nullable,
		// non-unique attribute (defaults to 0 when absent), so id-keyed maps
		// would let one record's coordinates clobber another's.
		ll, ok := coords[i]
		if !ok {
			continue
		}
		d := haversineMiles(origin.Latitude, origin.Longitude, ll.Lat, ll.Lon)
		if maxMiles > 0 && d > maxMiles {
			continue
		}
		sd := shelterDistance{Shelter: s, DistanceMiles: round1(d), CoordsGeocoded: geocoded[i]}
		// Reflect the resolved coordinates so the output is self-consistent.
		lat, lon := ll.Lat, ll.Lon
		sd.Latitude, sd.Longitude = &lat, &lon
		located = append(located, sd)
	}
	sort.SliceStable(located, func(i, j int) bool { return located[i].DistanceMiles < located[j].DistanceMiles })
	// LocatedCount is the number of shelters that could be ranked (resolved
	// coordinates, within max-miles) BEFORE the --limit truncation, so the
	// caller can tell "5 nearby, showing 2" from "only 2 nearby".
	locatedCount := len(located)
	if limit > 0 && len(located) > limit {
		located = located[:limit]
	}

	d := nearData{
		Origin:          origin,
		LocatedCount:    locatedCount,
		ReturnedCount:   len(located),
		UnlocatedCount:  len(unlocated),
		GeocodeCapped:   capped,
		GeocodeTimedOut: timedOut,
		Shelters:        located,
	}
	for _, s := range unlocated {
		d.Unlocated = append(d.Unlocated, unlocatedInfo{ShelterID: s.ShelterID, Name: s.Name, Address: shelterOneLine(s)})
	}
	d.Note = nearNote(origin, len(unlocated), capped, timedOut)
	return d
}

// resolveShelterCoords returns coordinates for every shelter that can be
// located: directly when lat/lon are present, else by geocoding the address
// (bounded concurrency, cached by one-line address, capped at maxGeocode).
// All maps are keyed by the shelter's INDEX in the input slice (not shelter_id,
// which is nullable and non-unique), so no record can clobber another's coords.
func resolveShelterCoords(ctx context.Context, shelters []Shelter) (coords map[int]latlon, geocoded map[int]bool, unlocated []Shelter, capped bool) {
	coords = map[int]latlon{}
	geocoded = map[int]bool{}

	// Phase 1: direct coordinates. Phase 2: collect distinct addresses to geocode.
	type need struct {
		idx     int
		oneLine string
	}
	var toGeocode []need
	seen := map[string]bool{}
	var distinct []string
	for i, s := range shelters {
		if s.Latitude != nil && s.Longitude != nil {
			coords[i] = latlon{Lat: *s.Latitude, Lon: *s.Longitude}
			continue
		}
		ol := shelterOneLine(s)
		if ol == "" {
			unlocated = append(unlocated, s)
			continue
		}
		toGeocode = append(toGeocode, need{idx: i, oneLine: ol})
		if !seen[ol] {
			seen[ol] = true
			distinct = append(distinct, ol)
		}
	}

	if len(distinct) > maxGeocode {
		capped = true
		distinct = distinct[:maxGeocode]
	}

	// Geocode distinct addresses concurrently into a cache.
	cache := geocodeDistinct(ctx, distinct)

	// Map results back by index; unresolved addresses become unlocated.
	for _, n := range toGeocode {
		if ll, ok := cache[n.oneLine]; ok {
			coords[n.idx] = ll
			geocoded[n.idx] = true
		} else {
			unlocated = append(unlocated, shelters[n.idx])
		}
	}
	return coords, geocoded, unlocated, capped
}

// geocodeDistinct geocodes a set of distinct one-line addresses with bounded
// concurrency, returning only the ones that resolved. ctx cancellation stops
// further work; partial results are still returned.
func geocodeDistinct(ctx context.Context, addrs []string) map[string]latlon {
	out := map[string]latlon{}
	if len(addrs) == 0 {
		return out
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, geocodeWorkers)
	for _, a := range addrs {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(addr string) {
			defer wg.Done()
			defer func() { <-sem }()
			if ctx.Err() != nil {
				return
			}
			ll, ok, err := geocodeOneLine(ctx, addr)
			if err != nil || !ok {
				return
			}
			mu.Lock()
			out[addr] = ll
			mu.Unlock()
		}(a)
	}
	wg.Wait()
	return out
}

func nearNote(origin originInfo, unlocated int, capped, timedOut bool) string {
	parts := []string{"Distances are straight-line miles, not driving distance."}
	if origin.Geocoded {
		parts = append(parts, "Origin was geocoded from the text you gave; a precise 'lat,lon' is most accurate.")
	}
	if unlocated > 0 {
		parts = append(parts, fmt.Sprintf("%d shelter(s) could not be located and are excluded from the ranking.", unlocated))
	}
	if timedOut {
		parts = append(parts, "Geocoding did not finish before the timeout; some shelters listed as unlocated may resolve on a retry (raise --timeout).")
	}
	if capped {
		parts = append(parts, fmt.Sprintf("Address geocoding was capped at %d, chosen by feed order (not distance), so a closer shelter could be among those not geocoded.", maxGeocode))
	}
	return strings.Join(parts, " ")
}

func renderNear(d nearData) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Closest shelters to %s (%.5f, %.5f):\n", d.Origin.Query, d.Origin.Latitude, d.Origin.Longitude)
	if len(d.Shelters) == 0 {
		fmt.Fprintln(&b, "  (no shelters matched; widen --max-miles or drop filters)")
	}
	for _, s := range d.Shelters {
		loc := strings.TrimSpace(s.City + ", " + s.State)
		geo := ""
		if s.CoordsGeocoded {
			geo = " [geocoded]"
		}
		fmt.Fprintf(&b, "- %.1f mi  %s (id %d) -- %s%s\n", s.DistanceMiles, s.Name, s.ShelterID, loc, geo)
		fmt.Fprintf(&b, "      pets %s | ada %s | wheelchair %s | pop/cap %s\n",
			petLabel(s.PetAccommodations), dashIfEmpty(s.ADACompliant), dashIfEmpty(s.WheelchairAccessible), popCapStr(s.Shelter))
	}
	if d.UnlocatedCount > 0 {
		if d.GeocodeTimedOut {
			fmt.Fprintf(&b, "\n%d shelter(s) are not in the ranking; geocoding timed out, so some may resolve on retry.\n", d.UnlocatedCount)
		} else {
			fmt.Fprintf(&b, "\n%d shelter(s) could not be located (no coordinates and address would not geocode).\n", d.UnlocatedCount)
		}
	}
	fmt.Fprintf(&b, "\n%s\n", d.Note)
	return b.String()
}

// round1 rounds to one decimal place using the standard library, which is
// overflow-safe (unlike int64 truncation) for any finite input.
func round1(f float64) float64 {
	return math.Round(f*10) / 10
}
