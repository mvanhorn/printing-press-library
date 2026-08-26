// Copyright 2026 Victor Wibisono and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/agoda/internal/agoda"
	"github.com/mvanhorn/printing-press-library/library/travel/agoda/internal/cliutil"
)

// searchFlags is the flag set shared by every destination-and-dates command.
// Keeping one struct means `hotels search`, `hotels rank`, `hotels fees`, and
// `vip delta` cannot drift apart in flag names or defaults.
type searchFlags struct {
	cityID   int
	checkin  string
	nights   int
	rooms    int
	adults   int
	children int
	currency string
	limit    int
	sort     string
}

const (
	defaultNights   = 2
	defaultAdults   = 2
	defaultRooms    = 1
	defaultCurrency = "USD"
	defaultLimit    = 20
)

// newAgodaClient builds a paced client. Cookie import is optional: every public
// command works anonymously, and only member-priced surfaces need a session.
func newAgodaClient(flags *rootFlags) *agoda.Client {
	timeout := 60 * time.Second
	if flags != nil && flags.timeout > 0 {
		timeout = flags.timeout
	}
	c := agoda.New(timeout)
	c.Cookie = agodaCookie()
	return c
}

// resolveCity turns a destination argument into a city id.
//
// A bare integer is treated as an id so callers can skip the lookup round-trip;
// anything else goes through Agoda's autocomplete.
func resolveCity(ctx context.Context, c *agoda.Client, arg string, explicitID int) (agoda.Destination, error) {
	if explicitID > 0 {
		return agoda.Destination{CityID: explicitID, Name: fmt.Sprintf("city %d", explicitID)}, nil
	}
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return agoda.Destination{}, fmt.Errorf("a destination is required")
	}
	if id, err := parsePositiveInt(arg); err == nil {
		return agoda.Destination{CityID: id, Name: fmt.Sprintf("city %d", id)}, nil
	}
	return c.ResolveCityID(ctx, arg)
}

func parsePositiveInt(s string) (int, error) {
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil || n <= 0 {
		return 0, fmt.Errorf("not a positive integer")
	}
	// Reject values like "12abc" that Sscanf would accept as a prefix.
	if fmt.Sprintf("%d", n) != strings.TrimSpace(s) {
		return 0, fmt.Errorf("not a positive integer")
	}
	return n, nil
}

// searchOptions normalizes flags into client options, filling defaults and
// defaulting the check-in date to a near-future date when the caller omits one.
func (sf *searchFlags) searchOptions(cityID int, authenticated bool) (agoda.SearchOptions, error) {
	checkin := strings.TrimSpace(sf.checkin)
	if checkin == "" {
		checkin = time.Now().AddDate(0, 0, 30).Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", checkin); err != nil {
		return agoda.SearchOptions{}, usageErr(fmt.Errorf("--checkin must be YYYY-MM-DD, got %q", sf.checkin))
	}
	nights := sf.nights
	if nights <= 0 {
		nights = defaultNights
	}
	adults := sf.adults
	if adults <= 0 {
		adults = defaultAdults
	}
	rooms := sf.rooms
	if rooms <= 0 {
		rooms = defaultRooms
	}
	currency := strings.ToUpper(strings.TrimSpace(sf.currency))
	if currency == "" {
		currency = defaultCurrency
	}
	field, order, err := resolveSort(sf.sort)
	if err != nil {
		return agoda.SearchOptions{}, err
	}
	opts := agoda.SearchOptions{
		SortField:     field,
		SortOrder:     order,
		CityID:        cityID,
		CheckIn:       checkin,
		Nights:        nights,
		Rooms:         rooms,
		Adults:        adults,
		Children:      sf.children,
		Currency:      currency,
		Locale:        "en-us",
		Origin:        "US",
		Authenticated: authenticated,
	}
	return opts, nil
}

// agodaSearchResult is the envelope every search-shaped command emits.
//
// Scanned/returned counts are reported explicitly so an agent can tell a genuine
// "nothing matched" apart from "we only looked at a handful of properties".
type agodaSearchResult struct {
	Destination        string           `json:"destination"`
	CityID             int              `json:"city_id"`
	CheckIn            string           `json:"checkin"`
	Nights             int              `json:"nights"`
	Currency           string           `json:"currency"`
	ScannedProperties  int              `json:"scanned_properties"`
	ReturnedProperties int              `json:"returned_properties"`
	Results            []agoda.Property `json:"results"`
	Note               string           `json:"note,omitempty"`
}

