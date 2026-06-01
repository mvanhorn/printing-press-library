// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

// Hand-authored shared plumbing for the OfferUp novel commands. Lives outside
// the generator-emitted files so it survives regen as a whole unit.

package cli

import (
	"errors"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/offerup/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/offerup/internal/offerup"
)

// searchAndRecord runs a live OfferUp search and best-effort caches the results
// to the local store (store-write failures are non-fatal). Shared by the
// commands whose store write is a side effect rather than load-bearing
// (price-check, deals, listings search). Commands that read the store back
// (new-since, price-drops, digest) open it themselves so they keep the handle.
func searchAndRecord(cmd *cobra.Command, flags *rootFlags, lf *locFlags, query string, opts offerup.SearchOptions) ([]offerup.Listing, error) {
	listings, err := newOfferupClient(flags).Search(cmd.Context(), query, opts)
	if err != nil {
		return nil, classifyOfferupError(err)
	}
	if st, err := openOfferupStore(); err == nil {
		defer st.Close()
		_, _ = st.RecordSearch(lf.storeKey(query), listings)
	}
	return listings, nil
}

// errMissingQuery is returned when listings search is invoked with neither a
// positional query nor --query.
var errMissingQuery = errors.New("a search query is required (positional argument or --query)")

// runAuthRead is the shared body for authenticated read commands. It honors
// --dry-run and verify-mode by emitting the empty value without a network call.
// On a real run it calls fetch and prints the result. Under the live-dogfood
// harness, a missing session (ErrNotLoggedIn from the generated cookie store)
// is a skip — the matrix runs without a logged-in OfferUp session — while in
// normal use a missing session is still a hard auth error (exit 4) with a clear
// "run 'auth login --chrome' (or set OFFERUP_COOKIE)" message.
func runAuthRead(cmd *cobra.Command, flags *rootFlags, empty any, fetch func() (any, error)) error {
	if dryRunOK(flags) {
		return nil
	}
	if cliutil.IsVerifyEnv() {
		return printJSONFiltered(cmd.OutOrStdout(), empty, flags)
	}
	v, err := fetch()
	if err != nil {
		if cliutil.IsDogfoodEnv() && errors.Is(err, offerup.ErrNotLoggedIn) {
			return printJSONFiltered(cmd.OutOrStdout(), empty, flags)
		}
		return classifyOfferupError(err)
	}
	return printJSONFiltered(cmd.OutOrStdout(), v, flags)
}

// classifyOfferupError maps an OfferUp client error to a typed CLI exit code:
// 429 rate-limit exhaustion -> exit 7, everything else -> API error (exit 5).
func classifyOfferupError(err error) error {
	var rle *cliutil.RateLimitError
	if errors.As(err, &rle) {
		return rateLimitErr(err)
	}
	if errors.Is(err, offerup.ErrNotLoggedIn) {
		return authErr(err)
	}
	return apiErr(err)
}

// newOfferupClient builds the OfferUp web client, honoring --timeout and
// --rate-limit (defaulting to 2 rps, conservative for OfferUp's throttling).
func newOfferupClient(flags *rootFlags) *offerup.Client {
	rate := flags.rateLimit
	if rate <= 0 {
		rate = 2
	}
	return offerup.NewClient(flags.timeout, rate)
}

// openOfferupStore opens the OfferUp SQLite tables at the default DB path.
func openOfferupStore() (*offerup.Store, error) {
	return offerup.OpenStore(defaultDBPath("offerup-pp-cli"))
}

// locationFromFlags assembles a search Location, or nil when no location flag
// was given (leaving the request on OfferUp's IP geo).
func locationFromFlags(zip, lat, lon, city, state string) *offerup.Location {
	if zip == "" && lat == "" && lon == "" && city == "" && state == "" {
		return nil
	}
	return &offerup.Location{Zip: zip, Lat: lat, Lon: lon, City: city, State: state}
}

// storeKeyFor namespaces stored listings by query AND location so price stats
// and new/drop tracking for "iphone near 98101" never mix with "iphone near
// 85001".
func storeKeyFor(query string, loc *offerup.Location) string {
	label := "default"
	switch {
	case loc == nil:
		// A nil location keeps the "default" key and guards the loc.* dereferences below.
	case loc.Zip != "":
		label = "zip:" + loc.Zip
	case loc.Lat != "" && loc.Lon != "":
		label = "geo:" + loc.Lat + "," + loc.Lon
	case loc.City != "":
		label = "city:" + strings.ToLower(strings.TrimSpace(loc.City))
		// Same city name in different states (Portland OR vs Portland ME) must
		// not share a store bucket, or price-drop/new-since/digest history for
		// one corrupts the other.
		if loc.State != "" {
			label += "," + strings.ToLower(strings.TrimSpace(loc.State))
		}
	}
	return query + "@" + label
}

// sinceCutoff converts a --since value (supports 7d / 24h / 1w shorthand) into
// the parsed duration plus an absolute cutoff. Empty defaults to 24h.
func sinceCutoff(since string) (time.Duration, time.Time, error) {
	if strings.TrimSpace(since) == "" {
		since = "24h"
	}
	d, err := cliutil.ParseDurationLoose(since)
	if err != nil {
		return 0, time.Time{}, err
	}
	return d, time.Now().Add(-d), nil
}

// locFlags holds the location/category flags shared by the query-based
// price-intelligence commands.
type locFlags struct {
	zip, lat, lon, city, state, category string
}

func (lf *locFlags) location() *offerup.Location {
	return locationFromFlags(lf.zip, lf.lat, lf.lon, lf.city, lf.state)
}

func (lf *locFlags) searchOpts(limit int) offerup.SearchOptions {
	return offerup.SearchOptions{Category: lf.category, Location: lf.location(), Limit: limit}
}

// storeKey namespaces stored rows by query + this command's location flags.
func (lf *locFlags) storeKey(query string) string {
	return storeKeyFor(query, lf.location())
}

// label returns a human-readable location label for output, or "" for the
// default IP-geo area.
func (lf *locFlags) label() string {
	switch {
	case lf.zip != "":
		return lf.zip
	case lf.city != "" && lf.state != "":
		return lf.city + ", " + lf.state
	case lf.city != "":
		return lf.city
	case lf.lat != "" && lf.lon != "":
		return lf.lat + "," + lf.lon
	default:
		return ""
	}
}