// runSearch is the shared fetch path: resolve the destination, run the search,
// drop unpriced rows, and apply the caller's limit.
func runSearch(ctx context.Context, c *agoda.Client, errOut io.Writer, dest string, sf *searchFlags, authenticated bool) (agodaSearchResult, []agoda.Property, error) {
	d, err := resolveCity(ctx, c, dest, sf.cityID)
	if err != nil {
		return agodaSearchResult{}, nil, err
	}
	opts, err := sf.searchOptions(d.CityID, authenticated)
	if err != nil {
		return agodaSearchResult{}, nil, err
	}
	props, err := c.CitySearch(ctx, opts)
	if err != nil {
		return agodaSearchResult{}, nil, err
	}
	priced := make([]agoda.Property, 0, len(props))
	for _, p := range props {
		if p.PriceAllIn > 0 {
			priced = append(priced, p)
		}
	}
	// Record what this search saw so the offline corpus grows with ordinary
	// use. Best-effort by design: a cache write must never fail a live search.
	rememberProperties(ctx, errOut, defaultDBPath("agoda-pp-cli"), d.CityID, priced)

	res := agodaSearchResult{
		Destination:       displayDestination(d, dest),
		CityID:            d.CityID,
		CheckIn:           opts.CheckIn,
		Nights:            opts.Nights,
		Currency:          opts.Currency,
		ScannedProperties: len(props),
	}
	if len(priced) == 0 {
		res.Results = make([]agoda.Property, 0)
		res.Note = "no priced availability returned for these dates; try a different date range or widen occupancy"
	}
	return res, priced, nil
}

func displayDestination(d agoda.Destination, fallback string) string {
	if d.Name != "" && !strings.HasPrefix(d.Name, "city ") {
		return d.Name
	}
	if strings.TrimSpace(fallback) != "" {
		return strings.TrimSpace(fallback)
	}
	return d.Name
}

// applyLimit trims to the caller's limit after any local sorting has run, so the
// limit always describes the final ranked list.
func applyLimit(props []agoda.Property, limit int) []agoda.Property {
	if limit <= 0 || limit >= len(props) {
		return props
	}
	return props[:limit]
}

func sortByAllIn(props []agoda.Property) {
	sort.SliceStable(props, func(i, j int) bool {
		return props[i].PriceAllIn < props[j].PriceAllIn
	})
}

// isRateLimited reports whether an error is a throttle rather than a data gap.
// Callers must not swallow these into an empty result.
func isRateLimited(err error) bool {
	var rl *cliutil.RateLimitError
	return errorsAs(err, &rl)
}

// agodaSortAliases maps friendly CLI values to Agoda's sort vocabulary.
//
// Only Ranking, Price, and Distance are accepted by the API; other spellings
// return an opaque HTTP 400, so the set is closed deliberately rather than
// passed through.
var agodaSortAliases = map[string][2]string{
	"":           {"Ranking", "Desc"},
	"ranking":    {"Ranking", "Desc"},
	"price-asc":  {"Price", "Asc"},
	"price":      {"Price", "Asc"},
	"price-desc": {"Price", "Desc"},
	"distance":   {"Distance", "Asc"},
}

// resolveSort validates the --sort value up front so an unsupported spelling
// fails with a usable message instead of Agoda's opaque 400.
func resolveSort(v string) (string, string, error) {
	key := strings.ToLower(strings.TrimSpace(v))
	if key == "true-price" {
		// Handled locally after fetching, since Agoda cannot sort by the
		// all-in figure. Ask for its default ranking and re-sort in Go.
		return "Ranking", "Desc", nil
	}
	pair, ok := agodaSortAliases[key]
	if !ok {
		return "", "", usageErr(fmt.Errorf(
			"--sort %q is not supported; use one of: ranking, price-asc, price-desc, distance, true-price", v))
	}
	return pair[0], pair[1], nil
}
